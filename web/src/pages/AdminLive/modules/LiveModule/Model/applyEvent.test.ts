import { describe, expect, it } from "vitest";
import type { LiveEvent } from "@/services/admin";
import { applyEvent, KEEP_RUNS } from "./applyEvent";

const at = "2026-11-25T08:00:00Z";
const start = (run: string): LiveEvent => ({ run, kind: "start", at, source_id: "smzdm" });
const step = (run: string, index: number): LiveEvent => ({
  run,
  kind: "step",
  at,
  source_id: "smzdm",
  index,
  step: { tool: "browser_find" },
});

describe("applyEvent", () => {
  it("builds a run from start, steps and end, newest run first", () => {
    let runs = applyEvent([], start("a"));
    runs = applyEvent(runs, step("a", 2));
    runs = applyEvent(runs, step("a", 1));
    runs = applyEvent(runs, step("a", 1)); // repeated after a reconnect
    runs = applyEvent(runs, start("b"));
    runs = applyEvent(runs, { run: "a", kind: "end", at, source_id: "smzdm", outcome: "ok" });
    expect(runs.map((r) => r.start.run)).toEqual(["b", "a"]);
    expect(runs[1]?.steps.map((s) => s.index)).toEqual([1, 2]);
    expect(runs[1]?.end?.outcome).toBe("ok");
  });

  it("ignores steps of runs it never saw start, and keeps a bounded list", () => {
    expect(applyEvent([], step("x", 1))).toEqual([]);
    let runs = applyEvent([], start("0"));
    for (let i = 1; i <= KEEP_RUNS + 3; i++) runs = applyEvent(runs, start(String(i)));
    expect(runs).toHaveLength(KEEP_RUNS);
  });
});
