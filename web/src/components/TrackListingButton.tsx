import { Link } from "react-router";

const style =
  "rounded-full border border-ink px-3 py-0.5 font-semibold text-ink hover:bg-ink hover:text-white focus-visible:bg-ink focus-visible:text-white";

/** "Track this listing" on an offer row: a link to the product page, or a button on it. */
export function TrackListingButton(props: { to: string } | { onClick: () => void }) {
  if ("to" in props) {
    return (
      <Link to={props.to} className={style}>
        Track this listing
      </Link>
    );
  }
  return (
    <button type="button" onClick={props.onClick} className={style}>
      Track this listing
    </button>
  );
}
