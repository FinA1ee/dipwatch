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
      <label className="flex items-center gap-2 text-sm text-slate-700">
        Store
        <select
          value={selected[0] ?? ""}
          onChange={(e) => onChange([e.target.value])}
          className="rounded border border-slate-300 px-2 py-1"
        >
          {stores.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name} ({s.region})
            </option>
          ))}
        </select>
      </label>
    );
  }
  const toggle = (id: string, on: boolean) => {
    const next = on ? [...selected, id] : selected.filter((s) => s !== id);
    if (next.length >= 1 && next.length <= perSearch) onChange(next);
  };
  return (
    <fieldset className="flex flex-wrap items-center gap-3 text-sm text-slate-700">
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
