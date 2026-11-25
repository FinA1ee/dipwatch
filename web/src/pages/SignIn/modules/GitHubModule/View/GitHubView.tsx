import { useState } from "react";
import { signInWithGitHub } from "@/services/auth";
import { safeReturnTo } from "../../../returnTo";

export function GitHubView({ next }: { next: string | null }) {
  const [error, setError] = useState("");
  return (
    <div className="space-y-2">
      <button
        type="button"
        onClick={() =>
          signInWithGitHub(safeReturnTo(next)).catch((e: unknown) =>
            setError(e instanceof Error ? e.message : "GitHub sign-in failed"),
          )
        }
        className="w-full rounded-[14px] bg-white px-4 py-2 font-medium hover:bg-paper"
      >
        Continue with GitHub
      </button>
      {error && <p className="text-sm text-rose-700">{error}</p>}
    </div>
  );
}
