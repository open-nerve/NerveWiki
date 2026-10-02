import { expect, test } from "vitest";

import { guide, install, linux, notes, pageNode } from "../test/page-server";
import { ancestorsOf, childrenOf, indexTree } from "./page-tree";

const tree = indexTree([guide, install, linux, notes]);

test("the children are each parent's, in the list's order", () => {
  expect(childrenOf(tree, null).map((n) => n.name)).toEqual(["Guide", "Notes"]);
  expect(childrenOf(tree, guide.id).map((n) => n.name)).toEqual(["Install"]);
  expect(childrenOf(tree, linux.id)).toEqual([]);
  expect(tree.byId.get(install.id)).toBe(install);
});

test("the ancestors go from the root down to the parent", () => {
  expect(ancestorsOf(tree, linux.id).map((n) => n.name)).toEqual(["Guide", "Install"]);
  expect(ancestorsOf(tree, guide.id)).toEqual([]);
  expect(ancestorsOf(tree, "nowhere")).toEqual([]);
});

test("a parent missing from the list ends the ancestors; a cycle does not loop", () => {
  const orphan = { ...pageNode(9, "Orphan"), parent_id: "gone" };
  expect(ancestorsOf(indexTree([orphan]), orphan.id)).toEqual([]);
  const a = { ...pageNode(7, "A"), parent_id: pageNode(8, "B").id };
  const b = { ...pageNode(8, "B"), parent_id: a.id };
  expect(ancestorsOf(indexTree([a, b]), a.id).length).toBeLessThanOrEqual(2);
});
