import type { TrackedItem } from "@/services/tracking";
import { formatMoney } from "@/utils/money";

const W = 160;
const H = 40;

/**
 * The tracked price over the last 30 days, as a small line with the latest
 * price and the change since the first day. One series, so the label names it;
 * the product page has the full chart.
 */
export function PriceTrend({ history }: { history: TrackedItem["history"] }) {
  const first = history[0];
  const last = history.at(-1);
  if (!first || !last) return <p className="text-[13px] text-muted">No price history yet</p>;
  if (history.length === 1) {
    return (
      <p className="text-[13px] text-muted">
        {formatMoney(last.price)} on {last.day}, one day so far
      </p>
    );
  }
  const values = history.map((d) => d.price.minor);
  const lo = Math.min(...values);
  const span = Math.max(...values) - lo || 1;
  const x = (i: number) => 2 + (i / (history.length - 1)) * (W - 4);
  const y = (v: number) => 4 + (1 - (v - lo) / span) * (H - 8);
  const path = values.map((v, i) => `${i ? "L" : "M"}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ");
  const change = ((last.price.minor - first.price.minor) / first.price.minor) * 100;
  const tone = change < -0.05 ? "text-drop" : change > 0.05 ? "text-warn" : "text-muted";
  const summary =
    Math.abs(change) < 0.05
      ? "no change"
      : `${change < 0 ? "down" : "up"} ${Math.abs(change).toFixed(1)}% since ${first.day}`;
  return (
    <figure className={`flex items-center gap-3 ${tone}`}>
      <svg
        width={W}
        height={H}
        viewBox={`0 0 ${W} ${H}`}
        role="img"
        aria-label={`Tracked price: ${formatMoney(last.price)}, ${summary}`}
      >
        <path d={path} fill="none" stroke="currentColor" strokeWidth={2} strokeLinejoin="round" />
        <circle cx={x(values.length - 1)} cy={y(last.price.minor)} r={3} fill="currentColor" />
      </svg>
      <figcaption className="text-[13px]">
        <span className="font-semibold text-ink">{formatMoney(last.price)}</span> {summary}
      </figcaption>
    </figure>
  );
}
