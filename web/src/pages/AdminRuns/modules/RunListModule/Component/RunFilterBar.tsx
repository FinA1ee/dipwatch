import type { RunFilter } from "@/services/admin";
import type { FilterKey } from "../Model/useRunList";

type Props = { filter: RunFilter; onChange: (key: FilterKey, value: string) => void };

const input = "rounded border border-slate-300 px-2 py-1";

export function RunFilterBar({ filter, onChange }: Props) {
  // Text fields apply on blur or Enter, so typing does not fire a request per key.
  const text = (key: FilterKey, label: string, placeholder: string) => (
    <label className="flex items-center gap-2">
      {label}
      <input
        key={`${key}-${filter[key] ?? ""}`}
        defaultValue={filter[key] ?? ""}
        placeholder={placeholder}
        className={`${input} w-32`}
        onBlur={(e) => onChange(key, e.target.value.trim())}
        onKeyDown={(e) => {
          if (e.key === "Enter") onChange(key, e.currentTarget.value.trim());
        }}
      />
    </label>
  );
  return (
    <div className="flex flex-wrap items-center gap-4 text-sm text-slate-700">
      <label className="flex items-center gap-2">
        Kind
        <select
          className={input}
          value={filter.kind ?? ""}
          onChange={(e) => onChange("kind", e.target.value)}
        >
          <option value="">Any</option>
          <option value="browse">browse</option>
          <option value="match">match</option>
          <option value="intent">intent</option>
        </select>
      </label>
      {text("source", "Source", "smzdm")}
      {text("outcome", "Outcome", "token_limit")}
      {text("prompt_version", "Prompt", "browse-v8")}
      <label className="flex items-center gap-2">
        Day (UTC)
        <input
          type="date"
          className={input}
          value={filter.day ?? ""}
          onChange={(e) => onChange("day", e.target.value)}
        />
      </label>
    </div>
  );
}
