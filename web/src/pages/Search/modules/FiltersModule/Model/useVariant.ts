import { defaultVariant } from "@/utils/variants";
import { useSearchStore } from "../../../store";

/** The variant the results show: the member's pick, else the preselected one. */
export function useVariant(): string {
  const picked = useSearchStore((s) => s.filters.variant);
  const offers = useSearchStore((s) => s.offers);
  const queryVariant = useSearchStore((s) => s.queryVariant);
  return picked ?? defaultVariant(offers, queryVariant);
}
