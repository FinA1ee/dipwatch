package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/FinA1ee/dipwatch/core/internal/llm"
	"github.com/FinA1ee/dipwatch/core/internal/sources"
)

func TestStrategyBuildsTaskFromStore(t *testing.T) {
	model := &scriptedModel{turns: []llm.Turn{call("1", "submit_offers",
		`{"offers":[{"title":"RTX 5090 显卡","price":"¥16,999","currency":"CNY","url":"https://item.jd.com/100.html"}]}`)}}
	s := Strategy{
		Agent: &Agent{Model: model, NewBrowser: func(context.Context) (Browser, error) { return &fakeBrowser{}, nil }},
		Store: Stores["jd"],
	}
	offers, err := s.Search(context.Background(), sources.Query{Product: "RTX 5090", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(offers) != 1 || offers[0].PriceMinor != 1699900 || offers[0].Currency != "CNY" {
		t.Fatalf("offers: %+v", offers)
	}
	prompt := model.lastSeen[1].Content
	if !strings.Contains(prompt, "https://search.jd.com/Search?keyword=RTX+5090&enc=utf-8") || !strings.Contains(prompt, "in CNY") {
		t.Fatalf("task prompt: %s", prompt)
	}
}

func TestEveryStoreHasAValidTemplate(t *testing.T) {
	for id, s := range Stores {
		if s.SourceID != id || !strings.Contains(s.SearchURL, "%s") || len(s.AllowedHosts) == 0 || len(s.Currency) != 3 {
			t.Errorf("store %s: %+v", id, s)
		}
		task := Task{StartURL: strings.Replace(s.SearchURL, "%s", "x", 1), AllowedHosts: s.AllowedHosts}
		if u := task.StartURL; !task.hostAllowed(strings.Split(strings.TrimPrefix(u, "https://"), "/")[0]) {
			t.Errorf("store %s: its own search URL is not on an allowed host", id)
		}
	}
}
