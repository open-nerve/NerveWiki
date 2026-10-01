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

/** formatBytes writes a size in bytes in the largest unit it fills, by 1024, such as "1.5 MB". */
export function formatBytes(bytes: number, locale: Locale): string {
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < byteUnits.length - 1) {
    value /= 1024;
    unit += 1;
  }
  const number = new Intl.NumberFormat(locale, { maximumFractionDigits: unit === 0 ? 0 : 1 }).format(value);
  return `${number} ${byteUnits[unit]}`;
}
