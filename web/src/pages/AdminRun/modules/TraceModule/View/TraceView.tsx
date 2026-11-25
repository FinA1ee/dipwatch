import { AdminError } from "@/components/AdminError";
import { StepList } from "@/components/StepList";
import { RunHeader } from "../Component/RunHeader";
import { useRun } from "../Model/useRun";

export function TraceView() {
  const { run, loading, error } = useRun();
  if (loading) return <p className="text-slate-600">Loading…</p>;
  if (error) return <AdminError error={error} />;
  if (!run) return null;
  return (
    <div className="space-y-6">
      <RunHeader run={run} />
      <section className="space-y-2">
        <h2 className="text-lg font-semibold">Trace</h2>
        <StepList steps={run.trace} />
      </section>
    </div>
  );
}
