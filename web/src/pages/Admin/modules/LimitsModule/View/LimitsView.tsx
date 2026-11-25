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
  if (!draft) return <p className="text-slate-600">Loading…</p>;
  return (
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
  );
}
