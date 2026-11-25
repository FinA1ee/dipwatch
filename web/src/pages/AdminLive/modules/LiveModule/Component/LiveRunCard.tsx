import { StepList } from "@/components/StepList";
import type { LiveRun, RunStep } from "@/services/admin";

const outcomeStyle: Record<string, string> = {
  ok: "bg-drop-soft text-drop",
  no_offers: "bg-paper text-muted",
};

export function LiveRunCard({ run }: { run: LiveRun }) {
  const steps = run.steps.map((e) => e.step).filter((s): s is RunStep => Boolean(s));
  const end = run.end;
  return (
    <article className="space-y-3 rounded-[14px] bg-white p-4">
      <header className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="font-semibold">
          {run.start.store || run.start.source_id} · {run.start.product}
        </h2>
        <p className="text-sm text-muted">
          {new Date(run.start.at).toLocaleTimeString()} ·{" "}
          {end ? (
            <span
              className={`rounded px-1.5 py-0.5 ${outcomeStyle[end.outcome ?? ""] ?? "bg-rose-50 text-rose-800"}`}
            >
              {end.outcome}
            </span>
          ) : (
            <span className="rounded bg-sky-50 px-1.5 py-0.5 text-sky-800">running</span>
          )}
          {end &&
            ` · ${end.offers ?? 0} offers · ${((end.tokens_in ?? 0) + (end.tokens_out ?? 0)).toLocaleString()} tokens · ¥${(end.cost_cny ?? 0).toFixed(3)}`}
        </p>
      </header>
      <StepList steps={steps} />
    </article>
  );
}
