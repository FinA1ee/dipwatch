import { useQuery } from "@tanstack/react-query";
import { api } from "./api/client";

function useHealth() {
  return useQuery({
    queryKey: ["health"],
    queryFn: async () => {
      // 503 still carries a typed body, so read it instead of throwing.
      const { data, error } = await api.GET("/healthz");
      if (data) return data;
      throw new Error(error ? "core unhealthy" : "core unreachable");
    },
    refetchInterval: 15_000,
    retry: false,
  });
}

function StatusRow({ label, value }: { label: string; value: string }) {
  const ok = value === "ok";
  return (
    <li className="flex items-center justify-between py-2">
      <span>{label}</span>
      <span className={ok ? "font-medium text-emerald-600" : "font-medium text-rose-600"}>
        {label}: {value}
      </span>
    </li>
  );
}

export function App() {
  const health = useHealth();

  return (
    <main className="mx-auto max-w-md p-8">
      <h1 className="text-3xl font-semibold">Dipwatch</h1>
      <p className="mt-2 text-slate-600">Track product prices across stores and get alerted on price dips.</p>
      <section className="mt-8 rounded-lg border border-slate-200 p-4">
        <h2 className="text-sm uppercase tracking-wide text-slate-500">System status</h2>
        {health.isPending && <p className="py-2">Checking…</p>}
        {health.isError && <StatusRow label="core" value="unreachable" />}
        {health.data && (
          <ul className="divide-y divide-slate-100">
            <StatusRow label="core" value={health.data.status} />
            <StatusRow label="db" value={health.data.db} />
          </ul>
        )}
      </section>
    </main>
  );
}
