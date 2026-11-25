import { AdminError } from "@/components/AdminError";
import { SourceTable } from "../Component/SourceTable";
import { SpendTable } from "../Component/SpendTable";
import { useMetrics } from "../Model/useMetrics";

export function MetricsView() {
  const m = useMetrics();
  return (
    <div className="space-y-6">
      <label className="flex items-center gap-2 text-sm text-muted">
        Last
        <select
          value={m.days}
          onChange={(e) => m.setDays(Number(e.target.value))}
          className="rounded border border-rule px-2 py-1"
        >
          {[1, 7, 30].map((d) => (
            <option key={d} value={d}>
              {d} {d === 1 ? "day" : "days"}
            </option>
          ))}
        </select>
      </label>
      {m.loading && <p className="text-muted">Loading…</p>}
      {m.error && <AdminError error={m.error} />}
      {m.metrics && (
        <>
          <section className="space-y-2">
            <h2 className="text-lg font-semibold">Browsing agent, per source and day</h2>
            <SourceTable rows={m.metrics.sources} />
          </section>
          <section className="space-y-2">
            <h2 className="text-lg font-semibold">LLM spend per day</h2>
            <SpendTable rows={m.metrics.spend} />
          </section>
        </>
      )}
    </div>
  );
}
