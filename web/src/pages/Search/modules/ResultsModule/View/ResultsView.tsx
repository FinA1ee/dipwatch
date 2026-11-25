import { useEffect } from "react";
import { OfferTable } from "@/components/OfferTable";
import { useSettings } from "@/hooks/useSettings";
import { useStores } from "@/hooks/useStores";
import { streamOffers } from "@/services/search";
import { ResultsHeader } from "../Component/ResultsHeader";
import { useResults } from "../Model/useResults";

/** Shows results and, while a refresh runs, follows its stream (S2). */
export function ResultsView() {
  const r = useResults();
  const currency = useSettings((s) => s.currency);
  const picked = useStores().selected.join(",");
  const productId = r.product?.id;
  const streaming = r.phase === "streaming";
  const { applyOffers, finish } = r;

  useEffect(() => {
    if (!streaming || productId === undefined) return;
    return streamOffers(productId, currency, picked.split(","), { onOffers: applyOffers, onDone: finish });
  }, [streaming, productId, currency, picked, applyOffers, finish]);

  if (r.phase === "error") {
    return <p className="rounded-lg bg-rose-50 p-3 text-rose-800">{r.error}</p>;
  }
  if (!r.product || r.phase === "idle") {
    return (
      <p className="text-slate-600">
        Search for a product to compare prices across stores in China and abroad.
      </p>
    );
  }
  return (
    <section className="space-y-3">
      <ResultsHeader
        productId={r.product.id}
        productName={r.product.name}
        shown={r.shown.length}
        total={r.total}
        pending={r.pending}
        streaming={streaming}
        timedOut={r.timedOut}
      />
      {r.shown.length > 0 ? (
        <OfferTable offers={r.shown} />
      ) : (
        <p className="text-slate-600">
          {streaming ? "Waiting for the first store…" : "No offers match. Try other filters."}
        </p>
      )}
    </section>
  );
}
