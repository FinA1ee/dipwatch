import { Link } from "react-router";
import type { SourceDay } from "@/services/admin";

const pct = (v: number) => `${Math.round(v * 100)}%`;
const yuan = (v: number) => `¥${v.toFixed(3)}`;

export function SourceTable({ rows }: { rows: SourceDay[] }) {
  if (rows.length === 0) return <p className="text-slate-600">No agent runs in this period.</p>;
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead className="text-left text-slate-500">
          <tr>
            {[
              "Day",
              "Source",
              "Runs",
              "Answered",
              "Offers / run",
              "Median steps",
              "Tokens / run",
              "Cost / run",
              "Top failure",
            ].map((h) => (
              <th key={h} className="py-1 pr-3 font-normal">
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="tabular-nums">
          {rows.map((r) => (
            <tr key={`${r.day}-${r.source_id}`} className="border-t border-slate-100">
              <td className="whitespace-nowrap py-1 pr-3">{r.day}</td>
              <td className="py-1 pr-3">
                <Link to={`/admin/runs?source=${r.source_id}&day=${r.day}`} className="hover:underline">
                  {r.source_id}
                </Link>
              </td>
              <td className="py-1 pr-3">{r.runs}</td>
              <td className="py-1 pr-3">{pct(r.success_rate)}</td>
              <td className="py-1 pr-3">{r.offers_per_run.toFixed(1)}</td>
              <td className="py-1 pr-3">{r.median_steps.toFixed(1)}</td>
              <td className="py-1 pr-3">{Math.round(r.tokens_per_run).toLocaleString()}</td>
              <td className="py-1 pr-3">{yuan(r.cost_per_run_cny)}</td>
              <td className="py-1 pr-3">
                {r.top_failure ? (
                  <Link
                    to={`/admin/runs?source=${r.source_id}&day=${r.day}&outcome=${r.top_failure}`}
                    className="hover:underline"
                  >
                    {r.top_failure}
                  </Link>
                ) : (
                  "–"
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
