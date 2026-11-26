import { OfferTable } from "@/components/OfferTable";
import { TrackListingButton } from "@/components/TrackListingButton";
import { useResults } from "../Model/useResults";

/** The ranked offers, each with "Track this listing". */
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
  const productId = r.product.id;
  return (
    <OfferTable
      offers={r.shown}
      action={(o) => <TrackListingButton to={`/p/${productId}?listing=${o.id}#track`} />}
    />
  );
}
