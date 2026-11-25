import type { RuleDraft } from "@/utils/rules";

type Props = {
  draft: RuleDraft;
  note: string;
  currency: string;
  onDraft: (next: RuleDraft) => void;
  onNote: (note: string) => void;
};

const box = "rounded border border-rule px-2 py-1";

/** Tracking rules (T1) and note (T2). Pure form: the caller saves. */
export function RuleEditor({ draft, note, currency, onDraft, onNote }: Props) {
  const set = (part: Partial<RuleDraft>) => onDraft({ ...draft, ...part });
  return (
    <div className="space-y-2 text-sm">
      <p className="font-medium">Alert me when</p>
      <label className="flex flex-wrap items-center gap-2">
        <input type="checkbox" checked={draft.below} onChange={(e) => set({ below: e.target.checked })} />
        the price is below
        <input
          inputMode="decimal"
          value={draft.belowAmount}
          onChange={(e) => set({ belowAmount: e.target.value, below: true })}
          className={`${box} w-28`}
          aria-label="Price threshold"
        />
        {currency}
      </label>
      <label className="flex flex-wrap items-center gap-2">
        <input type="checkbox" checked={draft.drop} onChange={(e) => set({ drop: e.target.checked })} />
        the price drops
        <input
          inputMode="numeric"
          value={draft.dropPercent}
          onChange={(e) => set({ dropPercent: e.target.value, drop: true })}
          className={`${box} w-16`}
          aria-label="Drop percentage"
        />
        % from the lowest price when tracking started
      </label>
      <label className="flex items-center gap-2">
        <input
          type="checkbox"
          checked={draft.backInStock}
          onChange={(e) => set({ backInStock: e.target.checked })}
        />
        an offer is back in stock
      </label>
      <label className="flex items-center gap-2">
        <input
          type="checkbox"
          checked={draft.newSource}
          onChange={(e) => set({ newSource: e.target.checked })}
        />
        a new store starts selling it
      </label>
      <label className="block">
        <span className="font-medium">Note</span>
        <textarea
          value={note}
          onChange={(e) => onNote(e.target.value)}
          maxLength={500}
          rows={2}
          placeholder="Why you track it, e.g. for the new PC build"
          className={`${box} mt-1 block w-full`}
        />
      </label>
    </div>
  );
}
