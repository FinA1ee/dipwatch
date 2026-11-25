// Command intent-eval checks the query gate's intent call (spec S3, S4) with the
// real model: every spelling in a group must map to one canonical name, and
// non-product texts must be rejected. It calls a paid model (about ¥0.02 per run).
//
//	go run ./cmd/intent-eval -env ../deploy/.env
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/FinA1ee/dipwatch/core/internal/guard"
	"github.com/FinA1ee/dipwatch/core/internal/llm"
)

type cases struct {
	Products    [][]string `json:"products"`
	NotProducts []string   `json:"not_products"`
}

func main() {
	file := flag.String("cases", "internal/guard/testdata/queries.json", "eval cases")
	envFile := flag.String("env", "", "read LLM_* settings from this .env file")
	flag.Parse()
	if err := run(*file, *envFile); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(file, envFile string) error {
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
	raw, err := os.ReadFile(file) //nolint:gosec // path given by the developer
	if err != nil {
		return err
	}
	var c cases
	if err := json.Unmarshal(raw, &c); err != nil {
		return err
	}
	checker := &guard.Checker{Model: llm.NewOpenAI(cfg)}

	// The catalog grows as in production: each new canonical name is offered to later
	// queries whose model-number words match it.
	var catalog []string
	var cost int64
	var calls int
	check := func(text string) (guard.Verdict, error) {
		query, err := guard.Normalize(text)
		if err != nil {
			return guard.Verdict{}, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		v, u, err := checker.Check(ctx, query, candidates(catalog, guard.CandidatePatterns(query)))
		cost += u.CostMicros
		calls++
		if err == nil && v.IsProduct && !contains(catalog, v.CanonicalName) {
			catalog = append(catalog, v.CanonicalName)
		}
		return v, err
	}

	fmt.Printf("# Intent eval: %s\n\nModel `%s`, %s.\n\n", guard.PromptVersion, cfg.Model, time.Now().Format("2006-01-02 15:04"))
	fmt.Println("| Spellings | Canonical names | One product |")
	fmt.Println("|---|---|---|")
	groupsOK := 0
	for _, group := range c.Products {
		names := map[string]int{}
		for _, text := range group {
			v, err := check(text)
			switch {
			case err != nil:
				names["error: "+err.Error()]++
			case !v.IsProduct:
				names["(not a product)"]++
			default:
				names[strings.ToLower(v.CanonicalName)]++
			}
		}
		ok := len(names) == 1 && names["(not a product)"] == 0
		if ok {
			groupsOK++
		}
		fmt.Printf("| %s | %s | %s |\n", strings.Join(group, ", "), join(names), mark(ok))
	}

	fmt.Println("\n| Not a product | Verdict | Rejected |")
	fmt.Println("|---|---|---|")
	rejected := 0
	for _, text := range c.NotProducts {
		v, err := check(text)
		verdict := "not a product"
		ok := err == nil && !v.IsProduct
		if err != nil {
			verdict = "error: " + err.Error()
		} else if v.IsProduct {
			verdict = v.CanonicalName
		}
		if ok {
			rejected++
		}
		fmt.Printf("| %s | %s | %s |\n", text, verdict, mark(ok))
	}
	fmt.Printf("\nProducts mapped to one name: %d of %d. Non-products rejected: %d of %d. %d calls, ¥%.3f.\n",
		groupsOK, len(c.Products), rejected, len(c.NotProducts), calls, float64(cost)/1e6)
	return nil
}

// candidates applies the ILIKE patterns to the in-memory catalog.
func candidates(catalog, patterns []string) []string {
	var out []string
	for _, name := range catalog {
		for _, p := range patterns {
			word := strings.NewReplacer(`\%`, "%", `\_`, "_", `\\`, `\`).Replace(strings.Trim(p, "%"))
			if strings.Contains(strings.ToLower(name), strings.ToLower(word)) {
				out = append(out, name)
				break
			}
		}
	}
	return out
}

func join(names map[string]int) string {
	parts := make([]string, 0, len(names))
	for n, count := range names {
		parts = append(parts, fmt.Sprintf("%s ×%d", n, count))
	}
	return strings.Join(parts, "; ")
}

func mark(ok bool) string {
	if ok {
		return "yes"
	}
	return "**no**"
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
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
