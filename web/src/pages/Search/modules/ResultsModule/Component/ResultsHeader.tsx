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
    <div className="flex flex-wrap items-baseline gap-x-[18px] gap-y-2">
      <h1 className="font-head text-[34px] font-semibold leading-tight tracking-tight">
        <Link to={`/p/${productId}`} className="hover:underline">
          {productName}
        </Link>
      </h1>
      <p className="text-muted" aria-live="polite">
        {shown === total ? `${total} offers` : `${shown} of ${total} offers`}
        {streaming &&
          (pending.length > 0 ? ` · still searching ${pending.join(", ")}…` : " · searching stores…")}
        {timedOut && " · some stores did not answer in time"}
      </p>
    </div>
  );
}
