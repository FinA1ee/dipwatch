import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getLimits, getSpend, type Limits, saveLimits } from "@/services/admin";

const KEY = ["admin", "limits"];

export function useLimits() {
  const client = useQueryClient();
  const query = useQuery({ queryKey: KEY, queryFn: getLimits, retry: false });
  const spend = useQuery({
    queryKey: ["admin", "spend"],
    queryFn: getSpend,
    retry: false,
    refetchInterval: 30_000,
  });
  const save = useMutation({
    mutationFn: (l: Limits) => saveLimits(l),
    onSuccess: (data) => {
      client.setQueryData(KEY, data);
      void client.invalidateQueries({ queryKey: ["admin", "spend"] });
      void client.invalidateQueries({ queryKey: ["stores"] }); // the per-search limit is shown there
    },
  });
  return { limits: query.data, spend: spend.data, loading: query.isPending, error: query.error, save };
}
