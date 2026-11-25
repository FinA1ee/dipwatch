type Props = {
  query: string;
  busy: boolean;
  onChange: (query: string) => void;
  onSubmit: () => void;
};

export function SearchForm({ query, busy, onChange, onSubmit }: Props) {
  return (
    <form
      className="flex gap-2"
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
        className="flex-1 rounded-lg border border-slate-300 px-3 py-2"
      />
      <button
        type="submit"
        disabled={busy || query.trim() === ""}
        className="rounded-lg bg-slate-900 px-4 py-2 font-medium text-white disabled:opacity-50"
      >
        {busy ? "Searching…" : "Search"}
      </button>
    </form>
  );
}
