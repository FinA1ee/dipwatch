import { serve } from "@hono/node-server";
import { createApp } from "./app.js";
import { type Auth, createAuth } from "./auth.js";
import { logMailer, smtpMailer } from "./mail.js";

const MIN_SECRET_LENGTH = 32;
const production = process.env.NODE_ENV === "production";

function required(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
}

function secret(name: string): string {
  const value = required(name);
  if (value.length < MIN_SECRET_LENGTH)
    throw new Error(`${name} must be at least ${MIN_SECRET_LENGTH} characters`);
  return value;
}

/**
 * Sign-in needs the auth secret and at least one method: GitHub (client id and
 * secret) or email (SMTP). Without them the site works for visitors only.
 */
function loadAuth(): Auth | null {
  if (!process.env.BETTER_AUTH_SECRET) {
    console.warn(JSON.stringify({ msg: "BETTER_AUTH_SECRET not set: sign-in disabled" }));
    return null;
  }
  const { GITHUB_CLIENT_ID: clientId, GITHUB_CLIENT_SECRET: clientSecret, SMTP_URL: smtpUrl } = process.env;
  const github = clientId && clientSecret ? { clientId, clientSecret } : undefined;
  // Printing magic links to the log is for development only.
  const mailer = smtpUrl ? smtpMailer(smtpUrl, required("MAIL_FROM")) : production ? undefined : logMailer();
  if (!github && !mailer) {
    console.warn(JSON.stringify({ msg: "no sign-in method configured (GitHub or SMTP): sign-in disabled" }));
    return null;
  }
  console.info(JSON.stringify({ msg: "sign-in methods", github: Boolean(github), email: Boolean(mailer) }));
  return createAuth({
    databaseUrl: required("DATABASE_URL"),
    publicUrl: required("PUBLIC_URL"),
    secret: secret("BETTER_AUTH_SECRET"),
    github,
    mailer,
  });
}

const adminEmails = new Set(
  (process.env.ADMIN_EMAILS ?? "")
    .split(",")
    .map((e) => e.trim().toLowerCase())
    .filter(Boolean),
);

const app = createApp({
  coreUrl: required("CORE_URL"),
  internalToken: secret("CORE_INTERNAL_TOKEN"),
  auth: loadAuth(),
  adminEmails,
});
const port = Number(process.env.PORT ?? 3000);

const server = serve({ fetch: app.fetch, port }, (info) => {
  console.info(JSON.stringify({ msg: "gateway listening", port: info.port, admins: adminEmails.size }));
});

for (const signal of ["SIGINT", "SIGTERM"] as const) {
  process.on(signal, () => server.close(() => process.exit(0)));
}
