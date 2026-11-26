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
    <div className="min-h-screen">
      <header>
        <nav className="mx-auto flex max-w-[1040px] items-center gap-6 px-6 py-[18px]">
          <Link to="/" className="font-head text-xl font-bold tracking-tight">
            dip<span className="text-drop">watch</span>
          </Link>
          <div className="flex-1" />
          <label className="flex items-center gap-2 text-sm font-medium text-muted">
            <span className="sr-only">Currency</span>
            <select
              value={currency}
              onChange={(e) => setCurrency(e.target.value as Currency)}
              className="bg-transparent font-medium text-muted"
            >
              {CURRENCIES.map((c) => (
                <option key={c}>{c}</option>
              ))}
            </select>
          </label>
          {!loading && member && (
            <span className="flex items-center gap-3 text-sm">
              <Link to="/me" className="hover:underline">
                My products
              </Link>
              <span className="text-muted">{member.email}</span>
              <button type="button" onClick={signOut} className="text-ink hover:underline">
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
      <main className="mx-auto max-w-[1040px] px-6 pt-2 pb-16">
        <Outlet />
      </main>
      <footer className="mx-auto max-w-[1040px] border-t border-rule px-6 py-6 text-sm text-muted">
        <Link to="/privacy" className="hover:underline">
          Privacy
        </Link>
      </footer>
    </div>
  );
}
