import { describe, expect, it } from "vitest";
import {
  CLIENT_IP_HEADER,
  createApp,
  INTERNAL_TOKEN_HEADER,
  ROLE_HEADER,
  USER_EMAIL_HEADER,
  USER_ID_HEADER,
} from "./app.js";
import type { Auth, SessionUser } from "./auth.js";

const TOKEN = "test-token-0123456789-0123456789-abcdef";

function fakeAuth(user: SessionUser | null): Auth & { handled: Request[] } {
  const handled: Request[] = [];
  return {
    handled,
    handler: async (req) => {
      handled.push(req);
      return Response.json({ ok: true });
    },
    sessionUser: async () => user,
  };
}

function appWithCapture(auth: Auth | null = null, admins: string[] = []) {
  const seen: Request[] = [];
  const app = createApp({
    coreUrl: "http://core:8000",
    internalToken: TOKEN,
    auth,
    adminEmails: new Set(admins),
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
        [USER_ID_HEADER]: "someone-else",
        [ROLE_HEADER]: "admin",
        [CLIENT_IP_HEADER]: "10.0.0.1",
        [INTERNAL_TOKEN_HEADER]: "forged",
        Cookie: "session=abc",
        Authorization: "Bearer abc",
        Accept: "application/json",
      },
    });
    const forwarded = seen[0]?.headers;
    expect(forwarded?.get(INTERNAL_TOKEN_HEADER)).toBe(TOKEN);
    expect(forwarded?.get(USER_ID_HEADER)).toBeNull();
    expect(forwarded?.get(ROLE_HEADER)).toBeNull();
    expect(forwarded?.get(CLIENT_IP_HEADER)).toBeNull();
    expect(forwarded?.get("cookie")).toBeNull();
    expect(forwarded?.get("authorization")).toBeNull();
    expect(forwarded?.get("accept")).toBe("application/json");
  });

  it("forwards the signed-in member, and the admin role only for listed emails", async () => {
    const member = appWithCapture(fakeAuth({ id: "u1", email: "Member@Example.test" }), [
      "admin@example.test",
    ]);
    await member.app.request("/api/search", { method: "POST" });
    expect(member.seen[0]?.headers.get(USER_ID_HEADER)).toBe("u1");
    expect(member.seen[0]?.headers.get(USER_EMAIL_HEADER)).toBe("Member@Example.test");
    expect(member.seen[0]?.headers.get(ROLE_HEADER)).toBeNull();

    const admin = appWithCapture(fakeAuth({ id: "u2", email: "Admin@Example.test" }), ["admin@example.test"]);
    await admin.app.request("/api/admin/runs");
    expect(admin.seen[0]?.headers.get(ROLE_HEADER)).toBe("admin");
  });

  it("passes the client IP Caddy appended, not one the client wrote", async () => {
    const { app, seen } = appWithCapture();
    await app.request("/api/search", {
      method: "POST",
      headers: { "X-Forwarded-For": "6.6.6.6, 198.51.100.7" },
    });
    expect(seen[0]?.headers.get(CLIENT_IP_HEADER)).toBe("198.51.100.7");
  });

  it("sends /api/auth/* to Better Auth, never to core", async () => {
    const auth = fakeAuth(null);
    const { app, seen } = appWithCapture(auth);
    const res = await app.request("/api/auth/sign-in/magic-link", { method: "POST" });
    expect(res.status).toBe(200);
    expect(auth.handled).toHaveLength(1);
    expect(seen).toHaveLength(0);

    const disabled = appWithCapture(null);
    expect((await disabled.app.request("/api/auth/get-session")).status).toBe(503);
  });
});
