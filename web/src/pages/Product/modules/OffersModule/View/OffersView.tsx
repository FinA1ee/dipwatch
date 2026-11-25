import { Link } from "react-router";
import { OfferTable } from "@/components/OfferTable";
import { PriceLadder } from "@/components/PriceLadder";
import { useListing } from "../Model/useListing";
import { useProductPage } from "../Model/useProductPage";

export function OffersView() {
  const { page, loading, error } = useProductPage();
  const { listing, choose } = useListing();
  if (loading) return <p className="text-muted">Loading…</p>;
  if (error || !page)
    return <p className="rounded-lg bg-rose-50 p-3 text-rose-800">{error?.message ?? "Not found"}</p>;
  return (
    <section className="space-y-6">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h1 className="text-[34px] font-semibold leading-tight">{page.product.name}</h1>
        <p className="text-sm text-muted">
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
      {page.offers.length > 0 && <PriceLadder offers={page.offers} />}
      {page.offers.length > 0 ? (
        <OfferTable
          offers={page.offers}
          selected={listing}
          action={(o) =>
            o.id === listing ? null : (
              <button
                type="button"
                onClick={() => {
                  choose(o.id);
                  document.getElementById("track")?.scrollIntoView({ behavior: "smooth" });
                }}
                className="font-medium text-ink hover:underline"
              >
                Track this listing
              </button>
            )
          }
        />
      ) : (
        <p className="text-muted">No offers yet.</p>
      )}
    </section>
  );
}
