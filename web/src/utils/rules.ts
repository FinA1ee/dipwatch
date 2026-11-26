import type { Rule } from "@/services/tracking";
import { formatMoney, minorPerMajor } from "./money";

/** The rule form's fields. Amounts are typed in major units ("1499.50"). */
export type RuleDraft = {
  below: boolean;
  belowAmount: string;
  drop: boolean;
  dropPercent: string;
  change: boolean;
  changePercent: string;
  backInStock: boolean;
  newSource: boolean;
};

export const EMPTY_DRAFT: RuleDraft = {
  below: false,
  belowAmount: "",
  drop: true,
  dropPercent: "10",
  change: false,
  changePercent: "5",
  backInStock: false,
  newSource: false,
};

/** Rules for the API, or an error message for the form. */
export function draftToRules(d: RuleDraft, currency: string): Rule[] | string {
  const rules: Rule[] = [];
  if (d.below) {
    const major = Number(d.belowAmount);
    if (!Number.isFinite(major) || major <= 0) return "Enter a price for “below”.";
    rules.push({ type: "below", below: { minor: Math.round(major * minorPerMajor(currency)), currency } });
  }
  if (d.drop) {
    const pct = Number(d.dropPercent);
    if (!Number.isInteger(pct) || pct < 1 || pct > 90)
      return "The drop must be a whole percentage from 1 to 90.";
    rules.push({ type: "drop", percent: pct });
  }
  if (d.change) {
    const pct = Number(d.changePercent);
    if (!Number.isInteger(pct) || pct < 1 || pct > 90)
      return "The price change must be a whole percentage from 1 to 90.";
    rules.push({ type: "change", percent: pct });
  }
  if (d.backInStock) rules.push({ type: "back_in_stock" });
  if (d.newSource) rules.push({ type: "new_source" });
  return rules.length > 0 ? rules : "Pick at least one rule.";
}

export function rulesToDraft(rules: Rule[]): RuleDraft {
  const d: RuleDraft = { ...EMPTY_DRAFT, drop: false };
  for (const r of rules) {
    if (r.type === "below" && r.below) {
      d.below = true;
      d.belowAmount = String(r.below.minor / minorPerMajor(r.below.currency));
    }
    if (r.type === "drop") {
      d.drop = true;
      d.dropPercent = String(r.percent ?? 10);
    }
    if (r.type === "change") {
      d.change = true;
      d.changePercent = String(r.percent ?? 5);
    }
    if (r.type === "back_in_stock") d.backInStock = true;
    if (r.type === "new_source") d.newSource = true;
  }
  return d;
}

/** "below ¥1,500.00", "10% drop", for lists. */
export function describeRule(r: Rule): string {
  switch (r.type) {
    case "below":
      return r.below ? `below ${formatMoney(r.below)}` : "below";
    case "drop":
      return `${r.percent ?? 0}% drop`;
    case "change":
      return `changes by ${r.percent ?? 0}%`;
    case "back_in_stock":
      return "back in stock";
    case "new_source":
      return "new store";
  }
}
