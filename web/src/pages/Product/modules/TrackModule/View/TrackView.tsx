import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { RuleEditor } from "@/components/RuleEditor";
import { useSettings } from "@/hooks/useSettings";
import { useTracked } from "@/hooks/useTracked";
import { draftToRules, EMPTY_DRAFT, type RuleDraft, rulesToDraft } from "@/utils/rules";

/** Track this product: rules, note, save or stop (T1, T2). */
export function TrackView() {
  const productId = Number(useParams().productId);
  const t = useTracked();
  const currency = useSettings((s) => s.currency);
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
      <section className="rounded-lg border border-slate-200 p-3 text-sm">
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
    t.save.mutate({ productId, note: note.trim(), rules });
  };
  const error = problem || t.save.error?.message || t.remove.error?.message;

  return (
    <section className="space-y-3 rounded-lg border border-slate-200 p-3">
      <h2 className="text-lg font-semibold">{current ? "You track this product" : "Track this product"}</h2>
      <RuleEditor draft={draft} note={note} currency={currency} onDraft={setDraft} onNote={setNote} />
      {error && <p className="text-sm text-rose-700">{error}</p>}
      <div className="flex flex-wrap items-center gap-3 text-sm">
        <button
          type="button"
          onClick={save}
          disabled={t.save.isPending}
          className="rounded-lg bg-slate-900 px-4 py-2 font-medium text-white disabled:opacity-50"
        >
          {current ? "Save changes" : "Track"}
        </button>
        {current && (
          <button type="button" onClick={() => t.remove.mutate(productId)} className="hover:underline">
            Stop tracking
          </button>
        )}
        {t.save.isSuccess && <span className="text-emerald-700">Saved.</span>}
        <Link to="/me" className="ml-auto hover:underline">
          My products and alert settings
        </Link>
      </div>
    </section>
  );
}
