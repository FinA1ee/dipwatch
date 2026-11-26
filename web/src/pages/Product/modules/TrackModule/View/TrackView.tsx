import { useEffect, useState } from "react";
import { Link, useLocation, useParams } from "react-router";
import { RuleEditor } from "@/components/RuleEditor";
import { useAlertSettings } from "@/hooks/useAlertSettings";
import { useSettings } from "@/hooks/useSettings";
import { useStores } from "@/hooks/useStores";
import { useTracked } from "@/hooks/useTracked";
import { draftToRules, EMPTY_DRAFT, type RuleDraft, rulesToDraft } from "@/utils/rules";
import { ANY_VARIANT } from "@/utils/variants";
import { useListing } from "../../OffersModule/Model/useListing";
import { useProductPage } from "../../OffersModule/Model/useProductPage";
import { useVariant } from "../../OffersModule/Model/useVariant";
import { AlertChannel } from "../Component/AlertChannel";

const CHECK_HOURS = [1, 2, 4, 6, 12, 24];

/**
 * Track this product, one size of it, or one listing: rules, note, where alerts
 * go, save or stop (T1, T2, T6). It sits right under the price summary.
 */
export function TrackView() {
  const productId = Number(useParams().productId);
  const { hash } = useLocation();
  const t = useTracked();
  const currency = useSettings((s) => s.currency);
  const stores = useStores();
  const { listing, choose } = useListing();
  const { page } = useProductPage();
  const { variant } = useVariant();
  const size = variant === ANY_VARIANT ? "" : variant; // "" (not stated) tracks any size too
  const current = t.items.find((i) => i.product_id === productId);
  const [draft, setDraft] = useState<RuleDraft>(EMPTY_DRAFT);
  const [note, setNote] = useState("");
  const [problem, setProblem] = useState("");
  const [checkEvery, setCheckEvery] = useState(0); // hours; 0: automatic
  const alerts = useAlertSettings();

  // Load the saved tracker into the form once it arrives.
  useEffect(() => {
    if (current) {
      setDraft(rulesToDraft(current.rules));
      setNote(current.note);
      setCheckEvery(current.check_every_hours);
    }
  }, [current]);

  // A "Track this listing" link from the search page lands here.
  useEffect(() => {
    if (hash === "#track" && page) document.getElementById("track")?.scrollIntoView();
  }, [hash, page]);

  if (!t.signedIn) {
    return (
      <section
        id="track"
        className="flex flex-wrap items-center gap-x-5 gap-y-3 rounded-[14px] bg-ink px-[22px] py-[18px] text-white"
      >
        <p className="min-w-60 flex-1">Get a Feishu message when the price drops below a price you choose.</p>
        <Link
          to={`/signin?next=${encodeURIComponent(`/p/${productId}${listing ? `?listing=${listing}` : ""}`)}`}
          className="rounded-[10px] bg-white px-[18px] py-2.5 font-semibold text-ink hover:bg-paper"
        >
          Sign in to track
        </Link>
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
    t.save.mutate({
      productId,
      note: note.trim(),
      rules,
      stores: stores.selected,
      offerId: listing,
      variant: listing ? "" : size,
      checkEveryHours: checkEvery,
    });
  };
  const error = problem || t.save.error?.message || t.remove.error?.message;
  const channelMessage = alerts.save.error
    ? { ok: false, text: alerts.save.error.message }
    : alerts.test.error
      ? { ok: false, text: alerts.test.error.message }
      : alerts.test.isSuccess
        ? { ok: true, text: "Sent. Check your Feishu group." }
        : null;
  const what = listing ? "this listing" : size ? `the ${size} price` : "the price";

  return (
    <section id="track" className="space-y-3 rounded-[14px] border-2 border-ink bg-white p-5">
      <h2 className="font-head text-[22px] font-semibold tracking-tight">
        {current ? `You track ${what}` : `Track ${what}`}
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
          Follows the cheapest {size ? `${size} ` : ""}listing at {storeNames}
          {!current && " (the store picked on the search page)"}.{" "}
          {size ? "Other sizes are ignored. " : "Pick a size above to compare like with like. "}Pick one
          listing in the list below to follow only that one.
        </p>
      )}
      {pinned?.gone && pinned.offer_id === listing && (
        <p className="text-sm text-warn">
          The store no longer lists it. Pick another listing below, or follow the cheapest listing.
        </p>
      )}
      <label className="flex flex-wrap items-center gap-2 text-sm">
        <span className="font-medium">Check the price</span>
        <select
          value={checkEvery}
          onChange={(e) => setCheckEvery(Number(e.target.value))}
          className="rounded border border-rule bg-white px-2 py-1"
        >
          <option value={0}>automatically (every 2 to 24 hours as it moves)</option>
          {CHECK_HOURS.map((h) => (
            <option key={h} value={h}>
              every {h} {h === 1 ? "hour" : "hours"}
            </option>
          ))}
        </select>
      </label>
      <RuleEditor draft={draft} note={note} currency={currency} onDraft={setDraft} onNote={setNote} />
      {alerts.settings && (
        <AlertChannel
          webhook={alerts.settings.feishu_webhook}
          saving={alerts.save.isPending}
          testing={alerts.test.isPending}
          message={channelMessage}
          onSave={(feishu) => alerts.save.mutate({ feishu, email: alerts.settings?.email_alerts ?? true })}
          onTest={() => alerts.test.mutate()}
        />
      )}
      {error && <p className="text-sm text-rose-700">{error}</p>}
      <div className="flex flex-wrap items-center gap-3 text-sm">
        <button
          type="button"
          onClick={save}
          disabled={t.save.isPending}
          className="rounded-[10px] bg-drop px-5 py-2.5 text-[15px] font-semibold text-white hover:opacity-90 disabled:opacity-50"
        >
          {current ? "Save changes" : `Track ${what}`}
        </button>
        {current && (
          <button type="button" onClick={() => t.remove.mutate(productId)} className="hover:underline">
            Stop tracking
          </button>
        )}
        {t.save.isSuccess && (
          <span className="text-drop">
            Saved.
            {alerts.settings?.feishu_webhook
              ? " Alerts will go to your Feishu group."
              : " Add a webhook above to get alerts."}
          </span>
        )}
        <Link to="/me" className="ml-auto hover:underline">
          My products and alert settings
        </Link>
      </div>
    </section>
  );
}
