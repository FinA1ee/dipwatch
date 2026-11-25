import { useCallback, useEffect, useRef } from "react";
import { useSearchParams } from "react-router";
import { useSettings } from "@/hooks/useSettings";
import { useStores } from "@/hooks/useStores";
import { search } from "@/services/search";
import { SearchForm } from "../Component/SearchForm";
import { StorePicker } from "../Component/StorePicker";
import { useSearchBox } from "../Model/useSearchBox";

/**
 * Runs searches. The query lives in ?q= so a result can be shared and survives a
 * reload; a currency or store change re-runs the current search (free when cached).
 */
export function SearchBoxView() {
  const box = useSearchBox();
  const currency = useSettings((s) => s.currency);
  const stores = useStores();
  const picked = stores.selected.join(",");
  const [params, setParams] = useSearchParams();
  const urlQuery = params.get("q") ?? "";
  const lastRun = useRef<{ query: string; currency: string; picked: string } | null>(null);
  const { setQuery, start, succeed, fail } = box;

  const run = useCallback(
    async (query: string) => {
      const sameProduct = lastRun.current?.query === query;
      lastRun.current = { query, currency, picked };
      start();
      try {
        succeed(await search(query, currency, picked.split(",")), sameProduct);
      } catch (e) {
        fail(e instanceof Error ? e.message : "Search failed");
      }
    },
    [currency, picked, start, succeed, fail],
  );

  // Runs when the URL query, the currency or the stores change.
  useEffect(() => {
    if (!urlQuery || picked === "") return;
    setQuery(urlQuery);
    const last = lastRun.current;
    if (last?.query !== urlQuery || last.currency !== currency || last.picked !== picked) void run(urlQuery);
  }, [urlQuery, currency, picked, run, setQuery]);

  return (
    <div className="space-y-3">
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
      <StorePicker
        stores={stores.all}
        selected={stores.selected}
        perSearch={stores.perSearch}
        onChange={stores.setStores}
      />
    </div>
  );
}
