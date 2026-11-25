import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { getMetrics } from "@/services/admin";

export function useMetrics() {
  const [days, setDays] = useState(7);
  const query = useQuery({
    queryKey: ["admin", "metrics", days],
    queryFn: () => getMetrics(days),
    retry: false,
  });
  return { days, setDays, metrics: query.data, loading: query.isPending, error: query.error };
}
