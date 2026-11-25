import { Link, Outlet, useLocation } from "react-router";
import { useMember } from "@/hooks/useMember";
import { CURRENCIES, type Currency, useSettings } from "@/hooks/useSettings";

/** App shell: header with search link, currency setting and sign-in state. */
export function Layout() {
  const { member, loading, signOut } = useMember();
  const currency = useSettings((s) => s.currency);
  const setCurrency = useSettings((s) => s.setCurrency);
  const location = useLocation();
  const returnTo = encodeURIComponent(location.pathname + location.search);

  return (
    <div className="min-h-screen bg-white text-slate-900">
      <header className="border-b border-slate-200">
        <nav className="mx-auto flex max-w-4xl items-center gap-4 px-4 py-3">
          <Link to="/" className="text-lg font-semibold">
            Dipwatch
          </Link>
          <div className="flex-1" />
          <label className="flex items-center gap-2 text-sm text-slate-600">
            Currency
            <select
              value={currency}
              onChange={(e) => setCurrency(e.target.value as Currency)}
              className="rounded border border-slate-300 px-2 py-1"
            >
              {CURRENCIES.map((c) => (
                <option key={c}>{c}</option>
              ))}
            </select>
          </label>
          {!loading && member && (
            <span className="flex items-center gap-3 text-sm">
              <span className="text-slate-600">{member.email}</span>
              <button type="button" onClick={signOut} className="text-slate-900 hover:underline">
                Sign out
              </button>
            </span>
          )}
          {!loading && !member && (
            <Link to={`/signin?next=${returnTo}`} className="text-sm hover:underline">
              Sign in
            </Link>
          )}
        </nav>
      </header>
      <main className="mx-auto max-w-4xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  );
}
