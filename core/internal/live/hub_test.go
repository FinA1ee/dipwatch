package live

import (
	"fmt"
	"strings"
	"testing"
)

func TestHubKeepsRecentRunsAndStreams(t *testing.T) {
	h := &Hub{}
	events, stop := h.Subscribe()
	defer stop()

	h.Add(Event{Run: "a", Kind: KindStart, SourceID: "smzdm"})
	h.Add(Event{Run: "a", Kind: KindStep, Index: 1, Step: &Step{Tool: "browser_navigate"}})
	h.Add(Event{Run: "late", Kind: KindStep, Index: 3}) // start missed: streamed, not stored
	h.Add(Event{Run: "a", Kind: KindEnd, Outcome: "ok"})
	if got := len(events); got != 4 {
		t.Fatalf("streamed %d events, want 4", got)
	}

	recent := h.Recent()
	if len(recent) != 1 || len(recent[0].Steps) != 1 || recent[0].End == nil || recent[0].End.Outcome != "ok" {
		t.Fatalf("recent: %+v", recent)
	}

	for i := range KeepRuns + 5 {
		h.Add(Event{Run: fmt.Sprint(i), Kind: KindStart})
	}
	recent = h.Recent()
	if len(recent) != KeepRuns || recent[0].Start.Run != fmt.Sprint(KeepRuns+4) {
		t.Fatalf("kept %d runs, newest %q", len(recent), recent[0].Start.Run)
	}
}

func TestFitKeepsEventsUnderTheNotifyLimit(t *testing.T) {
	s := Step{Result: strings.Repeat("页", 3000), Arguments: strings.Repeat("a", 900)}.Fit()
	if len(s.Result) > maxResult+len("…") || len(s.Arguments) > maxArguments+len("…") || !strings.HasSuffix(s.Result, "…") {
		t.Fatalf("result %d bytes, arguments %d bytes", len(s.Result), len(s.Arguments))
	}
	if !strings.HasPrefix(s.Result, "页") || strings.ContainsRune(s.Result, '�') {
		t.Fatal("cut inside a character")
	}
}
