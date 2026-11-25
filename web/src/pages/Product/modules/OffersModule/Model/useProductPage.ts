import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { useSettings } from "@/hooks/useSettings";
import { getProduct } from "@/services/search";

/** The product page's data, shared by its modules (one request, cached by React Query). */
export function useProductPage() {
  const productId = Number(useParams().productId);
  const currency = useSettings((s) => s.currency);
  const query = useQuery({
    queryKey: ["product", productId, currency],
    queryFn: () => getProduct(productId, currency),
    enabled: Number.isInteger(productId) && productId > 0,
  });
  return { page: query.data, loading: query.isPending, error: query.error };
}
