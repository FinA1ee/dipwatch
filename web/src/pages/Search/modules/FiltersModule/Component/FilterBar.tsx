import type { Filters } from "../../../store";

type Props = {
  filters: Filters;
  regions: string[];
  onChange: (part: Partial<Filters>) => void;
};

function Chip({ on, onClick, children }: { on: boolean; onClick: () => void; children: string }) {
  return (
    <button
      type="button"
      aria-pressed={on}
      onClick={onClick}
      className={`rounded-full border px-3.5 py-1.5 text-sm font-medium ${
        on ? "border-ink bg-ink text-white" : "border-rule bg-white text-ink hover:border-muted"
      }`}
    >
      {children}
    </button>
  );
}

export function FilterBar({ filters, regions, onChange }: Props) {
  return (
    <fieldset className="flex flex-wrap gap-2">
      <legend className="sr-only">Filter offers</legend>
      <Chip on={filters.condition === "any"} onClick={() => onChange({ condition: "any" })}>
        All conditions
      </Chip>
      <Chip on={filters.condition === "new"} onClick={() => onChange({ condition: "new" })}>
        New only
      </Chip>
      <Chip on={filters.condition === "used"} onClick={() => onChange({ condition: "used" })}>
        Used or refurbished
      </Chip>
      <Chip on={filters.inStockOnly} onClick={() => onChange({ inStockOnly: !filters.inStockOnly })}>
        In stock
      </Chip>
      {regions.length > 1 &&
        regions.map((r) => (
          <Chip
            key={r}
            on={filters.region === r}
            onClick={() => onChange({ region: filters.region === r ? "any" : r })}
          >
            {r}
          </Chip>
        ))}
    </fieldset>
  );
}
