/**
 * Where to go after sign-in. Only same-site paths: "//evil.example" and full
 * URLs would turn the sign-in link into an open redirect.
 */
export function safeReturnTo(next: string | null): string {
  if (!next?.startsWith("/") || next.startsWith("//") || next.startsWith("/\\")) return "/";
  return next;
}
