import { useProductPage } from "../../OffersModule/Model/useProductPage";
import { PriceChart } from "../Component/PriceChart";

/**
 * Price history of one listing. Listings of one model differ too much (48 GB
 * versions, laptops, server cards) for a product-wide line to mean anything,
 * so there is no chart until a listing is picked or tracked.
 */
export function HistoryView() {
  const { page } = useProductPage();
  if (!page) return null;
  if (!page.history_of) {
    return (
      <p className="text-sm text-muted">
        To see a price over time, press "Track this listing" on the listing you want. Listings of the same
        model differ too much in memory, edition and type for one line across all of them to be useful.
      </p>
    );
  }
  return (
    <section className="space-y-2">
      <h2 className="text-lg font-semibold">Price of this listing per day, last 30 days</h2>
      <PriceChart history={page.history} />
    </section>
  );
}
