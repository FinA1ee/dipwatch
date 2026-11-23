import { serve } from "@hono/node-server";
import { createApp } from "./app.js";

const MIN_TOKEN_LENGTH = 32;

function required(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
}

const internalToken = required("CORE_INTERNAL_TOKEN");
if (internalToken.length < MIN_TOKEN_LENGTH) {
  throw new Error(`CORE_INTERNAL_TOKEN must be at least ${MIN_TOKEN_LENGTH} characters`);
}

const app = createApp({ coreUrl: required("CORE_URL"), internalToken });
const port = Number(process.env.PORT ?? 3000);

const server = serve({ fetch: app.fetch, port }, (info) => {
  console.info(JSON.stringify({ msg: "gateway listening", port: info.port }));
});

for (const signal of ["SIGINT", "SIGTERM"] as const) {
  process.on(signal, () => server.close(() => process.exit(0)));
}
