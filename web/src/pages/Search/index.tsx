import { FiltersView } from "./modules/FiltersModule/View/FiltersView";
import { ResultsListView } from "./modules/ResultsModule/View/ResultsListView";
import { ResultsSummaryView } from "./modules/ResultsModule/View/ResultsSummaryView";
import { SearchBoxView } from "./modules/SearchBoxModule/View/SearchBoxView";

export function SearchPage() {
  return (
    <div className="space-y-7">
      <SearchBoxView />
      <ResultsSummaryView />
      <FiltersView />
      <ResultsListView />
    </div>
  );
}
