import { Link, useParams } from "react-router";
import { TraceView } from "./modules/TraceModule/View/TraceView";

export function AdminRunPage() {
  const { runId } = useParams();
  return (
    <div className="space-y-4">
      <div className="flex items-baseline justify-between">
        <h1 className="text-2xl font-semibold">Run {runId}</h1>
        <Link to="/admin/runs" className="text-sm hover:underline">
          All runs
        </Link>
      </div>
      <TraceView />
    </div>
  );
}
