package agent

import (
	"context"
	"encoding/json"
	"math"

	"github.com/FinA1ee/dipwatch/core/internal/store"
)

// DBRecorder writes each run to app.agent_runs.
type DBRecorder struct {
	Queries *store.Queries
}

func (r DBRecorder) Record(ctx context.Context, run Run) error {
	trace, err := json.Marshal(struct {
		PromptVersion string `json:"prompt_version"`
		Steps         []Step `json:"steps"`
		Offers        int    `json:"offers"`
		Error         string `json:"error,omitempty"`
	}{PromptVersion: PromptVersion, Steps: run.Trace, Offers: len(run.Offers), Error: errText(run.Err)})
	if err != nil {
		return err
	}
	var sourceID *string
	if run.Task.SourceID != "" {
		sourceID = &run.Task.SourceID
	}
	_, err = r.Queries.InsertAgentRun(ctx, store.InsertAgentRunParams{
		Kind:          "browse",
		SourceID:      sourceID,
		ProductID:     run.Task.ProductID,
		Model:         run.Model,
		Steps:         clampInt32(int64(run.Steps)),
		TokensIn:      clampInt32(run.TokensIn),
		TokensOut:     clampInt32(run.TokensOut),
		CostCnyMicros: run.CostMicros,
		DurationMs:    clampInt32(run.Duration.Milliseconds()),
		Outcome:       run.Outcome,
		Trace:         trace,
	})
	return err
}

func clampInt32(v int64) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(v) //nolint:gosec // bounded above
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return truncate(err.Error(), 500)
}
