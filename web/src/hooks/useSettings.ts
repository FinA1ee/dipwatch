import { create } from "zustand";
import { persist } from "zustand/middleware";

export const CURRENCIES = ["CNY", "USD", "JPY", "EUR", "SGD"] as const;
export type Currency = (typeof CURRENCIES)[number];

type Settings = {
  currency: Currency;
  /** Stores picked for searches and new trackers (budget limit: usually one). */
  stores: string[];
  setCurrency: (next: Currency | ((prev: Currency) => Currency)) => void;
  setStores: (next: string[] | ((prev: string[]) => string[])) => void;
};

/** Settings shared by every page, kept across reloads (M5.2). */
export const useSettings = create<Settings>()(
  persist(
    (set) => ({
      currency: "CNY",
      stores: [],
      setCurrency: (next) => set((s) => ({ currency: typeof next === "function" ? next(s.currency) : next })),
      setStores: (next) => set((s) => ({ stores: typeof next === "function" ? next(s.stores) : next })),
    }),
    { name: "dipwatch-settings" },
  ),
);
