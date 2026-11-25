import { Link } from "react-router";
import { OfferTable } from "@/components/OfferTable";
import { useResults } from "../Model/useResults";

/** The ranked offers and the prompt to track the product. */
export function ResultsListView() {
  const r = useResults();
  if (!r.product || r.phase === "idle" || r.phase === "error") return null;
  if (r.shown.length === 0) {
    return (
      <p className="text-muted">
        {r.phase === "streaming" ? "Waiting for the first store…" : "No offers match. Try other filters."}
      </p>
    );
  }
  return (
    <section className="space-y-8">
      <OfferTable
        offers={r.shown}
        action={(o) => (
          <Link
            to={`/p/${r.product?.id}?listing=${o.id}#track`}
            className="font-medium text-ink hover:underline"
          >
            Track this listing
          </Link>
        )}
      />
      <aside className="flex flex-wrap items-center gap-x-5 gap-y-3 rounded-[14px] bg-ink px-[22px] py-[18px] text-white">
        <p className="min-w-60 flex-1">
          Get a Feishu message when the {r.product.name} drops below a price you choose, at any listing or one
          you pick above.
        </p>
        <Link
          to={`/p/${r.product.id}`}
          className="rounded-[10px] bg-white px-[18px] py-2.5 font-semibold text-ink hover:bg-paper"
        >
          Track this product
        </Link>
      </aside>
    </section>
  );
}
