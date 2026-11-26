// Command source-smoke runs one source against the live site and prints what it
// found. It sends one search and touches no database.
//
//	go run ./cmd/source-smoke -source kakaku -q "RTX 5090"
//	go run ./cmd/source-smoke -source newegg -q "RTX 5090"               # agent, needs LLM_API_KEY
//	go run ./cmd/source-smoke -source newegg -q "RTX 5090" -browser-only # real browser, no LLM
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/FinA1ee/dipwatch/core/internal/agent"
	"github.com/FinA1ee/dipwatch/core/internal/llm"
	"github.com/FinA1ee/dipwatch/core/internal/sources"
	"github.com/FinA1ee/dipwatch/core/internal/sources/ecapi"
	"github.com/FinA1ee/dipwatch/core/internal/sources/kakaku"
)

func main() {
	source := flag.String("source", "kakaku", "source id")
	query := flag.String("q", "RTX 5090", "product to search for")
	limit := flag.Int("limit", 20, "maximum offers")
	browserOnly := flag.Bool("browser-only", false, "open the store's search page in Playwright and print the snapshot size, without an LLM")
	dump := flag.String("dump", "", "with -browser-only: also write the raw snapshot to this file")
	wait := flag.Duration("wait", 0, "with -browser-only: wait this long after opening the page, for pages that load results by script")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := run(ctx, *source, *query, *limit, *browserOnly, *dump, *wait); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, source, query string, limit int, browserOnly bool, dump string, wait time.Duration) error {
	if source == "kakaku" {
		return search(ctx, kakaku.New(&http.Client{Timeout: 30 * time.Second}, kakaku.DefaultBaseURL), source, query, limit)
	}
	if source == "jd" {
		key, keyID := os.Getenv("ECAPI_API_KEY"), os.Getenv("ECAPI_JD_KEY_ID")
		if key == "" || keyID == "" {
			return errors.New("ECAPI_API_KEY and ECAPI_JD_KEY_ID must be set (run it from a whitelisted IP)")
		}
		return search(ctx, &ecapi.JD{HTTP: &http.Client{Timeout: 30 * time.Second}, URL: ecapi.DefaultURL, APIKey: key, KeyID: keyID}, source, query, limit)
	}
	store, ok := agent.Stores[source]
	if !ok {
		return fmt.Errorf("unknown source %q", source)
	}
	if browserOnly {
		return openInBrowser(ctx, store, query, dump, wait)
	}

	cfg, err := llm.ConfigFromEnv()
	if errors.Is(err, llm.ErrNotConfigured) {
		return errors.New("LLM_API_KEY is not set; use -browser-only to test the browser alone")
	} else if err != nil {
		return err
	}
	a := &agent.Agent{
		Model: llm.NewOpenAI(cfg),
		NewBrowser: func(ctx context.Context) (agent.Browser, error) {
			return agent.StartPlaywright(ctx, agent.PlaywrightCommand())
		},
		Recorder: printTrace{},
	}
	return search(ctx, agent.Strategy{Agent: a, Store: store}, source, query, limit)
}

func search(ctx context.Context, s sources.Strategy, source, query string, limit int) error {
	offers, err := s.Search(ctx, sources.Query{Product: query, Limit: limit})
	if err != nil {
		return err
	}
	for _, o := range offers {
		fmt.Printf("%10d %s  %-8s %s\n", o.PriceMinor, o.Currency, o.Availability, o.Title)
	}
	fmt.Printf("\n%d offers from %s (%s strategy)\n", len(offers), source, s.Name())
	return nil
}

func openInBrowser(ctx context.Context, store agent.Store, query, dump string, wait time.Duration) error {
	b, err := agent.StartPlaywright(ctx, agent.PlaywrightCommand())
	if err != nil {
		return err
	}
	defer func() { _ = b.Close() }()

	target := strings.Replace(store.SearchURL, "%s", url.QueryEscape(query), 1)
	if _, err := b.Call(ctx, "browser_navigate", map[string]any{"url": target}); err != nil {
		return err
	}
	if wait > 0 {
		if _, err := b.Call(ctx, "browser_wait_for", map[string]any{"time": wait.Seconds()}); err != nil {
			return err
		}
	}
	snap, err := b.Call(ctx, "browser_snapshot", map[string]any{})
	if err != nil {
		return err
	}
	if dump != "" {
		if err := os.WriteFile(dump, []byte(snap), 0o600); err != nil {
			return err
		}
	}
	lines := strings.Split(snap, "\n")
	fmt.Printf("opened %s\nsnapshot: %d characters, %d lines, %d mention %q\n",
		target, len(snap), len(lines), strings.Count(strings.ToLower(snap), "5090"), "5090")
	ex := agent.Excerpt(snap, query, 12_000)
	fmt.Printf("excerpt sent to the model: %d characters, %d listing windows, %d mention %q\n",
		len(ex), strings.Count(ex, "\n  …\n"), strings.Count(strings.ToLower(ex), "5090"), "5090")
	exLines := strings.Split(ex, "\n")
	for _, l := range exLines[:min(40, len(exLines))] {
		fmt.Println("  " + l)
	}
	return nil
}

// printTrace prints each step of an agent run, so a live run can be inspected.
type printTrace struct{}

func (printTrace) Record(_ context.Context, run agent.Run) error {
	fmt.Fprintf(os.Stderr, "\n--- agent trace (%s, prompt %s)\n", run.Outcome, agent.PromptVersion)
	for i, s := range run.Trace {
		line := s.Tool
		if s.Arguments != "" && s.Arguments != "{}" {
			line += " " + s.Arguments
		}
		if s.Note != "" {
			line = s.Note
		}
		fmt.Fprintf(os.Stderr, "%2d. %.140s\n    -> %.140s\n", i+1, line, strings.ReplaceAll(s.Result, "\n", " "))
	}
	fmt.Fprintf(os.Stderr, "steps %d, tokens in %d out %d, cost %.4f yuan, %s\n\n",
		run.Steps, run.TokensIn, run.TokensOut, float64(run.CostMicros)/1e6, run.Duration.Round(time.Second))
	return nil
}
