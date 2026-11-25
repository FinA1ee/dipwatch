import { ANY_VARIANT, type VariantCount, variantLabel } from "@/utils/variants";

type Props = {
  variants: VariantCount[];
  value: string;
  onChange: (variant: string) => void;
};

/**
 * Size chips with listing counts. Prices are compared within one size, so one
 * is always picked; "All sizes" is there for browsing.
 */
export function VariantPicker({ variants, value, onChange }: Props) {
  if (variants.length < 2) return null;
  const options = [...variants, { variant: ANY_VARIANT, count: variants.reduce((n, v) => n + v.count, 0) }];
  return (
    <fieldset className="flex flex-wrap items-center gap-2">
      <legend className="sr-only">Size</legend>
      <span className="mr-1 text-sm font-medium text-muted">Size</span>
      {options.map((v) => (
        <button
          key={v.variant || "none"}
          type="button"
          aria-pressed={value === v.variant}
          onClick={() => onChange(v.variant)}
          className={`rounded-full border px-3.5 py-1.5 text-sm font-medium ${
            value === v.variant
              ? "border-drop bg-drop text-white"
              : "border-rule bg-white text-ink hover:border-muted"
          }`}
        >
          {variantLabel(v.variant)} <span className="opacity-70">({v.count})</span>
        </button>
      ))}
    </fieldset>
  );
}
