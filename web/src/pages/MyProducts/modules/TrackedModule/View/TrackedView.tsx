import { Link } from "react-router";
import { useStores } from "@/hooks/useStores";
import { useTracked } from "@/hooks/useTracked";
import { formatMoney } from "@/utils/money";
import { describeRule } from "@/utils/rules";

/** The member's tracked products (T1, T2), each linking to its offers and price chart (T7). */
export function TrackedView() {
  const t = useTracked();
  const { all } = useStores();
  const storeNames = (ids: string[]) =>
    ids.length === 0 ? "every store" : ids.map((id) => all.find((s) => s.id === id)?.name ?? id).join(", ");
  if (t.loading) return <p className="text-muted">Loading…</p>;
  if (t.items.length === 0) {
    return (
      <p className="text-muted">
        You are not tracking anything yet. Search for a product, open it, and press Track.
      </p>
    );
  }
  return (
    <ul className="divide-y divide-rule rounded-[14px] bg-white">
      {t.items.map((i) => (
        <li key={i.product_id} className="flex flex-wrap items-start gap-3 p-3">
          <div className="min-w-0 flex-1 space-y-1">
            <Link to={`/p/${i.product_id}`} className="font-medium hover:underline">
              {i.product_name}
            </Link>
            <p className="text-sm text-muted">
              At {storeNames(i.stores)} · alerts: {i.rules.map(describeRule).join(", ")}
              {i.baseline && ` · lowest when you started: ${formatMoney(i.baseline)}`}
            </p>
            {i.note && <p className="text-sm text-muted">{i.note}</p>}
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
