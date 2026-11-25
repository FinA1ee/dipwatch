// Command matcher-eval scores the LLM matcher (spec S6) on the listings labelled
// for the agent eval: is each listing the product itself? Ambiguous listings and
// the injection copy are left out. It calls a paid model (about ¥0.05 per run).
//
//	go run ./cmd/matcher-eval -env ../deploy/.env
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/FinA1ee/dipwatch/core/internal/agent/eval"
	"github.com/FinA1ee/dipwatch/core/internal/llm"
	"github.com/FinA1ee/dipwatch/core/internal/matcher"
)

func main() {
	dir := flag.String("cases", "internal/agent/eval/testdata", "directory of eval cases")
	envFile := flag.String("env", "", "read LLM_* settings from this .env file")
	flag.Parse()
	if err := run(*dir, *envFile); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(dir, envFile string) error {
	if envFile != "" {
		if err := loadLLMEnv(envFile); err != nil {
			return err
		}
	}
	cfg, err := llm.ConfigFromEnv()
	if errors.Is(err, llm.ErrNotConfigured) {
		return errors.New("LLM_API_KEY is not set")
	} else if err != nil {
		return err
	}
	m := &matcher.Matcher{Model: llm.NewOpenAI(cfg)}

	cases, err := eval.LoadCases(dir)
	if err != nil {
		return err
	}
	var tp, fp, tn, fn int
	var usage matcher.Usage
	var mistakes []string
	start := time.Now()
	for _, c := range cases {
		if c.Expected.Injection != nil {
			continue
		}
		var titles []string
		var labels []eval.Label
		for _, l := range c.Expected.Listings {
			if !l.Ambiguous {
				titles = append(titles, l.Title)
				labels = append(labels, l)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		decisions, u, err := m.Decide(ctx, c.Expected.Product, titles)
		cancel()
		usage.Calls += u.Calls
		usage.TokensIn += u.TokensIn
		usage.TokensOut += u.TokensOut
		usage.CostMicros += u.CostMicros
		if err != nil {
			return fmt.Errorf("%s: %w", c.Name, err)
		}
		for i, d := range decisions {
			want := labels[i].IsProduct
			switch {
			case d.IsProduct && want:
				tp++
			case d.IsProduct && !want:
				fp++
				mistakes = append(mistakes, fmt.Sprintf("%s: kept, but not the product: %.70s", c.Name, labels[i].Title))
			case !d.IsProduct && want:
				fn++
				mistakes = append(mistakes, fmt.Sprintf("%s: dropped, but is the product: %.70s", c.Name, labels[i].Title))
			default:
				tn++
			}
		}
		fmt.Fprintf(os.Stderr, "%-28s %3d listings\n", c.Name, len(titles))
	}

	total := tp + fp + tn + fn
	fmt.Printf("# Matcher eval: %s\n\n", matcher.PromptVersion)
	fmt.Printf("Model `%s`, %s, %d listings from %d cases, %d calls in %s.\n\n",
		cfg.Model, time.Now().Format("2006-01-02 15:04"), total, len(cases)-1, usage.Calls, time.Since(start).Round(time.Second))
	fmt.Println("| Metric | Value | Target |")
	fmt.Println("|---|---|---|")
	fmt.Printf("| Accuracy | %.1f%% | at least 90%% |\n", 100*float64(tp+tn)/float64(total))
	fmt.Printf("| Precision (kept listings that are the product) | %.1f%% | |\n", 100*float64(tp)/float64(max(1, tp+fp)))
	fmt.Printf("| Recall (product listings that were kept) | %.1f%% | |\n", 100*float64(tp)/float64(max(1, tp+fn)))
	fmt.Printf("| True / false positives | %d / %d | |\n", tp, fp)
	fmt.Printf("| True / false negatives | %d / %d | |\n", tn, fn)
	fmt.Printf("| Cost | ¥%.3f (%d tokens in, %d out) | |\n", float64(usage.CostMicros)/1e6, usage.TokensIn, usage.TokensOut)
	if len(mistakes) > 0 {
		fmt.Print("\n## Mistakes\n\n")
		for _, s := range mistakes {
			fmt.Println("- " + s)
		}
	}
	return nil
}

func loadLLMEnv(path string) error {
	raw, err := os.ReadFile(path) //nolint:gosec // path given by the developer
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && strings.HasPrefix(key, "LLM_") {
			if _, set := os.LookupEnv(key); !set {
				_ = os.Setenv(key, value)
			}
		}
	}
	return nil
}
