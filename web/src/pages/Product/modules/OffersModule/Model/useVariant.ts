import { useParams, useSearchParams } from "react-router";
import { useTracked } from "@/hooks/useTracked";
import { ANY_VARIANT, defaultVariant } from "@/utils/variants";
import { useListing } from "./useListing";
import { useProductPage } from "./useProductPage";

/**
 * The size the product page compares: `?variant=` (a pick, or a link from the
 * search page), else the listing's or the tracker's, else the preselected one.
 */
export function useVariant() {
  const productId = Number(useParams().productId);
  const [params, setParams] = useSearchParams();
  const { page } = useProductPage();
  const { listing } = useListing();
  const tracked = useTracked().items.find((i) => i.product_id === productId);
  const offers = page?.offers ?? [];
  const raw = params.get("variant");
  let variant: string;
  if (raw !== null) variant = raw;
  else if (listing) variant = offers.find((o) => o.id === listing)?.variant ?? ANY_VARIANT;
  else if (tracked) variant = tracked.variant || ANY_VARIANT;
  else variant = defaultVariant(offers);
  const choose = (next: string) =>
    setParams(
      (p) => {
        p.set("variant", next);
        p.set("listing", "0"); // a size replaces a picked listing
        return p;
      },
      { replace: true },
    );
  return { variant, choose };
}
