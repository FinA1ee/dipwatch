import { describe, expect, it } from "vitest";
import type { Offer } from "@/services/api";
import { ANY_VARIANT, defaultVariant, ofVariant, variantsOf } from "./variants";

const offer = (id: number, variant: string) => ({ id, variant }) as Offer;
const offers = [offer(1, "24GB"), offer(2, "48GB"), offer(3, ""), offer(4, "24GB"), offer(5, "")];

describe("variants", () => {
  it("counts variants, most common first and unstated last", () => {
    expect(variantsOf(offers)).toEqual([
      { variant: "24GB", count: 2 },
      { variant: "48GB", count: 1 },
      { variant: "", count: 2 },
    ]);
  });
  it("preselects the query's variant, else the most common stated one", () => {
    expect(defaultVariant(offers, "48GB")).toBe("48GB");
    expect(defaultVariant(offers, "96GB")).toBe("24GB");
    expect(defaultVariant([offer(1, "")])).toBe(ANY_VARIANT);
  });
  it("filters by variant", () => {
    expect(ofVariant(offers, "24GB").map((o) => o.id)).toEqual([1, 4]);
    expect(ofVariant(offers, ANY_VARIANT)).toHaveLength(5);
  });
});
