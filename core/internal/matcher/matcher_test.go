package matcher

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/FinA1ee/dipwatch/core/internal/llm"
)

type scripted struct {
	turns []llm.Turn
	calls int
	seen  [][]llm.Message
}

func (m *scripted) Name() string                   { return "scripted" }
func (m *scripted) CostMicros(in, out int64) int64 { return in + out }
func (m *scripted) Chat(_ context.Context, msgs []llm.Message, _ []llm.Tool) (llm.Turn, error) {
	m.seen = append(m.seen, msgs)
	t := m.turns[min(m.calls, len(m.turns)-1)]
	m.calls++
	return t, nil
}

func submit(args string) llm.Turn {
	return llm.Turn{ToolCalls: []llm.ToolCall{{ID: "1", Name: toolName, Arguments: args}}, TokensIn: 500, TokensOut: 50}
}

func TestDecideParsesDecisionsInOrder(t *testing.T) {
	m := &Matcher{Model: &scripted{turns: []llm.Turn{submit(`{"decisions":[
	  {"index":2,"is_product":false,"title_en":"GPU support bracket","condition":"new"},
	  {"index":1,"is_product":true,"title_en":"MSI RTX 5090 32G Ventus 3X OC","condition":"bogus"}]}`)}}}
	got, usage, err := m.Decide(context.Background(), "RTX 5090", []string{"微星 RTX 5090 32G Ventus 3X OC 显卡", "GPU 支架"})
	if err != nil {
		t.Fatal(err)
	}
	if !got[0].IsProduct || got[0].TitleEn != "MSI RTX 5090 32G Ventus 3X OC" || got[0].Condition != "unknown" || got[1].IsProduct {
		t.Fatalf("decisions: %+v", got)
	}
	if usage.Calls != 1 || usage.CostMicros != 550 {
		t.Fatalf("usage: %+v", usage)
	}
}

func TestMissingDecisionIsRetriedOnce(t *testing.T) {
	model := &scripted{turns: []llm.Turn{
		submit(`{"decisions":[{"index":1,"is_product":true,"title_en":"A","condition":"new"}]}`),
		submit(`{"decisions":[{"index":1,"is_product":true,"title_en":"A","condition":"new"},{"index":2,"is_product":false,"title_en":"B","condition":"new"}]}`),
	}}
	got, usage, err := (&Matcher{Model: model}).Decide(context.Background(), "X 1000", []string{"a", "b"})
	if err != nil || len(got) != 2 || usage.Calls != 2 {
		t.Fatalf("got %+v, usage %+v, err %v", got, usage, err)
	}
	last := model.seen[1][len(model.seen[1])-1]
	if !strings.Contains(last.Content, "missing decisions for listings 2") {
		t.Fatalf("retry feedback: %q", last.Content)
	}

	model = &scripted{turns: []llm.Turn{submit(`{"decisions":[]}`)}}
	if _, _, err := (&Matcher{Model: model}).Decide(context.Background(), "X 1000", []string{"a"}); err == nil || model.calls != 2 {
		t.Fatalf("want failure after 2 calls, got err %v after %d", err, model.calls)
	}
}

func TestBatches(t *testing.T) {
	titles := make([]string, BatchSize+5)
	for i := range titles {
		titles[i] = "listing"
	}
	model := &scripted{}
	all := func(n int) llm.Turn {
		var b strings.Builder
		b.WriteString(`{"decisions":[`)
		for i := 1; i <= n; i++ {
			if i > 1 {
				b.WriteString(",")
			}
			b.WriteString(`{"index":` + strconv.Itoa(i) + `,"is_product":true,"title_en":"t","condition":"new"}`)
		}
		b.WriteString("]}")
		return submit(b.String())
	}
	model.turns = []llm.Turn{all(BatchSize), all(5)}
	got, usage, err := (&Matcher{Model: model}).Decide(context.Background(), "X 1000", titles)
	if err != nil || len(got) != len(titles) || usage.Calls != 2 {
		t.Fatalf("got %d decisions in %d calls, err %v", len(got), usage.Calls, err)
	}
}

func TestTitleHashIgnoresCaseAndSpacing(t *testing.T) {
	if TitleHash("MSI  RTX 5090\n") != TitleHash("msi rtx 5090") || TitleHash("a") == TitleHash("b") {
		t.Fatal("hash normalization wrong")
	}
}
