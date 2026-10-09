import { expect, test } from "vitest";

import { titleKey } from "./title-key";

test.each([
  ["Café.PNG", "café.png"],
  ["Straße", "strasse"],
  ["STRASSE", "strasse"],
  ["FUẞ", "fuss"],
  ["GROẞE MAẞE", "grosse masse"],
  ["Σοφίας", "σοφίασ"],
  ["5µm", "5μm"],
  ["ﬁle", "file"],
])("%j compares as %j, as the server folds it", (title, key) => {
  expect(titleKey(title)).toBe(key);
});
