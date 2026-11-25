import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { getRun } from "@/services/admin";

export function useRun() {
  const runId = Number(useParams().runId);
  const query = useQuery({
    queryKey: ["admin", "run", runId],
    queryFn: () => getRun(runId),
    enabled: Number.isInteger(runId) && runId > 0,
    retry: false,
  });
  return { run: query.data, loading: query.isPending, error: query.error };
}
