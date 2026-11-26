import { OfferTable } from "@/components/OfferTable";
import { TrackListingButton } from "@/components/TrackListingButton";
import { ofVariant } from "@/utils/variants";
import { useListing } from "../Model/useListing";
import { useProductPage } from "../Model/useProductPage";
import { useVariant } from "../Model/useVariant";

/** Every offer of the picked size, each with "Track this listing". */
export function OffersView() {
  const { page } = useProductPage();
  const { listing, choose } = useListing();
  const { variant } = useVariant();
  if (!page) return null;
  const offers = ofVariant(page.offers, variant);
  if (offers.length === 0) return <p className="text-muted">No offers yet.</p>;
  return (
    <section className="space-y-2">
      <h2 className="text-lg font-semibold">All listings</h2>
      <OfferTable
        offers={offers}
        selected={listing}
        action={(o) =>
          o.id === listing ? null : (
            <TrackListingButton
              onClick={() => {
                choose(o.id);
                document.getElementById("track")?.scrollIntoView({ behavior: "smooth" });
              }}
            />
          )
        }
      />
    </section>
  );
}
