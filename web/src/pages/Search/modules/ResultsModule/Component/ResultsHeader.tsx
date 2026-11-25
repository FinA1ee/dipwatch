import { Link } from "react-router";

type Props = {
  productId: number;
  productName: string;
  shown: number;
  total: number;
  pending: string[];
  streaming: boolean;
  timedOut: boolean;
};

export function ResultsHeader({ productId, productName, shown, total, pending, streaming, timedOut }: Props) {
  return (
    <div className="flex flex-wrap items-baseline justify-between gap-2">
      <h2 className="text-xl font-semibold">
        <Link to={`/p/${productId}`} className="hover:underline">
          {productName}
        </Link>
      </h2>
      <p className="text-sm text-slate-600" aria-live="polite">
        {shown === total ? `${total} offers` : `${shown} of ${total} offers`}
        {streaming &&
          (pending.length > 0 ? ` · still searching ${pending.join(", ")}…` : " · searching stores…")}
        {timedOut && " · some stores did not answer in time"}
      </p>
    </div>
  );
}
