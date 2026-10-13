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

/** The units formatRelative writes a span in, the largest first, each with the seconds it holds. */
const relativeUnits = [
  ["year", 365 * 86_400],
  ["month", 30 * 86_400],
  ["week", 7 * 86_400],
  ["day", 86_400],
  ["hour", 3_600],
  ["minute", 60],
] as const;

/** relativeFormats are formatRelative's formats by language: a list writes a time for each version. */
const relativeFormats = new Map<string, Intl.RelativeTimeFormat>();

/**
 * formatRelative writes how long before now iso is, in the largest unit
 * the span fills, such as "3 minutes ago" or "yesterday"; under a minute,
 * or after now (a clock behind the server's), it is "now". What it leaves
 * out, formatDateTime writes.
 */
export function formatRelative(iso: string, now: number, locale: Locale): string {
  const seconds = Math.min(Math.round((Date.parse(iso) - now) / 1000), 0);
  let format = relativeFormats.get(locale);
  if (format === undefined) {
    format = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
    relativeFormats.set(locale, format);
  }
  for (const [unit, size] of relativeUnits) {
    if (-seconds >= size) {
      return format.format(Math.trunc(seconds / size), unit);
    }
  }
  return format.format(0, "second");
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
 * otherwise in the whole minutes it holds, such as "24 hours" or "1,530
 * minutes": how long a succeeded export is kept (transfer.export_ttl, at
 * least ten minutes), never said longer than it is.
 */
export function formatDuration(seconds: number, locale: Locale): string {
  const [value, unit] = seconds % 3600 === 0 ? [seconds / 3600, "hour"] : [Math.floor(seconds / 60), "minute"];
  return new Intl.NumberFormat(locale, { style: "unit", unit, unitDisplay: "long" }).format(value);
}
