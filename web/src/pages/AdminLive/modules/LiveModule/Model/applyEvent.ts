import type { LiveEvent, LiveRun } from "@/services/admin";

export const KEEP_RUNS = 20;

/** Adds one live event to the run list (newest first), like the server's hub does. */
export function applyEvent(runs: LiveRun[], e: LiveEvent): LiveRun[] {
  if (e.kind === "start") {
    if (runs.some((r) => r.start.run === e.run)) return runs;
    return [{ start: e, steps: [] }, ...runs].slice(0, KEEP_RUNS);
  }
  return runs.map((r) => {
    if (r.start.run !== e.run) return r;
    if (e.kind === "end") return { ...r, end: e };
    // A reconnect can deliver a step that the snapshot already had.
    if (r.steps.some((s) => s.index === e.index)) return r;
    return { ...r, steps: [...r.steps, e].sort((a, b) => (a.index ?? 0) - (b.index ?? 0)) };
  });
}
