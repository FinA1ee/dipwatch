package agent

import (
	"context"
	"net/url"
	"os"
	"strings"

	"github.com/FinA1ee/dipwatch/core/internal/sources"
)

// Store describes how the agent should approach one store.
type Store struct {
	SourceID     string
	Name         string
	SearchURL    string // %s is replaced with the URL-encoded product name
	AllowedHosts []string
	Currency     string
}

// Stores without an API, from docs/spikes/day1-sources.md.
var Stores = map[string]Store{
	"newegg":       {"newegg", "Newegg", "https://www.newegg.com/p/pl?d=%s", []string{"newegg.com"}, "USD"},
	"bhphoto":      {"bhphoto", "B&H Photo", "https://www.bhphotovideo.com/c/search?q=%s", []string{"bhphotovideo.com"}, "USD"},
	"pcpartpicker": {"pcpartpicker", "PCPartPicker", "https://pcpartpicker.com/search/?q=%s", []string{"pcpartpicker.com"}, "USD"},
	"smzdm":        {"smzdm", "SMZDM 什么值得买", "https://search.smzdm.com/?c=home&s=%s", []string{"smzdm.com"}, "CNY"},
	"jd":           {"jd", "JD.com 京东", "https://search.jd.com/Search?keyword=%s&enc=utf-8", []string{"jd.com"}, "CNY"},
	// From mainland China, amazon.com may redirect to amazon.sg and show SGD.
	"amazon-us": {"amazon-us", "Amazon US", "https://www.amazon.com/s?k=%s", []string{"amazon.com", "amazon.sg"}, "USD"},
	"amazon-jp": {"amazon-jp", "Amazon Japan", "https://www.amazon.co.jp/s?k=%s", []string{"amazon.co.jp"}, "JPY"},
}

// Strategy makes the agent a sources.Strategy named "agent".
type Strategy struct {
	Agent *Agent
	Store Store
}

func (s Strategy) Name() string { return "agent" }

func (s Strategy) Search(ctx context.Context, q sources.Query) ([]sources.Offer, error) {
	run, err := s.Run(ctx, q)
	if err != nil {
		return nil, err
	}
	return run.Offers, nil
}

// Run is Search with the full run record (trace, tokens, outcome), for evaluation.
func (s Strategy) Run(ctx context.Context, q sources.Query) (Run, error) {
	return s.Agent.Run(ctx, Task{
		SourceID:     s.Store.SourceID,
		StoreName:    s.Store.Name,
		StartURL:     strings.Replace(s.Store.SearchURL, "%s", url.QueryEscape(q.Product), 1),
		AllowedHosts: s.Store.AllowedHosts,
		Currency:     s.Store.Currency,
		Product:      q.Product,
		MaxOffers:    q.Limit,
	})
}

// PlaywrightCommand is how the worker starts Playwright MCP. The worker image
// installs it globally; on a dev machine npx fetches the pinned version.
func PlaywrightCommand() []string {
	if v := os.Getenv("PLAYWRIGHT_MCP_COMMAND"); v != "" {
		return strings.Fields(v)
	}
	return []string{"npx", "-y", "@playwright/mcp@0.0.86"}
}
