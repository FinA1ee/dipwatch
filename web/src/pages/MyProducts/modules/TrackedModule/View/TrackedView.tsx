import { Link } from "react-router";
import { useTracked } from "@/hooks/useTracked";
import { formatMoney } from "@/utils/money";
import { describeRule } from "@/utils/rules";

/** The member's tracked products (T1, T2), each linking to its offers and price chart (T7). */
export function TrackedView() {
  const t = useTracked();
  if (t.loading) return <p className="text-slate-600">Loading…</p>;
  if (t.items.length === 0) {
    return (
      <p className="text-slate-600">
        You are not tracking anything yet. Search for a product, open it, and press Track.
      </p>
    );
  }
  return (
    <ul className="divide-y divide-slate-100 rounded-lg border border-slate-200">
      {t.items.map((i) => (
        <li key={i.product_id} className="flex flex-wrap items-start gap-3 p-3">
          <div className="min-w-0 flex-1 space-y-1">
            <Link to={`/p/${i.product_id}`} className="font-medium hover:underline">
              {i.product_name}
            </Link>
            <p className="text-sm text-slate-600">
              Alerts: {i.rules.map(describeRule).join(", ")}
              {i.baseline && ` · lowest when you started: ${formatMoney(i.baseline)}`}
            </p>
            {i.note && <p className="text-sm text-slate-700">{i.note}</p>}
          </div>
          <span className="flex gap-3 text-sm">
            <Link to={`/p/${i.product_id}`} className="hover:underline">
              Prices and chart
            </Link>
            <button type="button" onClick={() => t.remove.mutate(i.product_id)} className="hover:underline">
              Stop
            </button>
          </span>
        </li>
      ))}
    </ul>
  );
}
