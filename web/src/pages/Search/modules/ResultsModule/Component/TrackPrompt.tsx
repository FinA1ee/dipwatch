import { Link } from "react-router";
import { ANY_VARIANT } from "@/utils/variants";

type Props = { productId: number; variant: string };

/** The main call to action, right under the price ladder: track the size being compared. */
export function TrackPrompt({ productId, variant }: Props) {
  const sized = variant !== ANY_VARIANT && variant !== "";
  return (
    <aside className="flex flex-wrap items-center gap-x-5 gap-y-3 rounded-[14px] bg-ink px-[22px] py-[18px] text-white">
      <p className="min-w-60 flex-1">
        Get a Feishu message when {sized ? `the ${variant} price` : "the price"} drops below a price you
        choose. To follow one listing only, use "Track this listing" below.
      </p>
      <Link
        to={`/p/${productId}${sized ? `?variant=${encodeURIComponent(variant)}` : ""}#track`}
        className="rounded-[10px] bg-drop px-[18px] py-2.5 font-semibold text-white hover:opacity-90"
      >
        Track {sized ? `the ${variant} price` : "this product"}
      </Link>
    </aside>
  );
}
