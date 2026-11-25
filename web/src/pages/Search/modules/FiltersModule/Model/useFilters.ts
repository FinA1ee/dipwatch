import { type Filters, useSearchStore } from "../../../store";
import { regionsOf } from "../../ResultsModule/Model/filterOffers";

export function useFilters() {
  const filters = useSearchStore((s) => s.filters);
  const offers = useSearchStore((s) => s.offers);
  const setFilters = useSearchStore((s) => s.setFilters);
  return {
    filters,
    regions: regionsOf(offers),
    visible: offers.length > 0,
    change: (part: Partial<Filters>) => setFilters((prev) => ({ ...prev, ...part })),
  };
}
