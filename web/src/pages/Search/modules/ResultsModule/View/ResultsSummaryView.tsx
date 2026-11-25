import { useEffect } from "react";
import { PriceLadder } from "@/components/PriceLadder";
import { useSettings } from "@/hooks/useSettings";
import { useStores } from "@/hooks/useStores";
import { streamOffers } from "@/services/search";
import { ResultsHeader } from "../Component/ResultsHeader";
import { useResults } from "../Model/useResults";

/** The product, the price ladder, and the stream of updates while a refresh runs (S2). */
export function ResultsSummaryView() {
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
    return <p className="rounded-[10px] bg-rose-50 p-3 text-rose-800">{r.error}</p>;
  }
  if (!r.product || r.phase === "idle") {
    return (
      <div className="pt-6">
        <h1 className="font-head text-[34px] font-semibold leading-tight tracking-tight">
          Find who sells it cheapest.
        </h1>
        <p className="mt-2 max-w-xl text-muted">
          Name a product, pick a store in China or Japan, and see every offer on one price line. Track it to
          get a Feishu message when the price drops.
        </p>
      </div>
    );
  }
  return (
    <section className="space-y-6">
      <ResultsHeader
        productId={r.product.id}
        productName={r.product.name}
        shown={r.shown.length}
        total={r.total}
        pending={r.pending}
        streaming={streaming}
        timedOut={r.timedOut}
      />
      {r.paused && (
        <p className="rounded-[10px] bg-amber-50 p-3 text-sm text-amber-900">
          These prices were not re-checked: today's checks are used up. They update again tomorrow.
        </p>
      )}
      {r.inviteOnly && (
        <p className="rounded-[10px] bg-amber-50 p-3 text-sm text-amber-900">
          These are stored prices. Dipwatch is invite-only for now, so only invited members can have them
          re-checked.
        </p>
      )}
      {r.shown.length > 0 && <PriceLadder offers={r.shown} />}
    </section>
  );
}
