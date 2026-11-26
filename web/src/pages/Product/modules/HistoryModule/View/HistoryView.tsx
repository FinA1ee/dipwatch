import { PriceChart } from "@/components/PriceChart";
import { useHistory } from "../Model/useHistory";

/**
 * The price over time: of what the member tracks, from the day tracking
 * started, else of the picked listing or size.
 */
export function HistoryView() {
  const h = useHistory();
  const { scope, since, history } = h;
  const tracked = "tracked" in h ? h.tracked : undefined;
  if (!scope) {
    return (
      <p className="text-sm text-muted">
        Pick a size below, or press "Track this listing" on one listing, to see its price over time. Listings
        of different sizes are never drawn as one line.
      </p>
    );
  }
  if (tracked && since) {
    const what = tracked.listing
      ? tracked.listing.title
      : `the cheapest ${tracked.variant ? `${tracked.variant} ` : ""}listing`;
    return (
      <section className="space-y-2 rounded-[14px] border-2 border-drop bg-white p-5">
        <h2 className="font-head text-[22px] font-semibold tracking-tight">Your tracked price</h2>
        <p className="text-sm text-muted">
          <span className="font-medium text-ink">{what}</span>, since you started tracking on{" "}
          {new Date(since).toLocaleDateString()}.{" "}
          {tracked.listing ? "Only this listing, at this store." : "Other sizes are left out."}
        </p>
        <PriceChart history={history} />
      </section>
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
