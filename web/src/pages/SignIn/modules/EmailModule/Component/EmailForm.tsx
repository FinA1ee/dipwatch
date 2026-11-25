type Props = {
  email: string;
  sending: boolean;
  error: string;
  onChange: (email: string) => void;
  onSubmit: () => void;
};

export function EmailForm({ email, sending, error, onChange, onSubmit }: Props) {
  return (
    <form
      className="space-y-2"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
    >
      <label className="block text-sm font-medium" htmlFor="email">
        Email
      </label>
      <div className="flex gap-2">
        <input
          id="email"
          type="email"
          required
          autoComplete="email"
          value={email}
          onChange={(e) => onChange(e.target.value)}
          className="flex-1 rounded-[14px] bg-white px-3 py-2"
        />
        <button
          type="submit"
          disabled={sending}
          className="rounded-lg bg-ink px-4 py-2 font-medium text-white disabled:opacity-50"
        >
          {sending ? "Sending…" : "Email me a link"}
        </button>
      </div>
      {error && <p className="text-sm text-rose-700">{error}</p>}
    </form>
  );
}
