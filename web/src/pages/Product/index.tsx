import { HistoryView } from "./modules/HistoryModule/View/HistoryView";
import { OffersView } from "./modules/OffersModule/View/OffersView";
import { TrackView } from "./modules/TrackModule/View/TrackView";

export function ProductPage() {
  return (
    <div className="space-y-8">
      <OffersView />
      <TrackView />
      <HistoryView />
    </div>
  );
}
