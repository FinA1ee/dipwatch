import { useCallback, useEffect, useRef } from "react";
import { useSearchParams } from "react-router";
import { useSettings } from "@/hooks/useSettings";
import { search } from "@/services/search";
import { SearchForm } from "../Component/SearchForm";
import { useSearchBox } from "../Model/useSearchBox";

/**
 * Runs searches. The query lives in ?q= so a result can be shared and survives a
 * reload; a currency change re-runs the current search (free when cached).
 */
export function SearchBoxView() {
  const box = useSearchBox();
  const currency = useSettings((s) => s.currency);
  const [params, setParams] = useSearchParams();
  const urlQuery = params.get("q") ?? "";
  const lastRun = useRef<{ query: string; currency: string } | null>(null);
  const { setQuery, start, succeed, fail } = box;

  const run = useCallback(
    async (query: string) => {
      const sameProduct = lastRun.current?.query === query;
      lastRun.current = { query, currency };
      start();
      try {
        succeed(await search(query, currency), sameProduct);
      } catch (e) {
        fail(e instanceof Error ? e.message : "Search failed");
      }
    },
    [currency, start, succeed, fail],
  );

  // Runs when the URL query or the currency changes.
  useEffect(() => {
    if (!urlQuery) return;
    setQuery(urlQuery);
    if (lastRun.current?.query !== urlQuery || lastRun.current.currency !== currency) void run(urlQuery);
  }, [urlQuery, currency, run, setQuery]);

  return (
    <SearchForm
      query={box.query}
      busy={box.phase === "searching"}
      onChange={setQuery}
      onSubmit={() => {
        const q = box.query.trim();
        if (q === urlQuery) void run(q);
        else setParams({ q });
      }}
    />
  );
}
