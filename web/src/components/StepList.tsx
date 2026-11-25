import type { RunStep } from "@/services/admin";

const tokens = (s: RunStep) =>
  s.tokens_in ? `${s.tokens_in.toLocaleString()} in / ${(s.tokens_out ?? 0).toLocaleString()} out` : "";

/**
 * A browsing run's steps (H4), used by the run page and the live log. Results
 * are untrusted page text, so everything renders as text only.
 */
export function StepList({ steps }: { steps: RunStep[] }) {
  if (steps.length === 0) return <p className="text-slate-600">No steps yet.</p>;
  // Notes are harness events between tool calls; only tool calls are numbered.
  let call = 0;
  return (
    <ol className="space-y-2">
      {steps.map((s, i) => {
        if (s.note) {
          return (
            // biome-ignore lint/suspicious/noArrayIndexKey: steps are an ordered log without ids
            <li key={i} className="rounded border border-amber-200 bg-amber-50 p-2 text-sm text-amber-900">
              {s.note}
              {tokens(s) && <span className="ml-2 text-amber-700">({tokens(s)})</span>}
            </li>
          );
        }
        call++;
        return (
          // biome-ignore lint/suspicious/noArrayIndexKey: steps are an ordered log without ids
          <li key={i} className="space-y-1 rounded border border-slate-200 p-2 text-sm">
            {s.thought && (
              <p className="rounded bg-sky-50 px-2 py-1 text-sky-900">
                <span className="font-medium">Model: </span>
                {s.thought}
              </p>
            )}
            <p className="flex flex-wrap items-baseline justify-between gap-2">
              <span className="font-mono font-medium">
                {call}. {s.tool}
              </span>
              <span className="text-xs text-slate-500 tabular-nums">
                {[tokens(s), s.ms ? `${(s.ms / 1000).toFixed(1)} s` : ""].filter(Boolean).join(" · ")}
              </span>
            </p>
            {s.arguments && <pre className="whitespace-pre-wrap break-all text-slate-700">{s.arguments}</pre>}
            {s.result && (
              <details>
                <summary className="cursor-pointer text-slate-500">Result</summary>
                <pre className="mt-1 max-h-64 overflow-auto whitespace-pre-wrap break-all text-slate-700">
                  {s.result}
                </pre>
              </details>
            )}
          </li>
        );
      })}
    </ol>
  );
}
