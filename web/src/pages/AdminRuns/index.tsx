import { Link } from "react-router";
import { RunListView } from "./modules/RunListModule/View/RunListView";

export function AdminRunsPage() {
  return (
    <div className="space-y-4">
      <div className="flex items-baseline justify-between">
        <h1 className="text-2xl font-semibold">Agent runs</h1>
        <Link to="/admin" className="text-sm hover:underline">
          Metrics
        </Link>
      </div>
      <RunListView />
    </div>
  );
}
