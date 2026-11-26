import type { Offer } from "@/services/api";

/** Every variant: no size filter. */
export const ANY_VARIANT = "any";

export type VariantCount = { variant: string; count: number };

/** Variants among the offers, most common first; listings that do not say ("") last. */
export function variantsOf(offers: Offer[]): VariantCount[] {
  const counts = new Map<string, number>();
  for (const o of offers) counts.set(o.variant, (counts.get(o.variant) ?? 0) + 1);
  return [...counts]
    .map(([variant, count]) => ({ variant, count }))
    .sort((a, b) => Number(a.variant === "") - Number(b.variant === "") || b.count - a.count);
}

/**
 * The variant to show before the member picks one: the one named in the query
 * if listings have it, else the most common stated one, else every variant.
 * Prices of different sizes are not comparable, so one is preselected.
 */
export function defaultVariant(offers: Offer[], preferred?: string): string {
  const all = variantsOf(offers);
  if (preferred && all.some((v) => v.variant === preferred)) return preferred;
  return all.find((v) => v.variant !== "")?.variant ?? ANY_VARIANT;
}

export function ofVariant(offers: Offer[], variant: string): Offer[] {
  return variant === ANY_VARIANT ? offers : offers.filter((o) => o.variant === variant);
}

export function variantLabel(variant: string): string {
  return variant === ANY_VARIANT ? "All sizes" : variant || "Size not stated";
}

/** Whether the price can be compared: not unconfirmed, not unusual for its variant. */
export function trusted(o: Offer): boolean {
  return !o.price_unconfirmed && !o.unusual_price;
}
