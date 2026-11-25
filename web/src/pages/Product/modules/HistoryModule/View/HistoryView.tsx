import { useProductPage } from "../../OffersModule/Model/useProductPage";
import { PriceChart } from "../Component/PriceChart";

export function HistoryView() {
  const { page } = useProductPage();
  if (!page) return null;
  return (
    <section className="space-y-2">
      <h2 className="text-lg font-semibold">
        {page.history_of
          ? "Price of this listing per day, last 30 days"
          : "Lowest price per day, last 30 days"}
      </h2>
      <PriceChart history={page.history} />
    </section>
  );
}
