import { useEffect, useState } from "react";
import { AdminError } from "@/components/AdminError";
import type { Limits } from "@/services/admin";
import { LimitsForm } from "../Component/LimitsForm";
import { useLimits } from "../Model/useLimits";

/** Budget limits, applied to the next request without a deploy (task 6.7). */
export function LimitsView() {
  const l = useLimits();
  const [draft, setDraft] = useState<Limits | null>(null);
  const [message, setMessage] = useState<{ ok: boolean; text: string } | null>(null);
  useEffect(() => {
    if (l.limits) setDraft(l.limits);
  }, [l.limits]);

  if (l.error) return <AdminError error={l.error} />;
  if (!draft) return <p className="text-muted">Loading…</p>;
  const levelText: Record<string, string> = {
    normal: "everything runs",
    tracked_only: "over 80%: agent runs only for tracked products",
    stored_only: "budget used up: stored prices only until midnight",
  };
  return (
    <div className="space-y-4">
      {l.spend && (
        <p className="text-sm">
          Today (China time): <strong>¥{l.spend.today_cny.toFixed(2)}</strong> of ¥
          {l.spend.budget_cny.toFixed(2)}, {levelText[l.spend.level]}
        </p>
      )}
      <LimitsForm
        value={draft}
        saving={l.save.isPending}
        message={message}
        onChange={setDraft}
        onSave={() =>
          l.save.mutate(draft, {
            onSuccess: () => setMessage({ ok: true, text: "Saved. New limits apply to the next request." }),
            onError: (e) => setMessage({ ok: false, text: e.message }),
          })
        }
      />
    </div>
  );
}
