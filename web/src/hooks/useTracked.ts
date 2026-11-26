import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMember } from "@/hooks/useMember";
import { deleteTracked, listTracked, type Rule, saveTracked } from "@/services/tracking";

const KEY = ["tracked"];

/** The member's tracked products, shared by the product page and "My products". */
export function useTracked() {
  const { member } = useMember();
  const client = useQueryClient();
  const query = useQuery({ queryKey: KEY, queryFn: listTracked, enabled: Boolean(member), retry: false });
  const refresh = () => client.invalidateQueries({ queryKey: KEY });
  const save = useMutation({
    mutationFn: (v: {
      productId: number;
      note: string;
      rules: Rule[];
      stores: string[];
      offerId: number | null;
      variant: string;
      checkEveryHours: number;
    }) => saveTracked(v.productId, v.note, v.rules, v.stores, v.offerId, v.variant, v.checkEveryHours),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: (productId: number) => deleteTracked(productId),
    onSuccess: refresh,
  });
  return {
    signedIn: Boolean(member),
    items: query.data ?? [],
    loading: query.isPending && Boolean(member),
    save,
    remove,
  };
}
