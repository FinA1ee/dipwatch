import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { RuleEditor } from "@/components/RuleEditor";
import { useSettings } from "@/hooks/useSettings";
import { useStores } from "@/hooks/useStores";
import { useTracked } from "@/hooks/useTracked";
import { draftToRules, EMPTY_DRAFT, type RuleDraft, rulesToDraft } from "@/utils/rules";

/** Track this product: rules, note, save or stop (T1, T2). */
export function TrackView() {
  const productId = Number(useParams().productId);
  const t = useTracked();
  const currency = useSettings((s) => s.currency);
  const stores = useStores();
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

  if (!t.signedIn) {
    return (
      <section className="rounded-[14px] bg-white p-4 text-sm">
        <Link
          to={`/signin?next=${encodeURIComponent(`/p/${productId}`)}`}
          className="font-medium hover:underline"
        >
          Sign in
        </Link>{" "}
        to track this product and get an alert when the price drops.
      </section>
    );
  }

  const save = () => {
    const rules = draftToRules(draft, currency);
    if (typeof rules === "string") return setProblem(rules);
    setProblem("");
    t.save.mutate({ productId, note: note.trim(), rules, stores: stores.selected });
  };
  const error = problem || t.save.error?.message || t.remove.error?.message;

  return (
    <section className="space-y-3 rounded-[14px] bg-white p-4">
      <h2 className="text-lg font-semibold">{current ? "You track this product" : "Track this product"}</h2>
      <p className="text-sm text-muted">
        At{" "}
        {(current && current.stores.length > 0 ? current.stores : stores.selected)
          .map((id) => stores.all.find((s) => s.id === id)?.name ?? id)
          .join(", ") || "every store"}
        {!current && " (the store picked on the search page)"}. Dipwatch re-checks it every few hours.
      </p>
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
