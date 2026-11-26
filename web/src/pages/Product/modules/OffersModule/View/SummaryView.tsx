import { PriceLadder } from "@/components/PriceLadder";
import { VariantPicker } from "@/components/VariantPicker";
import { ANY_VARIANT, ofVariant, variantsOf } from "@/utils/variants";
import { useProductPage } from "../Model/useProductPage";
import { useVariant } from "../Model/useVariant";

/**
 * Every listing of the picked size right now, on one price line. Not a chart
 * over time: that is the tracked price above.
 */
export function SummaryView() {
  const { page } = useProductPage();
  const sizes = useVariant();
  if (!page) return null;
  const offers = ofVariant(page.offers, sizes.variant);
  const size = sizes.variant === ANY_VARIANT ? "" : sizes.variant ? `${sizes.variant} ` : "";
  return (
    <section className="space-y-3">
      <h2 className="text-lg font-semibold">All {size}listings right now</h2>
      <VariantPicker variants={variantsOf(page.offers)} value={sizes.variant} onChange={sizes.choose} />
      {offers.length > 0 && <PriceLadder offers={offers} />}
    </section>
  );
}
