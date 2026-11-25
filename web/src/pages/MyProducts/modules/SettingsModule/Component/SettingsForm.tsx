type Props = {
  webhook: string;
  email: boolean;
  emailAvailable: boolean;
  address: string;
  saving: boolean;
  testing: boolean;
  canTest: boolean;
  message: { ok: boolean; text: string } | null;
  onWebhook: (v: string) => void;
  onEmail: (v: boolean) => void;
  onSave: () => void;
  onTest: () => void;
};

const button =
  "rounded-lg border border-slate-300 px-3 py-1.5 font-medium hover:bg-slate-50 disabled:opacity-50";

export function SettingsForm(p: Props) {
  return (
    <form
      className="space-y-3 text-sm"
      onSubmit={(e) => {
        e.preventDefault();
        p.onSave();
      }}
    >
      <label className="block space-y-1">
        <span className="font-medium">Feishu or Lark bot webhook</span>
        <input
          value={p.webhook}
          onChange={(e) => p.onWebhook(e.target.value)}
          placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/…"
          className="block w-full rounded border border-slate-300 px-2 py-1 font-mono"
        />
        <span className="block text-slate-500">
          In a Feishu group: Settings → Bots → Add bot → Custom bot, then copy its webhook address. Leave
          empty to turn Feishu alerts off.
        </span>
      </label>
      <label className="flex items-center gap-2">
        <input
          type="checkbox"
          checked={p.email && p.emailAvailable}
          disabled={!p.emailAvailable}
          onChange={(e) => p.onEmail(e.target.checked)}
        />
        Email alerts to {p.address}
        {!p.emailAvailable && <span className="text-slate-500">(not available yet on this site)</span>}
      </label>
      <div className="flex flex-wrap items-center gap-3">
        <button type="submit" disabled={p.saving} className={button}>
          Save
        </button>
        <button type="button" onClick={p.onTest} disabled={!p.canTest || p.testing} className={button}>
          Send a test message
        </button>
        {p.message && (
          <span className={p.message.ok ? "text-emerald-700" : "text-rose-700"}>{p.message.text}</span>
        )}
      </div>
    </form>
  );
}
