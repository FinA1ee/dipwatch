import { create } from "zustand";
import { persist } from "zustand/middleware";

export const CURRENCIES = ["CNY", "USD", "JPY", "EUR", "SGD"] as const;
export type Currency = (typeof CURRENCIES)[number];

type Settings = {
  currency: Currency;
  setCurrency: (next: Currency | ((prev: Currency) => Currency)) => void;
};

/** Display settings shared by every page, kept across reloads (M5.2). */
export const useSettings = create<Settings>()(
  persist(
    (set) => ({
      currency: "CNY",
      setCurrency: (next) => set((s) => ({ currency: typeof next === "function" ? next(s.currency) : next })),
    }),
    { name: "dipwatch-settings" },
  ),
);
