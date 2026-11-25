import { describe, expect, it } from "vitest";
import type { Offer } from "@/services/api";
import { ANY_VARIANT } from "@/utils/variants";
import { NO_FILTERS } from "../../../store";
import { filterOffers, regionsOf } from "./filterOffers";

function offer(
  id: number,
  region: string,
  condition: Offer["condition"],
  availability: Offer["availability"],
): Offer {
  return {
    id,
    title: `offer ${id}`,
    title_original: `offer ${id}`,
    variant: "24GB",
    source: { id: `s${region}`, name: "Store", region },
    url: "https://example.test",
    price: { minor: 100 * id, currency: "CNY" },
    converted: { minor: 100 * id, currency: "CNY" },
    condition,
    availability,
    seller_type: "unknown",
  };
}

const offers = [
  offer(1, "CN", "new", "in_stock"),
  offer(2, "JP", "used", "unknown"),
  offer(3, "US", "refurbished", "out_of_stock"),
  offer(4, "JP", "unknown", "in_stock"),
];

const ids = (list: Offer[]) => list.map((o) => o.id);

describe("filterOffers", () => {
  it("keeps everything and the ranking without filters", () => {
    expect(ids(filterOffers(offers, NO_FILTERS, ANY_VARIANT))).toEqual([1, 2, 3, 4]);
  });

  it("filters by condition, counting refurbished as used", () => {
    expect(ids(filterOffers(offers, { ...NO_FILTERS, condition: "new" }, ANY_VARIANT))).toEqual([1]);
    expect(ids(filterOffers(offers, { ...NO_FILTERS, condition: "used" }, ANY_VARIANT))).toEqual([2, 3]);
  });

  it("filters by region and stock, keeping unknown stock", () => {
    expect(ids(filterOffers(offers, { ...NO_FILTERS, region: "JP" }, ANY_VARIANT))).toEqual([2, 4]);
    expect(ids(filterOffers(offers, { ...NO_FILTERS, inStockOnly: true }, ANY_VARIANT))).toEqual([1, 2, 4]);
  });

  it("lists regions once, sorted", () => {
    expect(regionsOf(offers)).toEqual(["CN", "JP", "US"]);
  });
});
