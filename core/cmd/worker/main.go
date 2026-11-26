// Command worker runs background jobs.
//
// Usage:
//
//	worker                       process jobs until stopped
//	worker refresh "RTX 5090"    create the product if needed, enqueue a refresh, and exit
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/FinA1ee/dipwatch/core/internal/agent"
	"github.com/FinA1ee/dipwatch/core/internal/alerts"
	"github.com/FinA1ee/dipwatch/core/internal/config"
	"github.com/FinA1ee/dipwatch/core/internal/db"
	"github.com/FinA1ee/dipwatch/core/internal/fx"
	"github.com/FinA1ee/dipwatch/core/internal/jobs"
	"github.com/FinA1ee/dipwatch/core/internal/limits"
	"github.com/FinA1ee/dipwatch/core/internal/live"
	"github.com/FinA1ee/dipwatch/core/internal/llm"
	"github.com/FinA1ee/dipwatch/core/internal/matcher"
	"github.com/FinA1ee/dipwatch/core/internal/notify"
	"github.com/FinA1ee/dipwatch/core/internal/sources"
	"github.com/FinA1ee/dipwatch/core/internal/sources/ecapi"
	"github.com/FinA1ee/dipwatch/core/internal/sources/jdunion"
	"github.com/FinA1ee/dipwatch/core/internal/sources/kakaku"
	"github.com/FinA1ee/dipwatch/core/internal/spend"
	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// maxWorkers keeps concurrent source fetches low: the server has 2 vCPU and 4 GB (spec N1).
const maxWorkers = 2

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	var err error
	if len(os.Args) > 2 && os.Args[1] == "refresh" {
		err = enqueueRefresh(os.Args[2])
	} else {
		err = run()
	}
	if err != nil {
		slog.Error("worker failed", "err", err)
		os.Exit(1)
	}
}

// adapters builds the source strategies, plus the listing matcher when an LLM is configured.
func adapters(pool *pgxpool.Pool) (jobs.Adapters, *matcher.Matcher) {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	a := jobs.Adapters{
		"kakaku": {"html": kakaku.New(httpClient, kakaku.DefaultBaseURL)},
	}
	// JD through 喵有券 when its key and the authorized JD Union key_id are set,
	// else through the owner's own JD Union app key.
	if key, keyID := os.Getenv("ECAPI_API_KEY"), os.Getenv("ECAPI_JD_KEY_ID"); key != "" && keyID != "" {
		a["jd"] = map[string]sources.Strategy{"api": &ecapi.JD{HTTP: httpClient, URL: ecapi.DefaultURL, APIKey: key, KeyID: keyID}}
		slog.Info("JD enabled through 喵有券")
	} else if key, secret := os.Getenv("JD_UNION_APP_KEY"), os.Getenv("JD_UNION_APP_SECRET"); key != "" && secret != "" {
		a["jd"] = map[string]sources.Strategy{"api": &jdunion.Client{HTTP: httpClient, URL: jdunion.DefaultURL, AppKey: key, AppSecret: secret}}
		slog.Info("JD Union API enabled")
	}

	cfg, err := llm.ConfigFromEnv()
	if errors.Is(err, llm.ErrNotConfigured) {
		slog.Info("LLM_API_KEY not set: browsing agent and matcher disabled")
		return a, nil
	} else if err != nil {
		slog.Error("LLM config invalid: browsing agent and matcher disabled", "err", err)
		return a, nil
	}
	model := llm.NewOpenAI(cfg)
	browsing := &agent.Agent{
		Model: model,
		NewBrowser: func(ctx context.Context) (agent.Browser, error) {
			return agent.StartPlaywright(ctx, agent.PlaywrightCommand())
		},
		Recorder: agent.DBRecorder{Queries: store.New(pool)},
		Observer: live.Notifier{Queries: store.New(pool)},
	}
	for id, s := range agent.Stores {
		if a[id] == nil {
			a[id] = map[string]sources.Strategy{}
		}
		a[id]["agent"] = agent.Strategy{Agent: browsing, Store: s}
	}
	slog.Info("browsing agent and matcher enabled", "model", cfg.Model, "stores", len(agent.Stores))
	return a, &matcher.Matcher{Model: model}
}

// loadMailer returns the SMTP mailer for email alerts, or nil when SMTP_URL is
// not set: alerts then go to Feishu only.
func loadMailer() (*notify.Mailer, error) {
	smtpURL := os.Getenv("SMTP_URL")
	if smtpURL == "" {
		slog.Info("SMTP_URL not set: email alerts off")
		return nil, nil
	}
	return notify.NewMailer(smtpURL, os.Getenv("MAIL_FROM"))
}

func open(ctx context.Context) (*pgxpool.Pool, error) {
	url, err := config.LoadDatabaseURL()
	if err != nil {
		return nil, err
	}
	return db.Open(ctx, url)
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := open(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()

	mailer, err := loadMailer()
	if err != nil {
		return err
	}
	checker := &alerts.Checker{EmailEnabled: mailer != nil, Enqueue: jobs.EnqueueNotify}
	breaker := &spend.Breaker{DB: pool, Limits: &limits.Store{DB: pool}}

	workers := river.NewWorkers()
	sourceAdapters, listingMatcher := adapters(pool)
	river.AddWorker(workers, &jobs.RefreshProductWorker{DB: pool, Adapters: sourceAdapters, Spend: breaker})
	river.AddWorker(workers, &jobs.RefreshSourceWorker{
		DB:       pool,
		Registry: sources.NewRegistry(),
		Adapters: sourceAdapters,
		Matcher:  listingMatcher,
		Alerts:   checker,
		Spend:    breaker,
	})
	river.AddWorker(workers, &jobs.NotifyWorker{DB: pool, Mailer: mailer, PublicURL: os.Getenv("PUBLIC_URL")})
	river.AddWorker(workers, &jobs.ScheduleWorker{DB: pool, Spend: breaker})
	river.AddWorker(workers, &jobs.SimulateDropWorker{DB: pool, Alerts: checker})
	river.AddWorker(workers, &jobs.FXRefreshWorker{
		DB:     pool,
		Client: &http.Client{Timeout: 20 * time.Second},
		URL:    fx.DefaultURL,
	})
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: maxWorkers},
			jobs.NotifyQueue:   {MaxWorkers: 2},
		},
		Workers:         workers,
		PeriodicJobs:    []*river.PeriodicJob{jobs.FXPeriodicJob(), jobs.SchedulePeriodicJob()},
		SoftStopTimeout: 20 * time.Second,
	})
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	slog.Info("worker ready", "max_workers", maxWorkers)

	<-ctx.Done()
	slog.Info("worker shutting down")
	<-client.Stopped()
	return nil
}

// enqueueRefresh is a development helper until search (Day 4) creates products.
func enqueueRefresh(name string) error {
	if name == "" {
		return errors.New("product name is empty")
	}
	ctx := context.Background()
	pool, err := open(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()

	product, err := store.New(pool).UpsertProduct(ctx, store.UpsertProductParams{CanonicalName: name})
	if err != nil {
		return err
	}
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		return err
	}
	res, err := client.Insert(ctx, jobs.RefreshProductArgs{ProductID: product.ID}, nil)
	if err != nil {
		return err
	}
	slog.Info("refresh enqueued", "product_id", product.ID, "job_id", res.Job.ID, "duplicate", res.UniqueSkippedAsDuplicate)
	return nil
}
