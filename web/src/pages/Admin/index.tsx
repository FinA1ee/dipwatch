import { Link } from "react-router";
import { LimitsView } from "./modules/LimitsModule/View/LimitsView";
import { MetricsView } from "./modules/MetricsModule/View/MetricsView";

export function AdminPage() {
  return (
    <div className="space-y-6">
      <div className="flex items-baseline justify-between">
        <h1 className="text-2xl font-semibold">Agent metrics</h1>
        <span className="space-x-4 text-sm">
          <Link to="/admin/live" className="hover:underline">
            Live log
          </Link>
          <Link to="/admin/runs" className="hover:underline">
            All runs
          </Link>
        </span>
      </div>
      <section className="space-y-3 rounded-lg border border-slate-200 p-3">
        <h2 className="text-lg font-semibold">Budget limits</h2>
        <LimitsView />
      </section>
      <MetricsView />
    </div>
  );
}
