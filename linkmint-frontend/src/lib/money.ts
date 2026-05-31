/**
 * Format an integer minor-unit amount for display, e.g. `(1000, 'KES')` → `'KES 10.00'`.
 *
 * Presentational only — the protocol always works in integer minor units. This assumes a 2-decimal
 * currency, which is fine for the demo currencies (KES/USD).
 */
export function formatMinorUnits(amount: number, currency: string): string {
  const major = amount / 100;
  const formatted = major.toLocaleString(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
  return `${currency} ${formatted}`;
}
