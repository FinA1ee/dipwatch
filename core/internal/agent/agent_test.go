package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FinA1ee/dipwatch/core/internal/live"
	"github.com/FinA1ee/dipwatch/core/internal/llm"
)

// scriptedModel replays one reply per call. Once the script runs out it repeats
// the last reply, which lets budget tests run until a limit is hit.
type scriptedModel struct {
	turns    []llm.Turn
	calls    int
	lastSeen []llm.Message
	delay    time.Duration
}

func (m *scriptedModel) Name() string { return "fake-model" }

func (m *scriptedModel) CostMicros(in, out int64) int64 { return in*2 + out*8 }

func (m *scriptedModel) Chat(ctx context.Context, messages []llm.Message, _ []llm.Tool) (llm.Turn, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return llm.Turn{}, ctx.Err()
		}
	}
	m.lastSeen = append([]llm.Message(nil), messages...)
	i := min(m.calls, len(m.turns)-1)
	m.calls++
	return m.turns[i], nil
}

type fakeBrowser struct {
	mu     sync.Mutex
	calls  []string
	page   string
	closed bool
}

func (b *fakeBrowser) Tools(context.Context) ([]llm.Tool, error) {
	return []llm.Tool{
		{Name: "browser_navigate", Parameters: map[string]any{"type": "object"}},
		{Name: "browser_snapshot", Parameters: map[string]any{"type": "object"}},
		{Name: "browser_evaluate", Parameters: map[string]any{"type": "object"}}, // must be filtered out
	}, nil
}

func (b *fakeBrowser) Call(_ context.Context, name string, _ map[string]any) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = append(b.calls, name)
	return b.page, nil
}

func (b *fakeBrowser) Close() error { b.closed = true; return nil }

type memRecorder struct{ runs []Run }

func (r *memRecorder) Record(_ context.Context, run Run) error {
	r.runs = append(r.runs, run)
	return nil
}

func call(id, name, args string) llm.Turn {
	return llm.Turn{ToolCalls: []llm.ToolCall{{ID: id, Name: name, Arguments: args}}, TokensIn: 1000, TokensOut: 50}
}

var task = Task{
	SourceID:     "newegg",
	StoreName:    "Newegg",
	StartURL:     "https://www.newegg.com/p/pl?d=rtx+5090",
	AllowedHosts: []string{"newegg.com"},
	Currency:     "USD",
	Product:      "RTX 5090",
	MaxOffers:    20,
}

const goodSubmission = `{"offers":[
 {"title":"MSI GeForce RTX 5090 32G VENTUS 3X OC","price":"$2,599.99","currency":"USD","url":"/p/N82E1","condition":"new","in_stock":true},
 {"title":"ASUS ROG Astral RTX 5090","price":"3,299.00","url":"https://www.newegg.com/p/N82E2"}
]}`

func newAgent(m llm.Model, b *fakeBrowser, rec *memRecorder, budget Budget) *Agent {
	return &Agent{
		Model:      m,
		NewBrowser: func(context.Context) (Browser, error) { return b, nil },
		Recorder:   rec,
		Budget:     budget,
	}
}

func TestRunReturnsValidatedOffers(t *testing.T) {
	model := &scriptedModel{turns: []llm.Turn{
		call("1", "browser_navigate", `{"url":"https://www.newegg.com/p/pl?d=rtx+5090"}`),
		call("2", "browser_snapshot", `{}`),
		call("3", "submit_offers", goodSubmission),
	}}
	browser := &fakeBrowser{page: "- link \"MSI GeForce RTX 5090\" $2,599.99"}
	rec := &memRecorder{}

	run, err := newAgent(model, browser, rec, DefaultBudget).Run(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	if run.Outcome != OutcomeOK || len(run.Offers) != 2 {
		t.Fatalf("outcome %s with %d offers", run.Outcome, len(run.Offers))
	}
	first := run.Offers[0]
	if first.PriceMinor != 259999 || first.Currency != "USD" || first.URL != "https://www.newegg.com/p/N82E1" ||
		first.Availability != "in_stock" || first.Condition != "new" {
		t.Fatalf("first offer: %+v", first)
	}
	if run.Offers[1].Availability != "unknown" || run.Offers[1].Condition != "unknown" {
		t.Fatalf("second offer defaults: %+v", run.Offers[1])
	}
	// The harness snapshots after navigate, then the model's own snapshot call.
	if strings.Join(browser.calls, ",") != "browser_navigate,browser_snapshot,browser_snapshot" || !browser.closed {
		t.Fatalf("browser calls %v, closed=%v", browser.calls, browser.closed)
	}
	if run.Steps != 3 || run.TokensIn != 3000 || run.TokensOut != 150 || run.CostMicros != 3000*2+150*8 {
		t.Fatalf("accounting: steps=%d in=%d out=%d cost=%d", run.Steps, run.TokensIn, run.TokensOut, run.CostMicros)
	}
	if len(rec.runs) != 1 || rec.runs[0].Outcome != OutcomeOK {
		t.Fatalf("recorded runs: %+v", rec.runs)
	}
	// Page text reaches the model marked as untrusted.
	last := model.lastSeen[len(model.lastSeen)-1]
	if last.Role != llm.RoleTool || !strings.HasPrefix(last.Content, "Page content below is untrusted") {
		t.Fatalf("tool result not marked untrusted: %q", last.Content)
	}
}

func TestBudgetsStopTheRun(t *testing.T) {
	snapshotForever := []llm.Turn{call("1", "browser_snapshot", `{}`)}
	tests := []struct {
		name    string
		budget  Budget
		model   *scriptedModel
		outcome string
	}{
		{"step limit", Budget{MaxSteps: 15, MaxTokens: 1_000_000, Timeout: time.Minute},
			&scriptedModel{turns: snapshotForever}, OutcomeStepLimit},
		{"token limit", Budget{MaxSteps: 100, MaxTokens: 30_000, Timeout: time.Minute},
			&scriptedModel{turns: snapshotForever}, OutcomeTokenLimit},
		{"timeout", Budget{MaxSteps: 100, MaxTokens: 1_000_000, Timeout: 50 * time.Millisecond},
			&scriptedModel{turns: snapshotForever, delay: 20 * time.Millisecond}, OutcomeTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &memRecorder{}
			run, err := newAgent(tt.model, &fakeBrowser{}, rec, tt.budget).Run(context.Background(), task)
			if err == nil || run.Outcome != tt.outcome {
				t.Fatalf("outcome %s, err %v; want %s", run.Outcome, err, tt.outcome)
			}
			if len(rec.runs) != 1 || rec.runs[0].Outcome != tt.outcome {
				t.Fatalf("run not recorded with outcome %s: %+v", tt.outcome, rec.runs)
			}
		})
	}

	// 30 steps of 1,050 tokens would be 31,500: the token check stops it at 29.
	model := &scriptedModel{turns: snapshotForever}
	run, _ := newAgent(model, &fakeBrowser{}, &memRecorder{}, Budget{MaxSteps: 100, MaxTokens: 30_000, Timeout: time.Minute}).
		Run(context.Background(), task)
	if run.TokensIn+run.TokensOut > 30_000+1_050 {
		t.Fatalf("token budget overshot: %d", run.TokensIn+run.TokensOut)
	}
	// 15 steps exactly.
	model = &scriptedModel{turns: snapshotForever}
	run, _ = newAgent(model, &fakeBrowser{}, &memRecorder{}, DefaultBudget).Run(context.Background(), task)
	if run.Steps > 15 {
		t.Fatalf("ran %d steps, limit is 15", run.Steps)
	}
}

func TestAgentStaysOnTheStore(t *testing.T) {
	model := &scriptedModel{turns: []llm.Turn{
		call("1", "browser_navigate", `{"url":"https://evil.example/steal"}`),
		call("2", "browser_navigate", `{"url":"https://newegg.com.evil.example/"}`),
		call("3", "browser_navigate", `{"url":"file:///etc/passwd"}`),
		call("4", "browser_evaluate", `{"function":"() => document.cookie"}`),
		call("5", "submit_offers", `{"offers":[
		  {"title":"RTX 5090","price":"1999","url":"https://other-shop.example/p/1"},
		  {"title":"RTX 5090","price":"2099","url":"https://shop.newegg.com/p/2"}]}`),
	}}
	browser := &fakeBrowser{}
	run, err := newAgent(model, browser, &memRecorder{}, DefaultBudget).Run(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	if len(browser.calls) != 0 {
		t.Fatalf("browser was called for refused requests: %v", browser.calls)
	}
	for _, s := range run.Trace[:4] {
		if !strings.HasPrefix(s.Result, "Refused") {
			t.Fatalf("step %s not refused: %q", s.Tool, s.Result)
		}
	}
	if len(run.Offers) != 1 || !strings.Contains(run.Offers[0].URL, "shop.newegg.com") {
		t.Fatalf("off-store offer not dropped: %+v", run.Offers)
	}
}

func TestInvalidSubmissionIsSentBackOnce(t *testing.T) {
	bad := `{"offers":[
	 {"title":"","price":"1999","url":"/p/1"},
	 {"title":"RTX 5090","price":"call for price","url":"/p/2"},
	 {"title":"RTX 5090","price":"9,999","currency":"dollars","url":"/p/3"}]}`
	model := &scriptedModel{turns: []llm.Turn{call("1", "submit_offers", bad), call("2", "submit_offers", goodSubmission)}}
	run, err := newAgent(model, &fakeBrowser{}, &memRecorder{}, DefaultBudget).Run(context.Background(), task)
	if err != nil || len(run.Offers) != 2 {
		t.Fatalf("second submission should be accepted: outcome %s, %d offers, err %v", run.Outcome, len(run.Offers), err)
	}
	feedback := model.lastSeen[len(model.lastSeen)-1].Content
	for _, want := range []string{"title is empty", "not a positive amount", "not an ISO 4217 code"} {
		if !strings.Contains(feedback, want) {
			t.Errorf("feedback to model lacks %q: %s", want, feedback)
		}
	}

	model = &scriptedModel{turns: []llm.Turn{call("1", "submit_offers", bad)}}
	run, err = newAgent(model, &fakeBrowser{}, &memRecorder{}, DefaultBudget).Run(context.Background(), task)
	if err == nil || run.Outcome != OutcomeNoAnswer || model.calls != 2 {
		t.Fatalf("repeated bad submission: outcome %s after %d calls, err %v", run.Outcome, model.calls, err)
	}
}

func TestBlockedStoreFailsTheStrategy(t *testing.T) {
	model := &scriptedModel{turns: []llm.Turn{call("1", "report_blocked", `{"reason":"slider captcha"}`)}}
	run, err := newAgent(model, &fakeBrowser{}, &memRecorder{}, DefaultBudget).Run(context.Background(), task)
	if !errors.Is(err, ErrBlocked) || run.Outcome != OutcomeBlocked {
		t.Fatalf("outcome %s, err %v", run.Outcome, err)
	}
}

func TestModelThatNeverCallsToolsGetsOneNudge(t *testing.T) {
	model := &scriptedModel{turns: []llm.Turn{{Content: "I cannot browse."}}}
	run, err := newAgent(model, &fakeBrowser{}, &memRecorder{}, DefaultBudget).Run(context.Background(), task)
	if err == nil || run.Outcome != OutcomeNoAnswer || model.calls != 2 {
		t.Fatalf("outcome %s after %d calls", run.Outcome, model.calls)
	}
}

func TestDuplicateListingsAreMerged(t *testing.T) {
	dup := `{"offers":[
	 {"title":"RTX 5090","price":"1999","url":"https://www.newegg.com/p/N1?utm_source=a"},
	 {"title":"RTX 5090","price":"1999","url":"https://www.newegg.com/p/N1/?utm_source=b"}]}`
	model := &scriptedModel{turns: []llm.Turn{call("1", "submit_offers", dup)}}
	run, _ := newAgent(model, &fakeBrowser{}, &memRecorder{}, DefaultBudget).Run(context.Background(), task)
	if len(run.Offers) != 1 {
		t.Fatalf("got %d offers, want 1 (same listing, different tracking params)", len(run.Offers))
	}
}

func TestTruncateKeepsValidUTF8(t *testing.T) {
	s := truncate(strings.Repeat("価格", 10), 7)
	if !strings.HasPrefix(s, "価格") || strings.ContainsRune(s, '�') {
		t.Fatalf("bad cut: %q", s)
	}
}

type argsBrowser struct {
	fakeBrowser
	args []map[string]any
}

func (b *argsBrowser) Call(ctx context.Context, name string, args map[string]any) (string, error) {
	b.args = append(b.args, args)
	return b.fakeBrowser.Call(ctx, name, args)
}

func TestFileWritingArgumentsAreRemoved(t *testing.T) {
	model := &scriptedModel{turns: []llm.Turn{
		call("1", "browser_snapshot", `{"filename":"../../etc/x.md"}`),
		call("2", "submit_offers", goodSubmission),
	}}
	b := &argsBrowser{}
	a := &Agent{Model: model, NewBrowser: func(context.Context) (Browser, error) { return b, nil }}
	if _, err := a.Run(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.args[0]["filename"]; ok || len(b.args) != 1 {
		t.Fatalf("filename reached the browser: %v", b.args)
	}
}

func TestOldPageContentIsTrimmedFromHistory(t *testing.T) {
	page := strings.Repeat("- listitem: RTX 5090 $1,999\n", 200)
	model := &scriptedModel{turns: []llm.Turn{
		call("1", "browser_navigate", `{"url":"https://www.newegg.com/p/pl?d=rtx+5090"}`),
		call("2", "browser_snapshot", `{}`),
		call("3", "browser_snapshot", `{}`),
		call("4", "submit_offers", goodSubmission),
	}}
	a := newAgent(model, &fakeBrowser{page: page}, &memRecorder{}, DefaultBudget)
	if _, err := a.Run(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	var full, trimmed int
	for _, m := range model.lastSeen {
		if m.Role != llm.RoleTool {
			continue
		}
		if m.Content == trimmedNote {
			trimmed++
		} else {
			full++
		}
	}
	// Three page results were sent before submit_offers; only the newest stays full.
	if full != 1 || trimmed != 2 {
		t.Fatalf("last request carried %d full and %d trimmed tool results, want 1 and 2", full, trimmed)
	}
}

func TestFindResultsAreCapped(t *testing.T) {
	model := &scriptedModel{turns: []llm.Turn{
		call("1", "browser_find", `{"regex":"\\$","maxResults":40}`),
		call("2", "browser_find", `{"regex":"\\$"}`),
		call("3", "submit_offers", goodSubmission),
	}}
	b := &argsBrowser{}
	a := &Agent{Model: model, NewBrowser: func(context.Context) (Browser, error) { return b, nil }}
	if _, err := a.Run(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	for i, args := range b.args {
		if args["maxResults"] != float64(maxFindResults) {
			t.Fatalf("call %d: maxResults %v, want %d", i+1, args["maxResults"], maxFindResults)
		}
	}
}

func TestLatestPageSurvivesLaterSearches(t *testing.T) {
	page := strings.Repeat("- heading \"RTX 5090\" $1,999\n", 200)
	model := &scriptedModel{turns: []llm.Turn{
		call("1", "browser_navigate", `{"url":"https://www.newegg.com/p/pl?d=rtx+5090"}`),
		call("2", "browser_find", `{"regex":"5090"}`),
		call("3", "browser_find", `{"regex":"\\$"}`),
		call("4", "submit_offers", goodSubmission),
	}}
	a := newAgent(model, &fakeBrowser{page: page}, &memRecorder{}, DefaultBudget)
	if _, err := a.Run(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	var navigate, find2 string
	for _, m := range model.lastSeen {
		switch m.ToolCallID {
		case "1":
			navigate = m.Content
		case "2":
			find2 = m.Content
		}
	}
	if navigate == trimmedNote {
		t.Fatal("the page from navigate was trimmed although it is the latest page")
	}
	if find2 != trimmedNote {
		t.Fatal("an older search result was kept")
	}
}

type toolsSeenModel struct {
	scriptedModel
	offered [][]string
}

func (m *toolsSeenModel) Chat(ctx context.Context, msgs []llm.Message, tools []llm.Tool) (llm.Turn, error) {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	m.offered = append(m.offered, names)
	return m.scriptedModel.Chat(ctx, msgs, tools)
}

func TestHarnessForcesWrapUpNearBudget(t *testing.T) {
	model := &toolsSeenModel{scriptedModel: scriptedModel{turns: []llm.Turn{
		call("1", "browser_snapshot", `{}`), call("2", "browser_snapshot", `{}`), call("3", "browser_snapshot", `{}`),
		call("4", "browser_snapshot", `{}`), call("5", "browser_snapshot", `{}`), call("6", "browser_snapshot", `{}`),
		call("7", "browser_snapshot", `{}`), call("8", "submit_offers", goodSubmission),
	}}}
	a := &Agent{Model: model, NewBrowser: func(context.Context) (Browser, error) { return &fakeBrowser{}, nil },
		Budget: Budget{MaxSteps: 10, MaxTokens: 1_000_000, Timeout: time.Minute}}
	run, err := a.Run(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	last := model.offered[len(model.offered)-1]
	if strings.Join(last, ",") != "submit_offers,report_blocked" {
		t.Fatalf("after 7 of 10 steps the model was offered %v, want only the finishing tools", last)
	}
	if len(model.offered[0]) <= 2 {
		t.Fatalf("browser tools missing at the start: %v", model.offered[0])
	}
	found := false
	for _, s := range run.Trace {
		found = found || strings.HasPrefix(s.Note, "harness: budget")
	}
	if !found {
		t.Fatal("wrap-up not recorded in the trace")
	}
}

func TestEmptySubmissionMeansNoListings(t *testing.T) {
	model := &scriptedModel{turns: []llm.Turn{call("1", "submit_offers", `{"offers":[]}`)}}
	run, err := newAgent(model, &fakeBrowser{}, &memRecorder{}, DefaultBudget).Run(context.Background(), task)
	if err != nil || run.Outcome != OutcomeNoOffers || model.calls != 1 || len(run.Offers) != 0 {
		t.Fatalf("outcome %s after %d calls, err %v; want no_offers without an error after 1 call", run.Outcome, model.calls, err)
	}
}

type memObserver struct{ events []live.Event }

func (o *memObserver) Observe(_ context.Context, e live.Event) { o.events = append(o.events, e) }

func TestObserverSeesEachStepAsItHappens(t *testing.T) {
	look := call("1", "browser_navigate", `{"url":"https://www.example-store.test/s?k=x"}`)
	look.Content = "Opening the search results first."
	look.TokensIn, look.TokensOut = 900, 40
	model := &scriptedModel{turns: []llm.Turn{look, call("2", "submit_offers", goodSubmission)}}
	obs := &memObserver{}
	a := newAgent(model, &fakeBrowser{}, &memRecorder{}, DefaultBudget)
	a.Observer = obs
	run, err := a.Run(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}

	if len(obs.events) != 4 {
		t.Fatalf("events: %+v", obs.events)
	}
	start, first, end := obs.events[0], obs.events[1], obs.events[3]
	if start.Kind != live.KindStart || start.Run != run.Key || start.SourceID != task.SourceID || start.Product != task.Product {
		t.Fatalf("start: %+v", start)
	}
	if first.Kind != live.KindStep || first.Index != 1 || first.Step.Tool != "browser_navigate" ||
		first.Step.Thought != "Opening the search results first." || first.Step.TokensIn != 900 {
		t.Fatalf("first step: %+v %+v", first, first.Step)
	}
	if end.Kind != live.KindEnd || end.Outcome != OutcomeOK || end.Offers != len(run.Offers) || end.Run != run.Key {
		t.Fatalf("end: %+v", end)
	}
	if run.Trace[0].Thought == "" {
		t.Fatal("thought not kept in the recorded trace")
	}
}
