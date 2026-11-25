import { useCallback } from "react";
import type { SearchResult } from "@/services/api";
import { NO_FILTERS, useSearchStore } from "../../../store";

/** Reads and writes the search lifecycle for the search box. */
export function useSearchBox() {
  const query = useSearchStore((s) => s.query);
  const phase = useSearchStore((s) => s.phase);
  const product = useSearchStore((s) => s.product);
  const setQuery = useSearchStore((s) => s.setQuery);
  const patch = useSearchStore((s) => s.patch);

  // Stable, because the search view's effects depend on them.
  const start = useCallback(() => patch({ phase: "searching", error: null, timedOut: false }), [patch]);
  /** A new product resets filters; a re-run of the same product (currency change) keeps them. */
  const succeed = useCallback(
    (result: SearchResult, sameProduct: boolean) =>
      patch({
        phase: result.status === "refreshing" ? "streaming" : "done",
        product: result.product,
        offers: result.offers,
        pending: [],
        paused: result.paused ?? false,
        ...(sameProduct ? {} : { filters: NO_FILTERS }),
      }),
    [patch],
  );
  const fail = useCallback((message: string) => patch({ phase: "error", error: message }), [patch]);

  return { query, phase, product, setQuery, start, succeed, fail };
}
