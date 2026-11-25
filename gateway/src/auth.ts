import { betterAuth } from "better-auth";
import { magicLink } from "better-auth/plugins";
import { Pool } from "pg";
import type { Mailer } from "./mail.js";

export type AuthConfig = {
  databaseUrl: string;
  /** Site address users see, e.g. http://203.0.113.1:18080. Base of magic links and OAuth callbacks. */
  publicUrl: string;
  secret: string;
  github?: { clientId: string; clientSecret: string };
  mailer: Mailer;
};

/** The parts of Better Auth the app uses, so tests can pass a fake. */
export type Auth = {
  handler: (request: Request) => Promise<Response>;
  sessionUser: (headers: Headers) => Promise<SessionUser | null>;
};

export type SessionUser = { id: string; email: string };

const MAGIC_LINK_MINUTES = 15;

export function createAuth(config: AuthConfig): Auth {
  const auth = betterAuth({
    baseURL: config.publicUrl,
    basePath: "/api/auth",
    secret: config.secret,
    // Tables live in the auth schema, created by core's migrations (00007).
    database: new Pool({ connectionString: config.databaseUrl, options: "-c search_path=auth", max: 4 }),
    socialProviders: config.github ? { github: config.github } : {},
    account: {
      // A2: signing in with GitHub using an email already verified by a magic
      // link joins the same member instead of creating a second one.
      accountLinking: { enabled: true, trustedProviders: ["github"] },
    },
    plugins: [
      magicLink({
        expiresIn: MAGIC_LINK_MINUTES * 60,
        sendMagicLink: async ({ email, url }) => {
          await config.mailer.send({
            to: email,
            subject: "Your Dipwatch sign-in link",
            text: `Open this link to sign in to Dipwatch:\n\n${url}\n\nIt works once and expires in ${MAGIC_LINK_MINUTES} minutes. If you did not ask for it, ignore this email.`,
          });
        },
      }),
    ],
  });

  return {
    handler: (request) => auth.handler(request),
    sessionUser: async (headers) => {
      const session = await auth.api.getSession({ headers });
      return session ? { id: session.user.id, email: session.user.email } : null;
    },
  };
}
