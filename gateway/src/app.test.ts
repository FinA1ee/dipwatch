import { describe, expect, it } from "vitest";
import { createApp, INTERNAL_TOKEN_HEADER } from "./app.js";

const TOKEN = "test-token-0123456789-0123456789-abcdef";

function appWithCapture() {
  const seen: Request[] = [];
  const app = createApp({
    coreUrl: "http://core:8000",
    internalToken: TOKEN,
    fetch: async (req) => {
      seen.push(req);
      return Response.json({ status: "ok", db: "ok" });
    },
  });
  return { app, seen };
}

describe("gateway", () => {
  it("answers its own health check without calling core", async () => {
    const { app, seen } = appWithCapture();
    const res = await app.request("/healthz");
    expect(res.status).toBe(200);
    expect(seen).toHaveLength(0);
  });

  it("forwards /api/* to core without the /api prefix, keeping the query", async () => {
    const { app, seen } = appWithCapture();
    const res = await app.request("/api/healthz?x=1");
    expect(res.status).toBe(200);
    expect(seen[0]?.url).toBe("http://core:8000/healthz?x=1");
  });

  it("adds the internal token and drops spoofed identity headers and credentials", async () => {
    const { app, seen } = appWithCapture();
    await app.request("/api/healthz", {
      headers: {
        "X-Dipwatch-User": "someone-else",
        [INTERNAL_TOKEN_HEADER]: "forged",
        Cookie: "session=abc",
        Authorization: "Bearer abc",
        Accept: "application/json",
      },
    });
    const forwarded = seen[0]?.headers;
    expect(forwarded?.get(INTERNAL_TOKEN_HEADER)).toBe(TOKEN);
    expect(forwarded?.get("x-dipwatch-user")).toBeNull();
    expect(forwarded?.get("cookie")).toBeNull();
    expect(forwarded?.get("authorization")).toBeNull();
    expect(forwarded?.get("accept")).toBe("application/json");
  });
});
