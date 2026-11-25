import { useQuery } from "@tanstack/react-query";
import { useSettings } from "@/hooks/useSettings";
import { getStores, type Store } from "@/services/search";

/**
 * The stores a search can use and the member's pick, kept valid: stores that
 * were turned off are dropped, the pick never exceeds the per-search limit, and
 * it falls back to the first store.
 */
export function useStores() {
  const query = useQuery({ queryKey: ["stores"], queryFn: getStores, staleTime: 5 * 60_000 });
  const saved = useSettings((s) => s.stores);
  const setStores = useSettings((s) => s.setStores);
  const all: Store[] = query.data?.stores ?? [];
  const perSearch = query.data?.perSearch ?? 1;
  return {
    all,
    perSearch,
    loading: query.isPending,
    selected: validPick(saved, all, perSearch),
    setStores,
  };
}

export function validPick(saved: string[], all: Store[], perSearch: number): string[] {
  const ids = new Set(all.map((s) => s.id));
  const kept = saved.filter((id) => ids.has(id)).slice(0, perSearch);
  if (kept.length > 0) return kept;
  return all[0] ? [all[0].id] : [];
}
