import type { Offer } from "@/services/api";
import type { Filters } from "../../../store";

/** Applies the filter bar. Keeps the server's ranking (cheapest first in CNY). */
export function filterOffers(offers: Offer[], f: Filters): Offer[] {
  return offers.filter(
    (o) =>
      (f.condition === "any" ||
        (f.condition === "new"
          ? o.condition === "new"
          : o.condition === "used" || o.condition === "refurbished")) &&
      (f.region === "any" || o.source.region === f.region) &&
      (!f.inStockOnly || o.availability !== "out_of_stock"),
  );
}

/** Regions present in the results, for the region filter. */
export function regionsOf(offers: Offer[]): string[] {
  return [...new Set(offers.map((o) => o.source.region))].sort();
}
