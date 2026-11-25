import { HistoryView } from "./modules/HistoryModule/View/HistoryView";
import { OffersView } from "./modules/OffersModule/View/OffersView";

export function ProductPage() {
  return (
    <div className="space-y-8">
      <OffersView />
      <HistoryView />
    </div>
  );
}
