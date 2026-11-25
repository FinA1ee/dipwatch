import type { Offer } from "@/services/api";
import { formatMoney } from "@/utils/money";

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

/** Store links are untrusted data: only http(s) URLs become links. */
function safeHref(url: string): string | undefined {
  try {
    const u = new URL(url);
    return u.protocol === "https:" || u.protocol === "http:" ? u.href : undefined;
  } catch {
    return undefined;
  }
}

const shown = (o: Offer) => o.converted ?? o.price;

/**
 * Ranked offers (S5). The cheapest is marked in green; the others show how much
 * more they cost, so the list reads as steps up the ladder.
 */
export function OfferTable({ offers }: { offers: Offer[] }) {
  const cheapest = offers[0] ? shown(offers[0]) : null;
  return (
    <ul>
      {offers.map((o, i) => {
        const price = shown(o);
        const gap = cheapest && price.currency === cheapest.currency ? price.minor - cheapest.minor : null;
        const facts = [
          o.source.name,
          sellerLabel[o.seller_type],
          conditionLabel[o.condition],
          o.availability === "in_stock" ? "In stock" : "",
        ].filter(Boolean);
        return (
          <li key={o.id} className="grid grid-cols-[1fr_auto] gap-x-6 gap-y-1 border-b border-rule px-1 py-4">
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
                className={`block font-head text-[22px] font-bold leading-tight tracking-tight ${i === 0 ? "text-drop" : ""}`}
              >
                {formatMoney(price)}
              </strong>
              <span className={`block text-[13px] ${i === 0 ? "font-semibold text-drop" : "text-muted"}`}>
                {i === 0
                  ? "lowest"
                  : gap !== null && gap > 0
                    ? `+${formatMoney({ ...price, minor: gap })}`
                    : ""}
              </span>
              {o.converted && o.converted.currency !== o.price.currency && (
                <span className="block text-[13px] text-muted">{formatMoney(o.price)}</span>
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
            </span>
          </li>
        );
      })}
    </ul>
  );
}
