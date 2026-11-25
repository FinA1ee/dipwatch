import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router";
import { listRuns, type RunFilter } from "@/services/admin";

const KEYS = ["kind", "source", "outcome", "prompt_version", "day"] as const;
export type FilterKey = (typeof KEYS)[number];

/** Filters live in the URL, so a filtered view can be linked from the metrics table. */
export function useRunList() {
  const [params, setParams] = useSearchParams();
  const filter: RunFilter = {};
  for (const k of KEYS) {
    const v = params.get(k);
    if (v) (filter as Record<string, string>)[k] = v;
  }
  const before = Number(params.get("before")) || undefined;
  const query = useQuery({
    queryKey: ["admin", "runs", filter, before],
    queryFn: () => listRuns({ ...filter, before }),
    retry: false,
  });

  return {
    filter,
    runs: query.data ?? [],
    loading: query.isPending,
    error: query.error,
    setFilter: (key: FilterKey, value: string) =>
      setParams((prev) => {
        const next = new URLSearchParams(prev);
        if (value) next.set(key, value);
        else next.delete(key);
        next.delete("before"); // a new filter starts from the newest run
        return next;
      }),
    olderThan: (id: number) =>
      setParams((prev) => {
        const next = new URLSearchParams(prev);
        next.set("before", String(id));
        return next;
      }),
    newest:
      before !== undefined
        ? () =>
            setParams((prev) => {
              const next = new URLSearchParams(prev);
              next.delete("before");
              return next;
            })
        : null,
  };
}
