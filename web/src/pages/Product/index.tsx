import { HistoryView } from "./modules/HistoryModule/View/HistoryView";
import { OffersView } from "./modules/OffersModule/View/OffersView";
import { SummaryView } from "./modules/OffersModule/View/SummaryView";
import { TrackView } from "./modules/TrackModule/View/TrackView";

/** Summary first, then tracking, so "Track" is visible without scrolling past every listing. */
export function ProductPage() {
  return (
    <div className="space-y-8">
      <SummaryView />
      <TrackView />
      <HistoryView />
      <OffersView />
    </div>
  );
}
