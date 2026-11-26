import { PriceChart } from "@/components/PriceChart";
import type { TrackedItem } from "@/services/tracking";
import { formatMoney } from "@/utils/money";

const relative = new Intl.RelativeTimeFormat("en", { numeric: "auto" });

/** "2 hours ago", "in 3 hours": rough is fine for a schedule. */
function fromNow(iso: string): string {
  const minutes = Math.round((new Date(iso).getTime() - Date.now()) / 60000);
  if (Math.abs(minutes) < 60) return relative.format(minutes, "minute");
  return relative.format(Math.round(minutes / 60), "hour");
}

/**
 * The tracked price since tracking started, as a full chart, with the change
 * since the first day and when Dipwatch checks it again.
 */
export function PriceTrend({ item }: { item: TrackedItem }) {
  const history = item.history;
  const first = history[0];
  const last = history.at(-1);
  const change = first && last ? ((last.price.minor - first.price.minor) / first.price.minor) * 100 : 0;
  const summary =
    !first || !last
      ? "No price yet"
      : Math.abs(change) < 0.05
        ? `${formatMoney(last.price)}, unchanged since ${first.day}`
        : `${formatMoney(last.price)}, ${change < 0 ? "down" : "up"} ${Math.abs(change).toFixed(1)}% since ${first.day}`;
  const schedule = [
    item.last_checked && `checked ${fromNow(item.last_checked)}`,
    item.next_check &&
      `next check ${new Date(item.next_check) <= new Date() ? "soon" : fromNow(item.next_check)}`,
    item.check_every_hours ? `every ${item.check_every_hours} h` : "automatic interval",
  ].filter(Boolean);
  return (
    <div className="space-y-2 pt-2">
      <p className="text-sm">
        <span
          className={`font-semibold ${change < -0.05 ? "text-drop" : change > 0.05 ? "text-warn" : "text-ink"}`}
        >
          {summary}
        </span>
        <span className="text-muted"> · {schedule.join(" · ")}</span>
      </p>
      <PriceChart history={history.map((d) => ({ day: d.day, converted: d.price }))} />
    </div>
  );
}
