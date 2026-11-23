import { Hono } from "hono";
import { proxy } from "hono/proxy";

export const INTERNAL_TOKEN_HEADER = "x-dipwatch-internal";
const DIPWATCH_HEADER_PREFIX = "x-dipwatch-";

export type GatewayConfig = {
  coreUrl: string;
  internalToken: string;
  /** Overridable for tests. */
  fetch?: (request: Request) => Promise<Response>;
};

export function createApp(config: GatewayConfig) {
  const app = new Hono();

  app.get("/healthz", (c) => c.json({ status: "ok" }));

  app.all("/api/*", (c) => {
    const incoming = new URL(c.req.url);
    const target = `${config.coreUrl}${incoming.pathname.replace(/^\/api/, "")}${incoming.search}`;
    return proxy(target, {
      raw: c.req.raw,
      headers: coreHeaders(c.req.raw.headers, config.internalToken),
      customFetch: config.fetch,
    });
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
