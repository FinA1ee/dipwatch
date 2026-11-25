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
