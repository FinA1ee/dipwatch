import { useEffect, useState } from "react";
import { SettingsForm } from "../Component/SettingsForm";
import { useAlertSettings } from "../Model/useAlertSettings";

/** Where alerts go (T6). A form with local state; the Model saves. */
export function SettingsView() {
  const s = useAlertSettings();
  const [webhook, setWebhook] = useState("");
  const [email, setEmail] = useState(false);
  const [message, setMessage] = useState<{ ok: boolean; text: string } | null>(null);

  useEffect(() => {
    if (s.settings) {
      setWebhook(s.settings.feishu_webhook);
      setEmail(s.settings.email_alerts);
    }
  }, [s.settings]);

  if (s.loading) return <p className="text-slate-600">Loading…</p>;
  if (s.error || !s.settings)
    return <p className="text-rose-700">{s.error?.message ?? "Could not load settings"}</p>;

  const saved = s.settings.feishu_webhook;
  return (
    <SettingsForm
      webhook={webhook}
      email={email}
      emailAvailable={s.settings.email_available}
      address={s.settings.email}
      saving={s.save.isPending}
      testing={s.test.isPending}
      canTest={saved !== "" && saved === webhook.trim()}
      message={message}
      onWebhook={setWebhook}
      onEmail={setEmail}
      onSave={() =>
        s.save.mutate(
          { feishu: webhook.trim(), email },
          {
            onSuccess: () => setMessage({ ok: true, text: "Saved." }),
            onError: (e) => setMessage({ ok: false, text: e.message }),
          },
        )
      }
      onTest={() =>
        s.test.mutate(undefined, {
          onSuccess: () => setMessage({ ok: true, text: "Sent. Check the Feishu group." }),
          onError: (e) => setMessage({ ok: false, text: e.message }),
        })
      }
    />
  );
}
