import type { Store } from "@/services/search";

type Props = {
  stores: Store[];
  selected: string[];
  perSearch: number;
  onChange: (ids: string[]) => void;
};

/** Which stores a search uses. One store per search unless the owner allows more. */
export function StorePicker({ stores, selected, perSearch, onChange }: Props) {
  if (stores.length === 0) return null;
  if (perSearch === 1) {
    return (
      <select
        aria-label="Store"
        value={selected[0] ?? ""}
        onChange={(e) => onChange([e.target.value])}
        className="rounded-[10px] border border-rule bg-white px-3.5 py-3 text-[17px] font-medium"
      >
        {stores.map((s) => (
          <option key={s.id} value={s.id}>
            {s.name} ({s.region})
          </option>
        ))}
      </select>
    );
  }
  const toggle = (id: string, on: boolean) => {
    const next = on ? [...selected, id] : selected.filter((s) => s !== id);
    if (next.length >= 1 && next.length <= perSearch) onChange(next);
  };
  return (
    <fieldset className="flex flex-wrap items-center gap-3 rounded-[10px] border border-rule bg-white px-3.5 py-2 text-sm">
      <legend className="sr-only">Stores</legend>
      <span>Stores (up to {perSearch})</span>
      {stores.map((s) => {
        const on = selected.includes(s.id);
        return (
          <label key={s.id} className="flex items-center gap-1">
            <input
              type="checkbox"
              checked={on}
              disabled={!on && selected.length >= perSearch}
              onChange={(e) => toggle(s.id, e.target.checked)}
            />
            {s.name} ({s.region})
          </label>
        );
      })}
    </fieldset>
  );
}
