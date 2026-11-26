import { useState } from "react";
import type { Offer } from "@/services/api";
import { formatMoney } from "@/utils/money";
import { trusted } from "@/utils/variants";

type Props = {
  offers: Offer[]; // cheapest first; unconfirmed and unusual prices are left off
  note?: string; // e.g. how the lowest compares with last week
};

/**
 * Every offer as a tick on one price line, cheapest on the left, so the spread
 * and where the cheapest sits are visible at a glance. Hover or focus a tick to
 * read its offer; the list below has every value too.
 */
export function PriceLadder({ offers, note }: Props) {
  const [active, setActive] = useState<number | null>(null);
  const priced = offers
    .filter((o) => o.converted && trusted(o))
    .map((o) => ({ o, m: o.converted as NonNullable<Offer["converted"]> }));
  const first = priced[0];
  const last = priced.at(-1);
  if (!first || !last) return null;
  const lo = first.m.minor;
  const span = last.m.minor - lo || 1;
  const pos = (minor: number) => (priced.length === 1 ? 0 : ((minor - lo) / span) * 100);
  const focus = active !== null ? priced[active] : null;

  return (
    <section aria-label="Price spread" className="rounded-[14px] bg-white px-6 pt-5 pb-4">
      <div className="flex flex-wrap items-baseline justify-between gap-4">
        <p className="font-head text-[44px] font-bold leading-none tracking-tight tabular-nums">
          {formatMoney(first.m)}
          <span className="ml-2.5 font-sans text-[15px] font-medium tracking-normal text-muted">
            lowest now
          </span>
        </p>
        {note && (
          <span className="whitespace-nowrap rounded-full bg-drop-soft px-3 py-1 text-sm font-semibold text-drop">
            {note}
          </span>
        )}
      </div>
      <div className="relative mt-5 h-16" onPointerLeave={() => setActive(null)}>
        <div className="absolute inset-x-0 top-[30px] h-0.5 bg-rule" />
        {priced.map(({ o, m }, i) => (
          <button
            key={o.id}
            type="button"
            aria-label={`${formatMoney(m)}, ${o.title}, ${o.source.name}`}
            onPointerEnter={() => setActive(i)}
            onFocus={() => setActive(i)}
            onBlur={() => setActive(null)}
            className="absolute top-2 h-11 w-4 -translate-x-1/2 cursor-default"
            style={{ left: `${pos(m.minor)}%` }}
          >
            <span
              className={`absolute left-1/2 -translate-x-1/2 rounded-sm ${
                i === 0 ? "top-1.5 h-[34px] w-1 bg-drop" : "top-2.5 h-[26px] w-0.5 bg-muted"
              } ${active === i ? "ring-2 ring-ink ring-offset-2" : ""}`}
            />
          </button>
        ))}
        <span className="absolute top-12 left-0 text-xs font-semibold text-drop">
          {focus && active !== 0 ? "" : `${formatMoney(first.m)} ${first.o.source.name}`}
        </span>
        {priced.length > 1 && (
          <span className="absolute top-12 right-0 text-xs text-muted">{formatMoney(last.m)}</span>
        )}
        {focus && active !== 0 && (
          <span
            className="absolute top-12 max-w-[60%] -translate-x-1/2 truncate text-xs font-medium text-ink"
            style={{ left: `${Math.min(Math.max(pos(focus.m.minor), 15), 85)}%` }}
          >
            {formatMoney(focus.m)} · {focus.o.title}
          </span>
        )}
      </div>
    </section>
  );
}
