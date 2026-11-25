import { FilterBar } from "../Component/FilterBar";
import { useFilters } from "../Model/useFilters";

export function FiltersView() {
  const f = useFilters();
  if (!f.visible) return null;
  return <FilterBar filters={f.filters} regions={f.regions} onChange={f.change} />;
}
