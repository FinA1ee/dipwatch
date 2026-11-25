import type { Filters } from "../../../store";

type Props = {
  filters: Filters;
  regions: string[];
  onChange: (part: Partial<Filters>) => void;
};

const select = "rounded border border-slate-300 px-2 py-1";

export function FilterBar({ filters, regions, onChange }: Props) {
  return (
    <div className="flex flex-wrap items-center gap-4 text-sm text-slate-700">
      <label className="flex items-center gap-2">
        Condition
        <select
          className={select}
          value={filters.condition}
          onChange={(e) => onChange({ condition: e.target.value as Filters["condition"] })}
        >
          <option value="any">Any</option>
          <option value="new">New</option>
          <option value="used">Used or refurbished</option>
        </select>
      </label>
      <label className="flex items-center gap-2">
        Region
        <select
          className={select}
          value={filters.region}
          onChange={(e) => onChange({ region: e.target.value })}
        >
          <option value="any">Any</option>
          {regions.map((r) => (
            <option key={r}>{r}</option>
          ))}
        </select>
      </label>
      <label className="flex items-center gap-2">
        <input
          type="checkbox"
          checked={filters.inStockOnly}
          onChange={(e) => onChange({ inStockOnly: e.target.checked })}
        />
        Hide out of stock
      </label>
    </div>
  );
}
