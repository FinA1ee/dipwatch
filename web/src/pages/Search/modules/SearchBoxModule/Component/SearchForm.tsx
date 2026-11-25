import type { ReactNode } from "react";

type Props = {
  query: string;
  busy: boolean;
  onChange: (query: string) => void;
  onSubmit: () => void;
  store: ReactNode; // the store picker, between the box and the button
};

export function SearchForm({ query, busy, onChange, onSubmit, store }: Props) {
  return (
    <form
      className="grid gap-2 sm:grid-cols-[1fr_auto_auto]"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
    >
      <input
        value={query}
        onChange={(e) => onChange(e.target.value)}
        placeholder="A product, e.g. RTX 5090 or Sony WH-1000XM6"
        maxLength={80}
        aria-label="Product to search for"
        className="rounded-[10px] border border-rule bg-white px-3.5 py-3 text-[17px] font-medium"
      />
      {store}
      <button
        type="submit"
        disabled={busy || query.trim() === ""}
        className="rounded-[10px] bg-ink px-[22px] py-3 font-semibold text-white disabled:opacity-50"
      >
        {busy ? "Searching…" : "Search"}
      </button>
    </form>
  );
}
