import { useState } from "react";
import { sendMagicLink } from "@/services/auth";
import { safeReturnTo } from "../../../returnTo";
import { EmailForm } from "../Component/EmailForm";

/** Magic link: a one-off flow, so local state is enough. */
export function EmailView({ next }: { next: string | null }) {
  const [email, setEmail] = useState("");
  const [state, setState] = useState<"idle" | "sending" | "sent" | "error">("idle");
  const [error, setError] = useState("");

  if (state === "sent") {
    return (
      <p className="rounded-lg bg-drop-soft p-3 text-drop">
        Check {email} for a sign-in link. It works once and expires in 15 minutes.
      </p>
    );
  }
  return (
    <EmailForm
      email={email}
      sending={state === "sending"}
      error={state === "error" ? error : ""}
      onChange={setEmail}
      onSubmit={async () => {
        setState("sending");
        try {
          await sendMagicLink(email.trim(), safeReturnTo(next));
          setState("sent");
        } catch (e) {
          setError(e instanceof Error ? e.message : "Could not send the link");
          setState("error");
        }
      }}
    />
  );
}
