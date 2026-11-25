import { Link } from "react-router";
import type { RunSummary } from "@/services/admin";

const columns = ["When", "Kind", "Source", "Prompt", "Outcome", "Offers", "Steps", "Tokens", "Cost", "Time"];

export function RunTable({ runs }: { runs: RunSummary[] }) {
  if (runs.length === 0) return <p className="text-slate-600">No runs match.</p>;
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead className="text-left text-slate-500">
          <tr>
            {columns.map((h) => (
              <th key={h} className="py-1 pr-3 font-normal">
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="tabular-nums">
          {runs.map((r) => (
            <tr key={r.id} className="border-t border-slate-100">
              <td className="whitespace-nowrap py-1 pr-3">
                <Link to={`/admin/runs/${r.id}`} className="hover:underline">
                  {new Date(r.created_at).toLocaleString()}
                </Link>
              </td>
              <td className="py-1 pr-3">{r.kind}</td>
              <td className="py-1 pr-3">{r.source_id ?? "–"}</td>
              <td className="py-1 pr-3">{r.prompt_version}</td>
              <td className="py-1 pr-3">{r.outcome}</td>
              <td className="py-1 pr-3">{r.offers}</td>
              <td className="py-1 pr-3">{r.steps}</td>
              <td className="py-1 pr-3">{(r.tokens_in + r.tokens_out).toLocaleString()}</td>
              <td className="py-1 pr-3">¥{r.cost_cny.toFixed(3)}</td>
              <td className="py-1 pr-3">{(r.duration_ms / 1000).toFixed(1)} s</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
