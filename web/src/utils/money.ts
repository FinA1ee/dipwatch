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

const whole = new Map<string, Intl.NumberFormat>();

/** "CN¥1,469.90", and "CN¥14,699" when there are no cents to show, from minor units. */
export function formatMoney(money: Money): string {
  const per = minorPerMajor(money.currency);
  if (money.minor % per !== 0) return formatter(money.currency).format(money.minor / per);
  let f = whole.get(money.currency);
  if (!f) {
    f = new Intl.NumberFormat("en", {
      style: "currency",
      currency: money.currency,
      currencyDisplay: "symbol",
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    });
    whole.set(money.currency, f);
  }
  return f.format(money.minor / per);
}
