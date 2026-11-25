import { Link } from "react-router";
import { OfferTable } from "@/components/OfferTable";
import { useProductPage } from "../Model/useProductPage";

export function OffersView() {
  const { page, loading, error } = useProductPage();
  if (loading) return <p className="text-slate-600">Loading…</p>;
  if (error || !page)
    return <p className="rounded-lg bg-rose-50 p-3 text-rose-800">{error?.message ?? "Not found"}</p>;
  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h1 className="text-2xl font-semibold">{page.product.name}</h1>
        <p className="text-sm text-slate-600">
          {page.refreshing
            ? "Stores are being searched now."
            : page.last_refreshed
              ? `Checked ${new Date(page.last_refreshed).toLocaleString()}`
              : "Not checked yet."}{" "}
          <Link to={`/?q=${encodeURIComponent(page.product.name)}`} className="hover:underline">
            Search again
          </Link>
        </p>
      </div>
      {page.offers.length > 0 ? (
        <OfferTable offers={page.offers} />
      ) : (
        <p className="text-slate-600">No offers yet.</p>
      )}
    </section>
  );
}
