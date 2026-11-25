import type { Limits } from "@/services/admin";

type Field = keyof Omit<Limits, "$schema">;

const fields: { key: Field; label: string; hint: string }[] = [
  {
    key: "sources_per_search",
    label: "Stores per search",
    hint: "Each store is one agent or HTML run per uncached search",
  },
  {
    key: "tracked_per_member",
    label: "Tracked products per member",
    hint: "Each is re-checked every 2 to 24 hours",
  },
  {
    key: "visitor_daily_searches",
    label: "Daily new searches per visitor IP",
    hint: "Cached answers are free",
  },
  { key: "member_daily_searches", label: "Daily new searches per member", hint: "Cached answers are free" },
  {
    key: "daily_budget_cny",
    label: "Daily LLM budget (yuan)",
    hint: "At 80% only tracked products get agent runs; at 100% only stored prices are shown until midnight (China time)",
  },
];

type Props = {
  value: Limits;
  saving: boolean;
  message: { ok: boolean; text: string } | null;
  onChange: (next: Limits) => void;
  onSave: () => void;
};

export function LimitsForm({ value, saving, message, onChange, onSave }: Props) {
  return (
    <form
      className="space-y-3 text-sm"
      onSubmit={(e) => {
        e.preventDefault();
        onSave();
      }}
    >
      <div className="grid gap-3 sm:grid-cols-2">
        {fields.map((f) => (
          <label key={f.key} className="block space-y-1">
            <span className="font-medium">{f.label}</span>
            <input
              type="number"
              min={f.key === "sources_per_search" ? 1 : 0}
              step={f.key === "daily_budget_cny" ? 0.5 : 1}
              value={value[f.key]}
              onChange={(e) => onChange({ ...value, [f.key]: Number(e.target.value) })}
              className="block w-28 rounded border border-slate-300 px-2 py-1"
            />
            <span className="block text-slate-500">{f.hint}</span>
          </label>
        ))}
      </div>
      <div className="flex items-center gap-3">
        <button
          type="submit"
          disabled={saving}
          className="rounded-lg border border-slate-300 px-3 py-1.5 font-medium hover:bg-slate-50 disabled:opacity-50"
        >
          Save limits
        </button>
        {message && <span className={message.ok ? "text-emerald-700" : "text-rose-700"}>{message.text}</span>}
      </div>
    </form>
  );
}
