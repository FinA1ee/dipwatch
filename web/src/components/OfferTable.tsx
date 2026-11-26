import type { ReactNode } from "react";
import type { Offer } from "@/services/api";
import { formatMoney } from "@/utils/money";
import { safeHref } from "@/utils/url";
import { trusted } from "@/utils/variants";

const conditionLabel: Record<Offer["condition"], string> = {
  new: "New",
  used: "Used",
  refurbished: "Refurbished",
  unknown: "",
};

const sellerLabel: Record<Offer["seller_type"], string> = {
  official: "Official store",
  third_party: "Marketplace seller",
  individual: "Individual seller",
  unknown: "",
};

const shown = (o: Offer) => o.converted ?? o.price;

type Props = {
  offers: Offer[];
  /** A control per row, e.g. "Track this listing". */
  action?: (offer: Offer) => ReactNode;
  /** The offer id to mark as the one being tracked. */
  selected?: number | null;
};

/** Why a price is not compared, shown under it. */
function flagNote(o: Offer): string {
  if (o.price_unconfirmed) return "Price not confirmed at the store";
  if (o.unusual_price) return `Unusual price for ${o.variant || "this model"}`;
  return "";
}

/**
 * Ranked offers (S5). The cheapest comparable one is marked in green; the
 * others show how much more they cost, so the list reads as steps up the
 * ladder. Unconfirmed and unusual prices come last, with a note instead.
 */
export function OfferTable({ offers, action, selected }: Props) {
  const lowest = offers.find(trusted);
  const cheapest = lowest ? shown(lowest) : null;
  return (
    <ul>
      {offers.map((o) => {
        const price = shown(o);
        const isLowest = o === lowest;
        const note = flagNote(o);
        const gap = cheapest && price.currency === cheapest.currency ? price.minor - cheapest.minor : null;
        const facts = [
          o.source.name,
          o.variant,
          sellerLabel[o.seller_type],
          conditionLabel[o.condition],
          o.availability === "in_stock" ? "In stock" : "",
        ].filter(Boolean);
        return (
          <li
            key={o.id}
            aria-current={o.id === selected ? "true" : undefined}
            className={`grid grid-cols-[1fr_auto] gap-x-6 gap-y-1 border-b border-rule px-1 py-4 ${
              o.id === selected ? "-mx-3 rounded-[10px] bg-drop-soft px-4" : ""
            }`}
          >
            <a
              href={safeHref(o.url)}
              target="_blank"
              rel="noopener noreferrer nofollow"
              className="font-semibold leading-snug hover:underline"
            >
              {o.title}
            </a>
            <span className="row-span-3 text-right tabular-nums">
              <strong
                className={`block font-head text-[22px] font-bold leading-tight tracking-tight ${isLowest ? "text-drop" : note ? "text-muted" : ""}`}
              >
                {formatMoney(price)}
              </strong>
              <span
                className={`block text-[13px] ${isLowest ? "font-semibold text-drop" : note ? "font-medium text-warn" : "text-muted"}`}
              >
                {isLowest
                  ? "lowest"
                  : note
                    ? note
                    : gap !== null && gap > 0
                      ? `${formatMoney({ ...price, minor: gap })} more than the lowest`
                      : ""}
              </span>
              {o.converted && o.converted.currency !== o.price.currency && (
                <span className="block text-[13px] text-muted">{formatMoney(o.price)} at the store</span>
              )}
              {o.id === selected ? (
                <span className="mt-2 inline-block rounded-full bg-drop px-2.5 py-0.5 text-[13px] font-semibold text-white">
                  ✓ Tracking
                </span>
              ) : (
                action && <span className="mt-2 inline-block">{action(o)}</span>
              )}
            </span>
            {o.title !== o.title_original && (
              <span className="truncate text-[13px] text-muted" title={o.title_original}>
                {o.title_original}
              </span>
            )}
            <span className="flex flex-wrap gap-x-3.5 text-[13px] text-muted">
              {facts.map((f) => (
                <span key={f}>{f}</span>
              ))}
              {o.availability === "out_of_stock" && <span className="text-warn">Out of stock</span>}
              {safeHref(o.url) && (
                <a
                  href={safeHref(o.url)}
                  target="_blank"
                  rel="noopener noreferrer nofollow"
                  className="font-medium text-ink hover:underline"
                >
                  Open at {o.source.name}
                </a>
              )}
            </span>
          </li>
        );
      })}
    </ul>
  );
}
