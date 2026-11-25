import { Link } from "react-router";
import { LiveView } from "./modules/LiveModule/View/LiveView";

export function AdminLivePage() {
  return (
    <div className="space-y-4">
      <div className="flex items-baseline justify-between">
        <h1 className="text-2xl font-semibold">Live agent log</h1>
        <span className="space-x-4 text-sm">
          <Link to="/admin" className="hover:underline">
            Metrics
          </Link>
          <Link to="/admin/runs" className="hover:underline">
            All runs
          </Link>
        </span>
      </div>
      <LiveView />
    </div>
  );
}
