import { PriceChart } from "@/components/PriceChart";
import { useHistory } from "../Model/useHistory";

/** Price of one listing, or the lowest of one size, per day. */
export function HistoryView() {
  const { scope, since, history } = useHistory();
  if (!scope) {
    return (
      <p className="text-sm text-muted">
        Pick a size above, or press "Track this listing" on one listing, to see its price over time. Listings
        of different sizes are never drawn as one line.
      </p>
    );
  }
  return (
    <section className="space-y-2">
      <h2 className="text-lg font-semibold">
        {since
          ? `Your tracked price since ${new Date(since).toLocaleDateString()}`
          : scope.listing
            ? "Price of this listing per day, last 30 days"
            : `Lowest ${scope.variant} price per day, last 30 days`}
      </h2>
      <PriceChart history={history} />
    </section>
  );
}
