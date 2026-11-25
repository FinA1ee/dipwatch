package guard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FinA1ee/dipwatch/core/internal/db"
	"github.com/FinA1ee/dipwatch/core/internal/llm"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		in, want string
		err      error
	}{
		{"  RTX   5090 ", "rtx 5090", nil},
		{"ＲＴＸ　５０９０", "rtx 5090", nil}, // full-width input from Chinese keyboards
		{"索尼 WH-1000XM6", "索尼 wh-1000xm6", nil},
		{"   ", "", ErrEmpty},
		{strings.Repeat("a", MaxRunes+1), "", ErrTooLong},
		{strings.Repeat("显", MaxRunes), strings.Repeat("显", MaxRunes), nil}, // runes, not bytes
		{"https://evil.example/x", "", ErrURL},
		{"rtx 5090 jd.com", "", ErrURL},
		{"www.amazon", "", ErrURL},
	}
	for _, c := range cases {
		got, err := Normalize(c.in)
		if got != c.want || !errors.Is(err, c.err) {
			t.Errorf("Normalize(%q) = %q, %v; want %q, %v", c.in, got, err, c.want, c.err)
		}
	}
}

func TestCandidatePatterns(t *testing.T) {
	if got := CandidatePatterns("nvidia rtx 5090"); len(got) != 1 || got[0] != "%5090%" {
		t.Fatalf("model numbers: %v", got)
	}
	if got := CandidatePatterns("steam deck"); len(got) != 2 || got[0] != "%steam%" {
		t.Fatalf("words: %v", got)
	}
	if got := CandidatePatterns("100%_x1"); got[0] != `%100\%\_x1%` {
		t.Fatalf("escaping: %v", got)
	}
}

func TestParseRejectsUnusableNames(t *testing.T) {
	call := func(args string) llm.Turn {
		return llm.Turn{ToolCalls: []llm.ToolCall{{ID: "1", Name: toolName, Arguments: args}}}
	}
	if _, err := parse(call(`{"is_product":true,"canonical_name":"visit https://x.example","category":"gpu"}`)); err == nil {
		t.Fatal("URL in name accepted")
	}
	v, err := parse(call(`{"is_product":true,"canonical_name":" Sony  WH-1000XM6 ","category":"headphones"}`))
	if err != nil || v.CanonicalName != "Sony WH-1000XM6" || v.Category != "other" {
		t.Fatalf("got %+v, %v", v, err)
	}
	if v, err := parse(call(`{"is_product":false,"canonical_name":"ignored","category":"other"}`)); err != nil || v.IsProduct {
		t.Fatalf("not a product: %+v, %v", v, err)
	}
	if _, err := parse(llm.Turn{Content: "RTX 5090"}); err == nil {
		t.Fatal("answer without a tool call accepted")
	}
}

// byModelNumber is a scripted intent model: texts with a digit are products named
// after their model number, reusing an existing name when one is listed.
type byModelNumber struct {
	suffix string
	calls  int
	seen   []string
}

func (m *byModelNumber) Name() string                   { return "scripted" }
func (m *byModelNumber) CostMicros(in, out int64) int64 { return in + out }
func (m *byModelNumber) Chat(_ context.Context, msgs []llm.Message, _ []llm.Tool) (llm.Turn, error) {
	m.calls++
	prompt := msgs[len(msgs)-1].Content
	m.seen = append(m.seen, prompt)
	args := `{"is_product":false,"canonical_name":"","category":"other"}`
	searchText, _, _ := strings.Cut(prompt, "\n")
	if strings.ContainsAny(searchText, "0123456789") {
		name := "Test Card 7" + m.suffix
		if i := strings.Index(prompt, "- "); i >= 0 {
			name = strings.ToLower(strings.TrimSpace(strings.SplitN(prompt[i+2:], "\n", 2)[0])) // differs in case only
		}
		args = fmt.Sprintf(`{"is_product":true,"canonical_name":%q,"category":"gpu"}`, name)
	}
	return llm.Turn{ToolCalls: []llm.ToolCall{{ID: "1", Name: toolName, Arguments: args}}, TokensIn: 100, TokensOut: 10}, nil
}

func setup(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	if err := db.Migrate(ctx, url); err != nil {
		t.Fatal(err)
	}
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return ctx, pool
}

func TestGateCachesAndSharesOneProduct(t *testing.T) {
	ctx, pool := setup(t)
	// A model number unique to this run, so earlier runs' cache rows do not interfere.
	suffix := fmt.Sprint(time.Now().UnixNano())
	model := &byModelNumber{suffix: suffix}
	gate := &Gate{DB: pool, Checker: &Checker{Model: model}}

	first, err := gate.Resolve(ctx, "card 7"+suffix, nil)
	if err != nil || first.Cached {
		t.Fatalf("first: %+v, %v", first, err)
	}
	// A different spelling: the model sees the existing product and reuses it.
	second, err := gate.Resolve(ctx, "TEST card 7"+suffix, nil)
	if err != nil || second.ID != first.ID {
		t.Fatalf("second spelling: %+v, %v (first %+v)", second, err, first)
	}
	if !strings.Contains(model.seen[1], "Existing products:") {
		t.Fatalf("candidates not offered: %q", model.seen[1])
	}
	// Same text again, with different spacing: answered from the cache.
	again, err := gate.Resolve(ctx, "  card   7"+suffix, nil)
	if err != nil || !again.Cached || again.ID != first.ID || model.calls != 2 {
		t.Fatalf("cached: %+v, %v after %d calls", again, err, model.calls)
	}

	// Not a product: rejected, and the rejection is cached too.
	letters := strings.Map(func(r rune) rune { return 'a' + (r - '0') }, suffix) // unique, digit-free
	nonsense := "how to cook rice " + letters
	for range 2 {
		if _, err := gate.Resolve(ctx, nonsense, nil); !errors.Is(err, ErrNotProduct) {
			t.Fatalf("nonsense: %v", err)
		}
	}
	if model.calls != 3 {
		t.Fatalf("LLM calls: %d, want 3", model.calls)
	}

	// Bad input never reaches the model.
	if _, err := gate.Resolve(ctx, "http://x.example", nil); !errors.Is(err, ErrURL) || model.calls != 3 {
		t.Fatalf("URL: %v after %d calls", err, model.calls)
	}
}
