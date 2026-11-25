import { VariantPicker } from "@/components/VariantPicker";
import { FilterBar } from "../Component/FilterBar";
import { useFilters } from "../Model/useFilters";

export function FiltersView() {
  const f = useFilters();
  if (!f.visible) return null;
  return (
    <div className="space-y-3">
      <VariantPicker variants={f.variants} value={f.variant} onChange={(variant) => f.change({ variant })} />
      <FilterBar filters={f.filters} regions={f.regions} onChange={f.change} />
    </div>
  );
}
