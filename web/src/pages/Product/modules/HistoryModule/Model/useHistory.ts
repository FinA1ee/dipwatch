import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { useSettings } from "@/hooks/useSettings";
import { getProduct } from "@/services/search";
import { ANY_VARIANT } from "@/utils/variants";
import { useListing } from "../../OffersModule/Model/useListing";
import { useVariant } from "../../OffersModule/Model/useVariant";

/**
 * Price history of the picked listing, else the daily lowest of the picked
 * size. Different sizes are never one line, so "All sizes" and "Size not
 * stated" have no history (scope null).
 */
export function useHistory() {
  const productId = Number(useParams().productId);
  const currency = useSettings((s) => s.currency);
  const { listing } = useListing();
  const { variant } = useVariant();
  const sized = variant !== ANY_VARIANT && variant !== "" ? variant : null;
  const scope = listing ? { listing } : sized ? { variant: sized } : null;
  const query = useQuery({
    queryKey: ["history", productId, currency, scope],
    queryFn: () => getProduct(productId, currency, scope?.listing, scope?.variant),
    enabled: scope !== null,
    placeholderData: (previous) => previous,
  });
  return { scope, history: scope ? (query.data?.history ?? []) : [] };
}
