import { expect, test } from "vitest";

import { passwordLength } from "./password-length";

test.each([
  ["", "too_short"],
  ["xq7vbnz", "too_short"],
  ["xq7vbnzk", undefined],
  ["xq7vbnzk".repeat(16), undefined],
  ["xq7vbnzk".repeat(16) + "w", "too_long"],
  // Four emoji are 8 UTF-16 units, as the server counts.
  ["😀🎉🔑🌊", undefined],
  // The server judges the NFKC form: é decomposed is two units, composed one.
  ["é".repeat(4), "too_short"],
  ["ｘｑ７ｖｂｎｚｋ", undefined],
])("%j: %s", (password, want) => {
  expect(passwordLength(password)).toBe(want);
});
