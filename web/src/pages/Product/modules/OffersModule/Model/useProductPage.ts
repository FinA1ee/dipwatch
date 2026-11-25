import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { useSettings } from "@/hooks/useSettings";
import { getProduct } from "@/services/search";
import { useListing } from "./useListing";

/** The product page's data, shared by its modules (one request, cached by React Query). */
export function useProductPage() {
  const productId = Number(useParams().productId);
  const currency = useSettings((s) => s.currency);
  const { listing } = useListing();
  const query = useQuery({
    queryKey: ["product", productId, currency, listing],
    queryFn: () => getProduct(productId, currency, listing ?? undefined),
    // Switching the listing keeps this product's page on screen while the history loads.
    placeholderData: (previous, previousQuery) =>
      previousQuery?.queryKey[1] === productId ? previous : undefined,
    enabled: Number.isInteger(productId) && productId > 0,
  });
  return { page: query.data, loading: query.isPending, error: query.error };
}
