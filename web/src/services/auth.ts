import { magicLinkClient } from "better-auth/client/plugins";
import { createAuthClient } from "better-auth/react";

/** Better Auth runs in the gateway at /api/auth, on the same origin as the site. */
const client = createAuthClient({
  baseURL: window.location.origin,
  basePath: "/api/auth",
  plugins: [magicLinkClient()],
});

export type Member = { id: string; email: string };

export type SignInMethods = { email: boolean; github: boolean };

/** Which sign-in methods the gateway has configured. */
export async function signInMethods(): Promise<SignInMethods> {
  const res = await fetch("/api/sign-in-methods");
  if (!res.ok) return { email: false, github: false };
  return res.json();
}

export async function currentMember(): Promise<Member | null> {
  const { data } = await client.getSession();
  return data ? { id: data.user.id, email: data.user.email } : null;
}

/** Sends a sign-in link; the link brings the member back to `returnTo`. */
export async function sendMagicLink(email: string, returnTo: string): Promise<void> {
  const { error } = await client.signIn.magicLink({ email, callbackURL: returnTo });
  if (error) throw new Error(error.message ?? "Could not send the sign-in link");
}

/** Leaves the site for GitHub; it comes back to `returnTo`. */
export async function signInWithGitHub(returnTo: string): Promise<void> {
  const { error } = await client.signIn.social({ provider: "github", callbackURL: returnTo });
  if (error) throw new Error(error.message ?? "GitHub sign-in is not available");
}

export async function signOut(): Promise<void> {
  await client.signOut();
}
