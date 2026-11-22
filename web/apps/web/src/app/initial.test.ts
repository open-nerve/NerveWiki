import { expect, test } from "vitest";

import { initialOf } from "./initial";

// The avatars' letter (M13/P2/S1, decision 770): the first grapheme of the name, upper-cased. An accent after a
// letter stays the combining one it was: toLocaleUpperCase composes nothing (review C, P2C1-N3). The joiner and the
// accent escaped: an editor normalizing to NFC would compose e and its accent, unseen (review C, S1C-N7).
test.each([
  ["ada", "A"],
  ["  bob", "B"],
  ["产品研发", "产"],
  ["\u{1F469}\u200D\u{1F4BB} Dev", "\u{1F469}\u200D\u{1F4BB}"],
  ["e\u0301", "E\u0301"],
  ["", ""],
])("the initial of %j is %j", (name, initial) => {
  expect(initialOf(name)).toBe(initial);
});
