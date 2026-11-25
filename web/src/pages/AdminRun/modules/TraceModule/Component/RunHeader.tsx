import type { RunDetail } from "@/services/admin";

export function RunHeader({ run }: { run: RunDetail }) {
  const facts: [string, string][] = [
    ["Outcome", run.outcome],
    ["Source", run.source_id ?? "–"],
    ["Prompt", run.prompt_version || "–"],
    ["Model", run.model],
    ["Offers", String(run.offers)],
    ["Steps", String(run.steps)],
    ["Tokens in / out", `${run.tokens_in.toLocaleString()} / ${run.tokens_out.toLocaleString()}`],
    ["Cost", `¥${run.cost_cny.toFixed(4)}`],
    ["Duration", `${(run.duration_ms / 1000).toFixed(1)} s`],
    ["When", new Date(run.created_at).toLocaleString()],
  ];
  return (
    <div className="space-y-2">
      <dl className="grid grid-cols-2 gap-x-6 gap-y-1 text-sm sm:grid-cols-5">
        {facts.map(([k, v]) => (
          <div key={k}>
            <dt className="text-slate-500">{k}</dt>
            <dd className="font-medium tabular-nums">{v}</dd>
          </div>
        ))}
      </dl>
      {run.error && <p className="rounded bg-rose-50 p-2 text-sm text-rose-800">{run.error}</p>}
    </div>
  );
}
