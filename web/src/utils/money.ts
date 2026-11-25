import type { Money } from "@/services/api";

const formatters = new Map<string, Intl.NumberFormat>();

function formatter(currency: string): Intl.NumberFormat {
  let f = formatters.get(currency);
  if (!f) {
    f = new Intl.NumberFormat("en", { style: "currency", currency, currencyDisplay: "symbol" });
    formatters.set(currency, f);
  }
  return f;
}

/** Minor units per major unit for a currency: 100 for USD, 1 for JPY. */
export function minorPerMajor(currency: string): number {
  return 10 ** (formatter(currency).resolvedOptions().maximumFractionDigits ?? 2);
}

/** "$1,469.00", "¥14,699", from minor units. */
export function formatMoney(money: Money): string {
  return formatter(money.currency).format(money.minor / minorPerMajor(money.currency));
}
