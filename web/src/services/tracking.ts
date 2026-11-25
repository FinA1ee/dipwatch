import { api, fail, type Schemas } from "./api";

export type TrackedItem = Schemas["Item"];
export type TrackedListing = Schemas["Listing"];
export type Rule = Schemas["Rule"];
export type AlertSettings = Schemas["Settings"];

export async function listTracked(): Promise<TrackedItem[]> {
  const { data, error, response } = await api.GET("/me/tracked");
  if (!data) fail(response, error);
  return data.items;
}

/**
 * Tracks a product at the given stores (used when tracking starts), or updates
 * it. With offerId the tracker follows that listing only; null follows the
 * product's cheapest offer of the variant ("" any size).
 */
export async function saveTracked(
  productId: number,
  note: string,
  rules: Rule[],
  stores: string[],
  offerId: number | null,
  variant: string,
): Promise<TrackedItem> {
  const { data, error, response } = await api.PUT("/me/tracked/{productId}", {
    params: { path: { productId } },
    body: { note, rules, stores, ...(offerId ? { offer_id: offerId } : {}), ...(variant ? { variant } : {}) },
  });
  if (!data) fail(response, error);
  return data;
}

export async function deleteTracked(productId: number): Promise<void> {
  const { error, response } = await api.DELETE("/me/tracked/{productId}", {
    params: { path: { productId } },
  });
  if (!response.ok) fail(response, error);
}

export async function getSettings(): Promise<AlertSettings> {
  const { data, error, response } = await api.GET("/me/settings");
  if (!data) fail(response, error);
  return data;
}

export async function saveSettings(feishuWebhook: string, emailAlerts: boolean): Promise<AlertSettings> {
  const { data, error, response } = await api.PUT("/me/settings", {
    body: { feishu_webhook: feishuWebhook, email_alerts: emailAlerts },
  });
  if (!data) fail(response, error);
  return data;
}

export async function sendTestMessage(): Promise<void> {
  const { error, response } = await api.POST("/me/settings/test");
  if (!response.ok) fail(response, error);
}
