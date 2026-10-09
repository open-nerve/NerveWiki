import type { Locale } from "./locale";

// Dates as the interface's language writes them, in the browser's time zone
// unless timeZone says otherwise (M1/P6 design 3.6).

/** formatDate writes the day of iso, such as "Oct 1, 2026". */
export function formatDate(iso: string, locale: Locale, timeZone?: string): string {
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeZone }).format(new Date(iso));
}

/** formatDateTime writes the day and the time of iso, such as "Oct 1, 2026, 3:04 PM". */
export function formatDateTime(iso: string, locale: Locale, timeZone?: string): string {
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short", timeZone }).format(new Date(iso));
}

const byteUnits = ["B", "KB", "MB", "GB", "TB"] as const;

/** byteFormats are the numbers' formats of formatBytes by language and digits: a view writes a size for each link. */
const byteFormats = new Map<string, Intl.NumberFormat>();

/** formatBytes writes a size in bytes in the largest unit it fills, by 1024, such as "1.5 MB". */
export function formatBytes(bytes: number, locale: Locale): string {
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < byteUnits.length - 1) {
    value /= 1024;
    unit += 1;
  }
  const digits = unit === 0 ? 0 : 1;
  const key = `${locale} ${digits.toString()}`;
  let format = byteFormats.get(key);
  if (format === undefined) {
    format = new Intl.NumberFormat(locale, { maximumFractionDigits: digits });
    byteFormats.set(key, format);
  }
  return `${format.format(value)} ${byteUnits[unit]}`;
}

/**
 * formatDuration writes a span of seconds in whole hours when it is some,
 * otherwise in whole minutes, such as "24 hours": how long a succeeded
 * export is kept (transfer.export_ttl, at least ten minutes).
 */
export function formatDuration(seconds: number, locale: Locale): string {
  const [value, unit] = seconds % 3600 === 0 ? [seconds / 3600, "hour"] : [Math.round(seconds / 60), "minute"];
  return new Intl.NumberFormat(locale, { style: "unit", unit, unitDisplay: "long" }).format(value);
}
