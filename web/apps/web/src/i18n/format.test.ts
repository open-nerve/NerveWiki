import { expect, test } from "vitest";

import { formatDate, formatDateTime } from "./format";

const iso = "2026-10-01T15:04:00Z";

test("dates are written in the interface's language", () => {
  expect(formatDate(iso, "en", "UTC")).toBe("Oct 1, 2026");
  expect(formatDate(iso, "zh-CN", "UTC")).toBe("2026年10月1日");
  expect(formatDateTime(iso, "en", "UTC")).toBe("Oct 1, 2026, 3:04 PM");
  expect(formatDateTime(iso, "zh-CN", "UTC")).toBe("2026年10月1日 15:04");
});

test("dates are written in the time zone asked for", () => {
  expect(formatDateTime(iso, "en", "Asia/Shanghai")).toBe("Oct 1, 2026, 11:04 PM");
});
