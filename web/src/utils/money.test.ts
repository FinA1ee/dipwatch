import { describe, expect, it } from "vitest";
import { formatMoney, minorPerMajor } from "./money";

describe("money", () => {
  it("knows each currency's minor unit", () => {
    expect(minorPerMajor("USD")).toBe(100);
    expect(minorPerMajor("JPY")).toBe(1);
  });

  it("formats minor units", () => {
    expect(formatMoney({ minor: 146990, currency: "USD" })).toBe("$1,469.90");
    expect(formatMoney({ minor: 14699, currency: "JPY" })).toBe("¥14,699");
    // CNY and JPY must not look the same.
    expect(formatMoney({ minor: 146990, currency: "CNY" })).toBe("CN¥1,469.90");
    expect(formatMoney({ minor: 1469900, currency: "CNY" })).toBe("CN¥14,699"); // no ".00"
  });
});
