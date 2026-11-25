import type { RunStep } from "@/services/admin";

/** The step-by-step trace (H4). Results are untrusted page text: rendered as text only. */
export function StepList({ steps }: { steps: RunStep[] }) {
  if (steps.length === 0)
    return <p className="text-slate-600">This run has no step trace (a single LLM call).</p>;
  // Notes are harness events between tool calls; only tool calls are numbered.
  let call = 0;
  return (
    <ol className="space-y-3">
      {steps.map((s, i) =>
        s.note ? (
          // biome-ignore lint/suspicious/noArrayIndexKey: steps are an ordered log without ids
          <li key={i} className="rounded border border-amber-200 bg-amber-50 p-2 text-sm text-amber-900">
            {s.note}
          </li>
        ) : (
          // biome-ignore lint/suspicious/noArrayIndexKey: steps are an ordered log without ids
          <li key={i} className="rounded border border-slate-200 p-2 text-sm">
            <p className="font-mono font-medium">
              {++call}. {s.tool}
            </p>
            {s.arguments && (
              <pre className="mt-1 whitespace-pre-wrap break-all text-slate-700">{s.arguments}</pre>
            )}
            {s.result && (
              <details className="mt-1">
                <summary className="cursor-pointer text-slate-500">Result</summary>
                <pre className="mt-1 max-h-64 overflow-auto whitespace-pre-wrap break-all text-slate-700">
                  {s.result}
                </pre>
              </details>
            )}
          </li>
        ),
      )}
    </ol>
  );
}
