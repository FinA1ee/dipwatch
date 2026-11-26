import { variantsOf } from "@/utils/variants";
import { type Filters, useSearchStore } from "../../../store";
import { regionsOf } from "../../ResultsModule/Model/filterOffers";
import { useVariant } from "./useVariant";

export function useFilters() {
  const filters = useSearchStore((s) => s.filters);
  const offers = useSearchStore((s) => s.offers);
  const setFilters = useSearchStore((s) => s.setFilters);
  const variant = useVariant();
  return {
    filters,
    variant,
    variants: variantsOf(offers),
    regions: regionsOf(offers),
    visible: offers.length > 0,
    change: (part: Partial<Filters>) => setFilters((prev) => ({ ...prev, ...part })),
  };
}
