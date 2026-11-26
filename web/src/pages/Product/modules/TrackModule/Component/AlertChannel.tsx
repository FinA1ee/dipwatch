import { useState } from "react";

type Props = {
  webhook: string; // "" when none is saved
  saving: boolean;
  testing: boolean;
  message: { ok: boolean; text: string } | null;
  onSave: (webhook: string) => void;
  onTest: () => void;
};

/**
 * Where alerts go, on the track form itself: without a saved Feishu webhook a
 * tracker would send nothing, so the field is shown right here.
 */
export function AlertChannel({ webhook, saving, testing, message, onSave, onTest }: Props) {
  const [draft, setDraft] = useState("");
  return (
    <div className="space-y-2 rounded-[10px] bg-paper p-3 text-sm">
      {webhook ? (
        <p className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <span>Alerts go to your Feishu group.</span>
          <button
            type="button"
            onClick={onTest}
            disabled={testing}
            className="font-medium underline disabled:opacity-50"
          >
            Send a test message
          </button>
        </p>
      ) : (
        <>
          <p className="font-medium">Where should alerts go?</p>
          <p className="text-muted">
            In a Feishu group, add a custom bot (群设置 → 群机器人 → 自定义机器人), copy its webhook address
            and paste it here. If the bot has a keyword, use "dipwatch".
          </p>
          <div className="flex flex-wrap gap-2">
            <input
              type="url"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/…"
              aria-label="Feishu webhook address"
              className="min-w-0 flex-1 rounded border border-rule bg-white px-2 py-1.5"
            />
            <button
              type="button"
              onClick={() => onSave(draft.trim())}
              disabled={saving || !draft.trim()}
              className="rounded-lg border border-ink bg-white px-3 py-1.5 font-medium disabled:opacity-50"
            >
              Save webhook
            </button>
          </div>
        </>
      )}
      {message && <p className={message.ok ? "text-drop" : "text-rose-700"}>{message.text}</p>}
    </div>
  );
}
