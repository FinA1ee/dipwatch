import { HistoryView } from "./modules/HistoryModule/View/HistoryView";
import { HeaderView } from "./modules/OffersModule/View/HeaderView";
import { OffersView } from "./modules/OffersModule/View/OffersView";
import { SummaryView } from "./modules/OffersModule/View/SummaryView";
import { TrackView } from "./modules/TrackModule/View/TrackView";

/**
 * The price you track over time first, then every listing right now, then
 * tracking settings and the full list.
 */
export function ProductPage() {
  return (
    <div className="space-y-8">
      <HeaderView />
      <HistoryView />
      <SummaryView />
      <TrackView />
      <OffersView />
    </div>
  );
}
