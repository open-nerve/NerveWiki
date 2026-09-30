import { expect, test } from "vitest";

import { translator } from "./i18n";
import { en, type MessageKey } from "./messages/en";
import { zhCN } from "./messages/zh-CN";

// The type Messages already makes every language have exactly en's keys.
const keys = Object.keys(en) as MessageKey[];

function placeholders(text: string): string[] {
  return [...text.matchAll(/\{(\w+)\}/g)].map((m) => m[1] ?? "").toSorted();
}

test.each(keys)("%s has the same placeholders in every language, and text", (key) => {
  expect(placeholders(zhCN[key])).toEqual(placeholders(en[key]));
  expect(en[key].trim()).not.toBe("");
  expect(zhCN[key].trim()).not.toBe("");
});

test("t fills each placeholder", () => {
  const t = translator("en");

  expect(t("home.version", { version: "1.2.3", commit: "4f2a9c1" })).toBe("Version 1.2.3 (4f2a9c1)");
  expect(t("home.loading")).toBe("Loading…");
});

// Checked by the type checker; the calls themselves only need not throw.
test("t takes exactly the placeholders of its key", () => {
  const t = translator("en");

  // @ts-expect-error: home.version needs version and commit
  t("home.version");
  // @ts-expect-error: commit is missing
  t("home.version", { version: "1.2.3" });
  // @ts-expect-error: home.loading has no placeholder
  t("home.loading", { version: "1.2.3" });
});
