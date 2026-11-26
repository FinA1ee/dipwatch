import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { useSettings } from "@/hooks/useSettings";
import { useTracked } from "@/hooks/useTracked";
import { getProduct } from "@/services/search";
import { ANY_VARIANT } from "@/utils/variants";
import { useListing } from "../../OffersModule/Model/useListing";
import { useVariant } from "../../OffersModule/Model/useVariant";

/**
 * Price history of the picked listing, else the daily lowest of the picked
 * size. Different sizes are never one line, so "All sizes" and "Size not
 * stated" have no history (scope null). When the page shows what the member
 * tracks, it is the tracker's own history, from the day tracking started.
 */
export function useHistory() {
  const productId = Number(useParams().productId);
  const currency = useSettings((s) => s.currency);
  const { listing } = useListing();
  const { variant } = useVariant();
  const sized = variant !== ANY_VARIANT && variant !== "" ? variant : null;
  const scope = listing ? { listing } : sized ? { variant: sized } : null;
  const tracked = useTracked().items.find((i) => i.product_id === productId);
  const ownHistory =
    tracked &&
    (tracked.listing
      ? tracked.listing.offer_id === listing
      : !listing && (tracked.variant || ANY_VARIANT) === variant);
  const query = useQuery({
    queryKey: ["history", productId, currency, scope],
    queryFn: () => getProduct(productId, currency, scope?.listing, scope?.variant),
    enabled: scope !== null && !ownHistory,
    placeholderData: (previous) => previous,
  });
  if (tracked && ownHistory) {
    return {
      scope: scope ?? { variant: "" },
      since: tracked.tracked_at,
      history: tracked.history.map((d) => ({ day: d.day, converted: d.price })),
    };
  }
  return { scope, since: undefined, history: scope ? (query.data?.history ?? []) : [] };
}
