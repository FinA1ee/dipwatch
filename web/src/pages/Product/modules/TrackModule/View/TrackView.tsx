import { useEffect, useState } from "react";
import { Link, useLocation, useParams } from "react-router";
import { RuleEditor } from "@/components/RuleEditor";
import { useSettings } from "@/hooks/useSettings";
import { useStores } from "@/hooks/useStores";
import { useTracked } from "@/hooks/useTracked";
import { draftToRules, EMPTY_DRAFT, type RuleDraft, rulesToDraft } from "@/utils/rules";
import { useListing } from "../../OffersModule/Model/useListing";
import { useProductPage } from "../../OffersModule/Model/useProductPage";

/** Track this product, or one of its listings: rules, note, save or stop (T1, T2). */
export function TrackView() {
  const productId = Number(useParams().productId);
  const { hash } = useLocation();
  const t = useTracked();
  const currency = useSettings((s) => s.currency);
  const stores = useStores();
  const { listing, choose } = useListing();
  const { page } = useProductPage();
  const current = t.items.find((i) => i.product_id === productId);
  const [draft, setDraft] = useState<RuleDraft>(EMPTY_DRAFT);
  const [note, setNote] = useState("");
  const [problem, setProblem] = useState("");

  // Load the saved tracker into the form once it arrives.
  useEffect(() => {
    if (current) {
      setDraft(rulesToDraft(current.rules));
      setNote(current.note);
    }
  }, [current]);

  // A "Track this listing" link from the search page lands here.
  useEffect(() => {
    if (hash === "#track" && page) document.getElementById("track")?.scrollIntoView();
  }, [hash, page]);

  if (!t.signedIn) {
    return (
      <section id="track" className="rounded-[14px] bg-white p-4 text-sm">
        <Link
          to={`/signin?next=${encodeURIComponent(`/p/${productId}${listing ? `?listing=${listing}` : ""}`)}`}
          className="font-medium hover:underline"
        >
          Sign in
        </Link>{" "}
        to track this product and get an alert when the price drops.
      </section>
    );
  }

  const offer = page?.offers.find((o) => o.id === listing);
  const pinned = current?.listing;
  const listingTitle = offer?.title ?? (pinned?.offer_id === listing ? pinned?.title : undefined);
  const listingStore =
    offer?.source.name ??
    (pinned?.offer_id === listing
      ? (stores.all.find((st) => st.id === pinned?.store)?.name ?? pinned?.store)
      : undefined);
  const storeNames =
    (current && current.stores.length > 0 ? current.stores : stores.selected)
      .map((id) => stores.all.find((s) => s.id === id)?.name ?? id)
      .join(", ") || "every store";

  const save = () => {
    const rules = draftToRules(draft, currency);
    if (typeof rules === "string") return setProblem(rules);
    setProblem("");
    t.save.mutate({ productId, note: note.trim(), rules, stores: stores.selected, offerId: listing });
  };
  const error = problem || t.save.error?.message || t.remove.error?.message;

  return (
    <section id="track" className="space-y-3 rounded-[14px] bg-white p-4">
      <h2 className="text-lg font-semibold">
        {current ? "You track this product" : listing ? "Track this listing" : "Track this product"}
      </h2>
      {listing ? (
        <p className="text-sm text-muted">
          Follows one listing:{" "}
          <span className="font-medium text-ink">{listingTitle ?? `offer ${listing}`}</span>
          {listingStore && ` at ${listingStore}`}. Alerts are about this listing only.{" "}
          <button type="button" onClick={() => choose(null)} className="font-medium text-ink hover:underline">
            Follow the cheapest listing instead
          </button>
        </p>
      ) : (
        <p className="text-sm text-muted">
          Follows the cheapest listing at {storeNames}
          {!current && " (the store picked on the search page)"}. Pick one listing in the list above to follow
          only that one.
        </p>
      )}
      {pinned?.gone && pinned.offer_id === listing && (
        <p className="text-sm text-warn">
          The store no longer lists it. Pick another listing above, or follow the cheapest listing.
        </p>
      )}
      <p className="text-sm text-muted">Dipwatch re-checks it every few hours.</p>
      <RuleEditor draft={draft} note={note} currency={currency} onDraft={setDraft} onNote={setNote} />
      {error && <p className="text-sm text-rose-700">{error}</p>}
      <div className="flex flex-wrap items-center gap-3 text-sm">
        <button
          type="button"
          onClick={save}
          disabled={t.save.isPending}
          className="rounded-lg bg-ink px-4 py-2 font-medium text-white disabled:opacity-50"
        >
          {current ? "Save changes" : "Track"}
        </button>
        {current && (
          <button type="button" onClick={() => t.remove.mutate(productId)} className="hover:underline">
            Stop tracking
          </button>
        )}
        {t.save.isSuccess && <span className="text-drop">Saved.</span>}
        <Link to="/me" className="ml-auto hover:underline">
          My products and alert settings
        </Link>
      </div>
    </section>
  );
}
