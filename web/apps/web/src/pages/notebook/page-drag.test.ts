import { expect, test } from "vitest";

import { indexTree } from "../../stores/page-tree";
import { guide, install, linux, notes, pageNode } from "../../test/page-server";
import { dropMove, dropOperations, type DropOperation } from "./page-drag";

// Guide > Install > Linux, and Notes.
const tree = indexTree([guide, install, linux, notes]);

test.each<[string, string, string, DropOperation, unknown]>([
  ["Notes before Guide: first at the root", notes.id, guide.id, "reorder-before", { parent_id: null, after_id: null }],
  ["Guide after Notes", guide.id, notes.id, "reorder-after", { parent_id: null, after_id: notes.id }],
  [
    "Guide before Notes: after itself is not after Guide",
    guide.id,
    notes.id,
    "reorder-before",
    { parent_id: null, after_id: null },
  ],
  ["Notes into Install: its last child", notes.id, install.id, "combine", { parent_id: install.id }],
  ["Linux before Guide: out to the root", linux.id, guide.id, "reorder-before", { parent_id: null, after_id: null }],
  ["Linux after Notes", linux.id, notes.id, "reorder-after", { parent_id: null, after_id: notes.id }],
  [
    "Notes after Linux: Linux's sibling",
    notes.id,
    linux.id,
    "reorder-after",
    { parent_id: install.id, after_id: linux.id },
  ],
  ["Guide into Linux: into its own subtree", guide.id, linux.id, "combine", undefined],
  ["Guide before Install: into its own subtree", guide.id, install.id, "reorder-before", undefined],
  ["Guide onto itself", guide.id, guide.id, "combine", undefined],
  ["Notes before itself", notes.id, notes.id, "reorder-before", undefined],
  ["Install after itself", install.id, install.id, "reorder-after", undefined],
])("%s", (_name, dragged, target, operation, want) => {
  expect(dropMove(tree, dragged, target, operation)).toEqual(want);
});

test("a drop deeper than ten levels is forbidden, as deep as ten is not", () => {
  const levels = [pageNode(10, "L1")];
  for (let i = 2; i <= 9; i++) {
    levels.push(pageNode(9 + i, `L${i}`, levels[levels.length - 1]));
  }
  const two = pageNode(30, "Two");
  const below = pageNode(31, "Below", two);
  const deep = indexTree([...levels, two, below]);
  const ninth = levels[8]?.id ?? "";

  expect(dropMove(deep, below.id, ninth, "combine")).toEqual({ parent_id: ninth });
  expect(dropMove(deep, two.id, ninth, "combine")).toBeUndefined();
  expect(dropOperations(deep, two.id, ninth, false)).toEqual({
    "reorder-before": "available",
    "reorder-after": "available",
    combine: "blocked",
  });
});

test("over the page itself nothing is offered; into its subtree is blocked", () => {
  expect(dropOperations(tree, guide.id, guide.id, false)).toEqual({
    "reorder-before": "not-available",
    "reorder-after": "not-available",
    combine: "not-available",
  });
  expect(dropOperations(tree, guide.id, linux.id, false)).toEqual({
    "reorder-before": "blocked",
    "reorder-after": "blocked",
    combine: "blocked",
  });
});

test("after a page that shows its children is not offered: into it is, and before it", () => {
  expect(dropOperations(tree, notes.id, guide.id, true)).toEqual({
    "reorder-before": "available",
    "reorder-after": "not-available",
    combine: "available",
  });
  expect(dropOperations(tree, notes.id, guide.id, false)["reorder-after"]).toBe("available");
});
