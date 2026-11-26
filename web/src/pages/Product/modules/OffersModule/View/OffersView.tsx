import { Link } from "react-router";
import { OfferTable } from "@/components/OfferTable";
import { PriceLadder } from "@/components/PriceLadder";
import { VariantPicker } from "@/components/VariantPicker";
import { ofVariant, variantsOf } from "@/utils/variants";
import { useListing } from "../Model/useListing";
import { useProductPage } from "../Model/useProductPage";
import { useVariant } from "../Model/useVariant";

export function OffersView() {
  const { page, loading, error } = useProductPage();
  const { listing, choose } = useListing();
  const sizes = useVariant();
  if (loading) return <p className="text-muted">Loading…</p>;
  if (error || !page)
    return <p className="rounded-lg bg-rose-50 p-3 text-rose-800">{error?.message ?? "Not found"}</p>;
  const offers = ofVariant(page.offers, sizes.variant);
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
      <VariantPicker variants={variantsOf(page.offers)} value={sizes.variant} onChange={sizes.choose} />
      {offers.length > 0 && <PriceLadder offers={offers} />}
      {offers.length > 0 ? (
        <OfferTable
          offers={offers}
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
