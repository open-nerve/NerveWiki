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

test("t fills the placeholders it has values for", () => {
  const t = translator("en");

  expect(t("home.version", { version: "1.2.3", commit: "4f2a9c1" })).toBe("Version 1.2.3 (4f2a9c1)");
  expect(t("error.code")).toBe("Error code: {code}");
});
