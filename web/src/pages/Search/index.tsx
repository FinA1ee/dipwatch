import { FiltersView } from "./modules/FiltersModule/View/FiltersView";
import { ResultsView } from "./modules/ResultsModule/View/ResultsView";
import { SearchBoxView } from "./modules/SearchBoxModule/View/SearchBoxView";

export function SearchPage() {
  return (
    <div className="space-y-5">
      <SearchBoxView />
      <FiltersView />
      <ResultsView />
    </div>
  );
}
