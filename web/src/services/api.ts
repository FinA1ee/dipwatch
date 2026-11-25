import createClient from "openapi-fetch";
import type { components, paths } from "./schema";

/** Typed client for the core API, reached through the gateway at /api. */
export const api = createClient<paths>({ baseUrl: "/api" });

export type Schemas = components["schemas"];
export type Offer = Schemas["Offer"];
export type Money = Schemas["Money"];
export type SearchResult = Schemas["Result"];
export type OffersEvent = Schemas["OffersEvent"];
export type DoneEvent = Schemas["DoneEvent"];
export type ProductPage = Schemas["ProductPage"];
export type DayLow = Schemas["DayLow"];

/** An API error with the message core wrote for people (problem+json `detail`). */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

/** Throws ApiError for a failed response, with core's message when it sent one. */
export function fail(response: Response, error: unknown): never {
  const detail = (error as { detail?: string } | undefined)?.detail;
  throw new ApiError(response.status, detail ?? `Request failed (${response.status})`);
}
