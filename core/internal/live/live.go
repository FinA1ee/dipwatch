// Package live carries browsing-agent progress from the worker to the admin
// page while a run is in progress (Day 6, "live agent log"). The worker sends
// each event with Postgres NOTIFY; core-api keeps recent runs and streams them.
package live

import (
	"time"
	"unicode/utf8"
)

// Channel is the Postgres NOTIFY channel for agent events.
const Channel = "dipwatch_agent"

// Event kinds.
const (
	KindStart = "start"
	KindStep  = "step"
	KindEnd   = "end"
)

// Step is one entry of a run's trace: a tool call or a harness note.
type Step struct {
	Tool      string `json:"tool,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	Result    string `json:"result,omitempty" doc:"Excerpt of what the tool returned"`
	Note      string `json:"note,omitempty" doc:"Harness event, e.g. the budget wrap-up"`
	Thought   string `json:"thought,omitempty" doc:"Text the model wrote with this turn's tool calls"`
	TokensIn  int64  `json:"tokens_in,omitempty" doc:"Tokens of the model call that chose this step (first step of a turn only)"`
	TokensOut int64  `json:"tokens_out,omitempty"`
	Ms        int64  `json:"ms,omitempty" doc:"Time the tool took"`
}

// Event is one update about a run. A start carries the run's identity; a step
// carries its index and the step; an end carries the outcome and totals.
type Event struct {
	Run       string    `json:"run" doc:"Run key, unique per run"`
	Kind      string    `json:"kind" enum:"start,step,end"`
	At        time.Time `json:"at"`
	SourceID  string    `json:"source_id"`
	Store     string    `json:"store,omitempty"`
	Product   string    `json:"product,omitempty"`
	Index     int       `json:"index,omitempty" doc:"Step number, from 1"`
	Step      *Step     `json:"step,omitempty"`
	Outcome   string    `json:"outcome,omitempty"`
	Offers    int       `json:"offers,omitempty"`
	TokensIn  int64     `json:"tokens_in,omitempty"`
	TokensOut int64     `json:"tokens_out,omitempty"`
	CostCNY   float64   `json:"cost_cny,omitempty"`
}

// Postgres caps a NOTIFY payload at 8000 bytes; steps are cut to stay well under.
const (
	maxArguments = 500
	maxResult    = 1500
	maxThought   = 1500
)

// Fit shortens a step's text fields so its event fits in one notification.
func (s Step) Fit() Step {
	s.Arguments = cut(s.Arguments, maxArguments)
	s.Result = cut(s.Result, maxResult)
	s.Thought = cut(s.Thought, maxThought)
	s.Note = cut(s.Note, maxThought)
	return s
}

func cut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "…"
}
