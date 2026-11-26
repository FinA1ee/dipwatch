import { Link, Outlet, useLocation } from "react-router";
import { CurrencyMenu } from "@/components/CurrencyMenu";
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
        <nav className="mx-auto flex max-w-[1040px] items-center gap-4 px-6 py-[18px] sm:gap-6">
          <Link to="/" className="font-head text-xl font-bold tracking-tight">
            dip<span className="text-drop">watch</span>
          </Link>
          <div className="flex-1" />
          <CurrencyMenu value={currency} options={CURRENCIES} onChange={(c) => setCurrency(c as Currency)} />
          {!loading && member && (
            <span className="flex items-center gap-3 text-sm">
              <Link to="/me" className="whitespace-nowrap hover:underline">
                My products
              </Link>
              <span className="hidden text-muted md:inline">{member.email}</span>
              <button type="button" onClick={signOut} className="whitespace-nowrap text-ink hover:underline">
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
