import { describe, expect, it } from "vitest";
import { describeRule, draftToRules, EMPTY_DRAFT, rulesToDraft } from "./rules";

describe("rules", () => {
  it("turns the form into API rules, amounts in minor units", () => {
    const rules = draftToRules(
      { ...EMPTY_DRAFT, below: true, belowAmount: "1499.5", dropPercent: "15", backInStock: true },
      "CNY",
    );
    expect(rules).toEqual([
      { type: "below", below: { minor: 149950, currency: "CNY" } },
      { type: "drop", percent: 15 },
      { type: "back_in_stock" },
    ]);
    expect(draftToRules({ ...EMPTY_DRAFT, drop: false, below: true, belowAmount: "50000" }, "JPY")).toEqual([
      { type: "below", below: { minor: 50000, currency: "JPY" } },
    ]);
  });

  it("explains what is wrong instead of sending bad rules", () => {
    expect(draftToRules({ ...EMPTY_DRAFT, drop: false }, "CNY")).toBe("Pick at least one rule.");
    expect(typeof draftToRules({ ...EMPTY_DRAFT, dropPercent: "95" }, "CNY")).toBe("string");
    expect(typeof draftToRules({ ...EMPTY_DRAFT, below: true, belowAmount: "abc" }, "CNY")).toBe("string");
  });

  it("round-trips through the form", () => {
    const rules = [
      { type: "below" as const, below: { minor: 149950, currency: "CNY" } },
      { type: "new_source" as const },
    ];
    expect(draftToRules(rulesToDraft(rules), "CNY")).toEqual(rules);
    expect(rules.map(describeRule)).toEqual(["below CN¥1,499.50", "new store"]);
  });
});
