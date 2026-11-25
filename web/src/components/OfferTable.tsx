import type { Offer } from "@/services/api";
import { formatMoney } from "@/utils/money";

const conditionLabel: Record<Offer["condition"], string> = {
  new: "New",
  used: "Used",
  refurbished: "Refurbished",
  unknown: "",
};

const availabilityLabel: Record<Offer["availability"], string> = {
  in_stock: "In stock",
  out_of_stock: "Out of stock",
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

/** Ranked offers (S5): converted price first, the store's own price underneath. */
export function OfferTable({ offers }: { offers: Offer[] }) {
  return (
    <ul className="divide-y divide-slate-100 rounded-lg border border-slate-200">
      {offers.map((o) => (
        <li key={o.id} className="flex items-start gap-4 p-3">
          <div className="min-w-0 flex-1">
            <a
              href={safeHref(o.url)}
              target="_blank"
              rel="noopener noreferrer nofollow"
              className="font-medium text-slate-900 hover:underline"
            >
              {o.title}
            </a>
            {o.title !== o.title_original && (
              <p className="truncate text-sm text-slate-500" title={o.title_original}>
                {o.title_original}
              </p>
            )}
            <p className="mt-1 text-sm text-slate-600">
              {o.source.name} · {o.source.region}
              {conditionLabel[o.condition] && ` · ${conditionLabel[o.condition]}`}
              {availabilityLabel[o.availability] && ` · ${availabilityLabel[o.availability]}`}
              {sellerLabel[o.seller_type] && ` · ${sellerLabel[o.seller_type]}`}
            </p>
          </div>
          <div className="text-right">
            <p className="text-lg font-semibold tabular-nums">
              {o.converted ? formatMoney(o.converted) : formatMoney(o.price)}
            </p>
            {o.converted && o.converted.currency !== o.price.currency && (
              <p className="text-sm text-slate-500 tabular-nums">{formatMoney(o.price)}</p>
            )}
          </div>
        </li>
      ))}
    </ul>
  );
}
