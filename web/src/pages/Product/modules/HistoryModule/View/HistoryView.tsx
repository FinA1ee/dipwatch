import { PriceChart } from "../Component/PriceChart";
import { useHistory } from "../Model/useHistory";

/** Price of one listing, or the lowest of one size, per day. */
export function HistoryView() {
  const { scope, history } = useHistory();
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
        {scope.listing ? "Price of this listing per day" : `Lowest ${scope.variant} price per day`}, last 30
        days
      </h2>
      <PriceChart history={history} />
    </section>
  );
}
