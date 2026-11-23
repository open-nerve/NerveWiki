import { expect, test } from "vitest";

import { onMac } from "./platform";

test("macOS is told by the platform", () => {
  expect(onMac("MacIntel")).toBe(true);
  expect(onMac("Win32")).toBe(false);
  expect(onMac("Linux x86_64")).toBe(false);
});
