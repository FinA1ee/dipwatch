import { api, fail, type Schemas } from "./api";

export type Metrics = Schemas["Metrics"];
export type SourceDay = Schemas["SourceDay"];
export type SpendDay = Schemas["SpendDay"];
export type RunSummary = Schemas["Summary"];
export type RunDetail = Schemas["Detail"];
export type RunStep = Schemas["Step"];

export type RunFilter = {
  kind?: "browse" | "match" | "intent";
  source?: string;
  outcome?: string;
  prompt_version?: string;
  day?: string;
  before?: number;
};

export async function getMetrics(days: number): Promise<Metrics> {
  const { data, error, response } = await api.GET("/admin/metrics", { params: { query: { days } } });
  if (!data) fail(response, error);
  return data;
}

export async function listRuns(filter: RunFilter): Promise<RunSummary[]> {
  const { data, error, response } = await api.GET("/admin/runs", {
    params: { query: { ...filter, limit: 50 } },
  });
  if (!data) fail(response, error);
  return data.runs ?? [];
}

export async function getRun(runId: number): Promise<RunDetail> {
  const { data, error, response } = await api.GET("/admin/runs/{runId}", { params: { path: { runId } } });
  if (!data) fail(response, error);
  return data;
}

export type LiveEvent = Schemas["Event"];
export type LiveRun = Schemas["Run"];

export type LiveHandlers = {
  onSnapshot: (runs: LiveRun[]) => void;
  onEvent: (event: LiveEvent) => void;
  onConnection: (connected: boolean) => void;
};

/**
 * Follows browsing-agent runs as they happen (admins only). EventSource
 * reconnects on its own; each reconnect starts with a fresh snapshot.
 */
export function streamLive(handlers: LiveHandlers): () => void {
  const source = new EventSource("/api/admin/live");
  source.onopen = () => handlers.onConnection(true);
  source.onerror = () => handlers.onConnection(false);
  source.addEventListener("snapshot", (e) =>
    handlers.onSnapshot(JSON.parse((e as MessageEvent<string>).data).runs ?? []),
  );
  source.addEventListener("event", (e) => handlers.onEvent(JSON.parse((e as MessageEvent<string>).data)));
  return () => source.close();
}

export type Limits = Schemas["Limits"];

export async function getLimits(): Promise<Limits> {
  const { data, error, response } = await api.GET("/admin/limits");
  if (!data) fail(response, error);
  return data;
}

export async function saveLimits(limits: Limits): Promise<Limits> {
  const { $schema: _, ...body } = limits as Limits & { $schema?: string };
  const { data, error, response } = await api.PUT("/admin/limits", { body });
  if (!data) fail(response, error);
  return data;
}
