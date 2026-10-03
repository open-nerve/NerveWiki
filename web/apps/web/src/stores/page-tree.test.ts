import { expect, test } from "vitest";

import { guide, install, linux, notes, pageNode } from "../test/page-server";
import {
  ancestorsOf,
  canHold,
  childrenOf,
  depthOf,
  findPages,
  freeTitle,
  heightOf,
  indexTree,
  maxDepth,
  subtreeOf,
} from "./page-tree";

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

test("a page's depth, subtree and height", () => {
  expect([null, guide.id, install.id, linux.id].map((id) => depthOf(tree, id))).toEqual([0, 1, 2, 3]);
  expect(subtreeOf(tree, guide.id).map((n) => n.name)).toEqual(["Guide", "Install", "Linux"]);
  expect(subtreeOf(tree, "nowhere")).toEqual([]);
  expect([guide.id, install.id, linux.id, notes.id].map((id) => heightOf(tree, id))).toEqual([3, 2, 1, 1]);
});

test("a parent holds a page and its subtree unless it is in it or the subtree would go deeper than ten", () => {
  expect(maxDepth).toBe(10);
  expect(canHold(tree, null, linux.id)).toBe(true);
  expect(canHold(tree, notes.id, guide.id)).toBe(true);
  expect(canHold(tree, guide.id, guide.id)).toBe(false);
  expect(canHold(tree, linux.id, guide.id)).toBe(false);
  const chain = [pageNode(10, "L1")];
  for (let i = 2; i <= 8; i++) {
    chain.push(pageNode(9 + i, `L${i}`, chain[chain.length - 1]));
  }
  const deep = indexTree([...chain, guide, install, linux]);
  const eighth = chain[7]?.id ?? "";
  expect(canHold(deep, eighth, install.id)).toBe(true);
  expect(canHold(deep, eighth, guide.id)).toBe(false);
});

const untitled = (n: number) => (n === 1 ? "Untitled" : `Untitled ${n}`);
const cafe = (n: number) => (n === 1 ? "Café" : `Café ${n}`);

test("a free title is the first that no sibling has, by case and NFC", () => {
  expect(freeTitle([], [], untitled)).toBe("Untitled");
  expect(freeTitle([pageNode(20, "UNTITLED")], [], untitled)).toBe("Untitled 2");
  expect(freeTitle([pageNode(20, "Untitled")], ["untitled 2"], untitled)).toBe("Untitled 3");
  expect(freeTitle([pageNode(21, "Cafe\u0301")], [], cafe)).toBe("Café 2");
});

test("the pages found hold the query by case and NFC, from the top down; a blank query finds them all", () => {
  // Listed parents first, but not from the top down.
  const listed = indexTree([guide, notes, install, linux, { ...pageNode(22, "Cafe\u0301"), parent_id: notes.id }]);
  const names = (query: string) => findPages(listed, query).map((page) => page.name);

  expect(names("  ")).toEqual(["Guide", "Install", "Linux", "Notes", "Cafe\u0301"]);
  expect(names("IN")).toEqual(["Install", "Linux"]);
  expect(names(" no ")).toEqual(["Notes"]);
  expect(names("café")).toEqual(["Cafe\u0301"]);
  expect(names("zzz")).toEqual([]);
});
