import { Link } from "react-router";
import { MetricsView } from "./modules/MetricsModule/View/MetricsView";

export function AdminPage() {
  return (
    <div className="space-y-6">
      <div className="flex items-baseline justify-between">
        <h1 className="text-2xl font-semibold">Agent metrics</h1>
        <Link to="/admin/runs" className="text-sm hover:underline">
          All runs
        </Link>
      </div>
      <MetricsView />
    </div>
  );
}
