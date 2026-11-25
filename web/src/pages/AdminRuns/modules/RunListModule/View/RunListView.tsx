import { AdminError } from "@/components/AdminError";
import { RunFilterBar } from "../Component/RunFilterBar";
import { RunTable } from "../Component/RunTable";
import { useRunList } from "../Model/useRunList";

export function RunListView() {
  const list = useRunList();
  const oldest = list.runs.at(-1);
  return (
    <div className="space-y-4">
      <RunFilterBar filter={list.filter} onChange={list.setFilter} />
      {list.loading && <p className="text-muted">Loading…</p>}
      {list.error && <AdminError error={list.error} />}
      {!list.loading && !list.error && <RunTable runs={list.runs} />}
      <div className="flex gap-4 text-sm">
        {list.newest && (
          <button type="button" onClick={list.newest} className="hover:underline">
            Newest
          </button>
        )}
        {oldest && list.runs.length >= 50 && (
          <button type="button" onClick={() => list.olderThan(oldest.id)} className="hover:underline">
            Older runs
          </button>
        )}
      </div>
    </div>
  );
}
