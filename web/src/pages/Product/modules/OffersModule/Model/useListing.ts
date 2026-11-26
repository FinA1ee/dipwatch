import { useParams, useSearchParams } from "react-router";
import { useTracked } from "@/hooks/useTracked";

/**
 * The listing (offer id) the product page is about: `?listing=<id>` from a
 * "Track this listing" link, else the listing the member's tracker follows.
 * `?listing=0` or no pin means the product's cheapest offer (null).
 */
export function useListing() {
  const productId = Number(useParams().productId);
  const [params, setParams] = useSearchParams();
  const tracked = useTracked().items.find((i) => i.product_id === productId);
  const raw = params.get("listing");
  const listing = raw === null ? (tracked?.listing?.offer_id ?? null) : Number(raw) || null;
  const choose = (id: number | null) =>
    setParams(
      (p) => {
        p.set("listing", String(id ?? 0));
        p.delete("variant"); // the listing's size, or the tracker's
        return p;
      },
      { replace: true },
    );
  return { listing, choose };
}
