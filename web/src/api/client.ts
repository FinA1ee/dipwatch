import createClient from "openapi-fetch";
import type { paths } from "./schema";

/** Typed client for the core API, reached through the gateway at /api. */
export const api = createClient<paths>({ baseUrl: "/api" });
