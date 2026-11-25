import type { SpendDay } from "@/services/admin";

export function SpendTable({ rows }: { rows: SpendDay[] }) {
  if (rows.length === 0) return <p className="text-slate-600">No LLM calls in this period.</p>;
  return (
    <table className="w-full text-sm">
      <thead className="text-left text-slate-500">
        <tr>
          <th className="py-1 pr-3 font-normal">Day</th>
          <th className="py-1 pr-3 font-normal">Kind</th>
          <th className="py-1 pr-3 text-right font-normal">Calls</th>
          <th className="py-1 text-right font-normal">Cost</th>
        </tr>
      </thead>
      <tbody className="tabular-nums">
        {rows.map((r) => (
          <tr key={`${r.day}-${r.kind}`} className="border-t border-slate-100">
            <td className="whitespace-nowrap py-1 pr-3">{r.day}</td>
            <td className="py-1 pr-3">{r.kind}</td>
            <td className="py-1 pr-3 text-right">{r.calls}</td>
            <td className="py-1 text-right">¥{r.cost_cny.toFixed(3)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
