import { create } from "zustand";
import type { Offer, SearchResult } from "@/services/api";

export type Phase = "idle" | "searching" | "streaming" | "done" | "error";

export type Filters = {
  condition: "any" | "new" | "used";
  region: string; // "any" or a source region, e.g. "JP"
  inStockOnly: boolean;
};

export const NO_FILTERS: Filters = { condition: "any", region: "any", inStockOnly: false };

type Update<T> = T | ((prev: T) => T);

type SearchState = {
  query: string;
  phase: Phase;
  product: SearchResult["product"] | null;
  offers: Offer[];
  pending: string[]; // sources still searching
  error: string | null;
  timedOut: boolean;
  paused: boolean; // stored prices only: today's budget is spent
  inviteOnly: boolean; // stored prices only: the viewer is not invited
  filters: Filters;
  setQuery: (next: Update<string>) => void;
  setFilters: (next: Update<Filters>) => void;
  /** Applies a partial state change; setters for the search lifecycle live in Models. */
  patch: (next: Partial<Omit<SearchState, "setQuery" | "setFilters" | "patch">>) => void;
};

function resolve<T>(next: Update<T>, prev: T): T {
  return typeof next === "function" ? (next as (p: T) => T)(prev) : next;
}

/** State shared by the search page's modules: the box, the filters and the results. */
export const useSearchStore = create<SearchState>()((set) => ({
  query: "",
  phase: "idle",
  product: null,
  offers: [],
  pending: [],
  error: null,
  timedOut: false,
  paused: false,
  inviteOnly: false,
  filters: NO_FILTERS,
  setQuery: (next) => set((s) => ({ query: resolve(next, s.query) })),
  setFilters: (next) => set((s) => ({ filters: resolve(next, s.filters) })),
  patch: (next) => set(next),
}));
