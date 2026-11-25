import { Hono } from "hono";
import { proxy } from "hono/proxy";
import type { Auth } from "./auth.js";

export const INTERNAL_TOKEN_HEADER = "x-dipwatch-internal";
export const USER_ID_HEADER = "x-dipwatch-user-id";
export const USER_EMAIL_HEADER = "x-dipwatch-user-email";
export const ROLE_HEADER = "x-dipwatch-role";
export const CLIENT_IP_HEADER = "x-dipwatch-client-ip";
const DIPWATCH_HEADER_PREFIX = "x-dipwatch-";

export type GatewayConfig = {
  coreUrl: string;
  internalToken: string;
  /** Null when sign-in is not configured; every request is then a visitor. */
  auth: Auth | null;
  /** Lower-case emails that get the admin role. */
  adminEmails: ReadonlySet<string>;
  /** Overridable for tests. */
  fetch?: (request: Request) => Promise<Response>;
};

export function createApp(config: GatewayConfig) {
  const app = new Hono();

  app.get("/healthz", (c) => c.json({ status: "ok" }));

  // Better Auth owns /api/auth/*: sign-in, callbacks, session and sign-out.
  app.all("/api/auth/*", (c) =>
    config.auth ? config.auth.handler(c.req.raw) : c.json({ error: "sign-in is not configured" }, 503),
  );

  app.all("/api/*", async (c) => {
    const incoming = new URL(c.req.url);
    const target = `${config.coreUrl}${incoming.pathname.replace(/^\/api/, "")}${incoming.search}`;
    const user = config.auth ? await config.auth.sessionUser(c.req.raw.headers) : null;
    const headers = coreHeaders(c.req.raw.headers, config.internalToken);
    if (user) {
      headers.set(USER_ID_HEADER, user.id);
      headers.set(USER_EMAIL_HEADER, user.email);
      if (config.adminEmails.has(user.email.toLowerCase())) headers.set(ROLE_HEADER, "admin");
    }
    const ip = clientIp(c.req.raw.headers);
    if (ip) headers.set(CLIENT_IP_HEADER, ip);
    return proxy(target, { raw: c.req.raw, headers, customFetch: config.fetch });
  });

  return app;
}

/**
 * Core trusts X-Dipwatch-* headers, so a client must never be able to set them.
 * Cookies and Authorization stay at the gateway: core never sees credentials.
 */
function coreHeaders(incoming: Headers, internalToken: string): Headers {
  const headers = new Headers(incoming);
  for (const name of [...headers.keys()]) {
    if (name.startsWith(DIPWATCH_HEADER_PREFIX)) headers.delete(name);
  }
  headers.delete("cookie");
  headers.delete("authorization");
  headers.set(INTERNAL_TOKEN_HEADER, internalToken);
  return headers;
}

/**
 * The visitor's IP for quotas. Caddy ignores X-Forwarded-For from clients and
 * appends the address it saw, so the last entry is the one to trust.
 */
function clientIp(headers: Headers): string | null {
  const forwarded = headers.get("x-forwarded-for");
  const last = forwarded?.split(",").at(-1)?.trim();
  return last || null;
}
