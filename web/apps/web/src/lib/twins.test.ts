import { expect, test } from "vitest";

import { shortId, twinsOf } from "./twins";

test("the twins of a list are the items another has the key of, each of them; an item of its own key is none", () => {
  const list = [
    { id: "a", name: "Plans", place: "Guide" },
    { id: "b", name: "Plans", place: "Guide" },
    { id: "c", name: "Plans", place: "Notes" },
    { id: "d", name: "Notes", place: "Guide" },
    { id: "e", name: "Plans", place: "Guide" },
  ];

  expect(twinsOf(list, (item) => `${item.name}\n${item.place}`)).toEqual(new Set(["a", "b", "e"]));
  expect(twinsOf(list, (item) => item.name)).toEqual(new Set(["a", "b", "c", "e"]));
  expect(twinsOf(list, (item) => item.id)).toEqual(new Set());
  expect(twinsOf([], (item: { id: string }) => item.id)).toEqual(new Set());
});

test("a short id is the last six digits of the id, its dashes left out", () => {
  expect(shortId("0199a2b4-0000-7000-8000-0000000e1a2b")).toBe("0e1a2b");
  expect(shortId("0199a2b4-0000-7000-8000-00000000-0e1")).toBe("0000e1");
});
