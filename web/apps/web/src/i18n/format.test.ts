import { expect, test } from "vitest";

import { formatBytes, formatDate, formatDateTime, formatDuration } from "./format";

const iso = "2026-10-01T15:04:00Z";

test("dates are written in the interface's language", () => {
  expect(formatDate(iso, "en", "UTC")).toBe("Oct 1, 2026");
  expect(formatDate(iso, "zh-CN", "UTC")).toBe("2026年10月1日");
  // ICU puts a space or a narrow no-break space before PM, by its version.
  expect(formatDateTime(iso, "en", "UTC")).toMatch(/^Oct 1, 2026, 3:04\sPM$/);
  expect(formatDateTime(iso, "zh-CN", "UTC")).toBe("2026年10月1日 15:04");
});

test("dates are written in the time zone asked for", () => {
  expect(formatDateTime(iso, "en", "Asia/Shanghai")).toMatch(/^Oct 1, 2026, 11:04\sPM$/);
});

test.each([
  [0, "en", "0 B"],
  [1023, "en", "1,023 B"],
  [1024, "en", "1 KB"],
  [1536, "en", "1.5 KB"],
  [5 * 1024 ** 3, "en", "5 GB"],
  [2 * 1024 ** 5, "en", "2,048 TB"],
  [1536, "zh-CN", "1.5 KB"],
] as const)("%d bytes in %s: %s", (bytes, locale, want) => {
  expect(formatBytes(bytes, locale)).toBe(want);
});

test.each([
  [86_400, "en", "24 hours"],
  [3_600, "en", "1 hour"],
  [600, "en", "10 minutes"],
  [5_400, "en", "90 minutes"],
  [86_400, "zh-CN", "24小时"],
  [600, "zh-CN", "10分钟"],
] as const)("%d seconds in %s: %s", (seconds, locale, want) => {
  expect(formatDuration(seconds, locale)).toBe(want);
});
