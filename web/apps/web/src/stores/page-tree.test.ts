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
  holdersOf,
  indexTree,
  inTreeOrder,
  maxDepth,
  placeOfTitle,
  subtreeOf,
} from "./page-tree";

const tree = indexTree([guide, install, linux, notes]);

test("a page whose title another has, by case and NFC, is told apart by the pages it is under; a title of its own needs nothing", () => {
  const again = pageNode(5, "notes", linux);
  const twice = indexTree([guide, install, linux, notes, again]);
  expect(placeOfTitle(twice, again.id)).toBe("Guide / Install / Linux");
  expect(placeOfTitle(twice, notes.id)).toBe("");
  expect(placeOfTitle(twice, install.id)).toBeUndefined();
  expect(placeOfTitle(tree, notes.id)).toBeUndefined();
  expect(placeOfTitle(twice, "nowhere")).toBeUndefined();
});

test("the children are each parent's, in the list's order", () => {
  expect(childrenOf(tree, null).map((n) => n.name)).toEqual(["Guide", "Notes"]);
  expect(childrenOf(tree, guide.id).map((n) => n.name)).toEqual(["Install"]);
  expect(childrenOf(tree, linux.id)).toEqual([]);
  expect(tree.byId.get(install.id)).toBe(install);
});

test("pages in the tree's order are each before the pages under it, siblings in their order; one not in the tree is left out", () => {
  // Listed with a sibling after the children of the one before it: the tree's order is not the list's.
  const later = pageNode(5, "Later", guide);
  const listed = indexTree([guide, notes, install, later, linux]);
  const ids = new Set([notes, linux, later, guide].map((page) => page.id));
  expect(inTreeOrder(listed, ids).map((n) => n.name)).toEqual(["Guide", "Linux", "Later", "Notes"]);
  expect(inTreeOrder(listed, new Set(["nowhere", install.id]))).toEqual([install]);
  expect(inTreeOrder(listed, new Set())).toEqual([]);
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

/** spreadLimit is how many arguments a call here takes: the first count, doubling from 2^16, that a spread throws at. */
function spreadLimit(): number {
  for (let n = 1 << 16; ; n *= 2) {
    try {
      Math.max(...Array.from({ length: n }, () => 0));
    } catch (error) {
      if (error instanceof RangeError) {
        return n;
      }
      throw error;
    }
  }
}

test("a page with more children than a call takes arguments has them all in its subtree, and its height: none is spread into a call", () => {
  // Twice what throws: past the limit of this engine, whose stack is the test's (M5-polish 12.12).
  const many = spreadLimit() * 2;
  const children = Array.from({ length: many }, (_, i) => ({ ...linux, id: `c${String(i)}`, parent_id: guide.id }));
  // Built by hand: indexTree normalizes each title, too slow for as many.
  const wide = {
    byId: new Map([guide, ...children].map((node) => [node.id, node])),
    children: new Map([
      [null, [guide]],
      [guide.id, children],
    ]),
    titles: new Map<string, number>(),
  };
  expect(subtreeOf(wide, guide.id)).toHaveLength(many + 1);
  expect(heightOf(wide, guide.id)).toBe(2);
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

test("the parents that hold a page are those canHold tells, in a deep tree and in a flat one: the page's subtree and height worked out once", () => {
  // Deep: a chain of ten, and the page moved under its second, with a child, a grandchild and a leaf under it, three
  // deep: the chain's seventh holds it at ten exactly, its eighth would make eleven.
  const chain = [pageNode(10, "L1")];
  for (let i = 2; i <= 10; i++) {
    chain.push(pageNode(9 + i, `L${i.toString()}`, chain.at(-1)));
  }
  const moved = pageNode(30, "Moved", chain[1]);
  const child = pageNode(31, "Child", moved);
  const deep = indexTree([
    ...chain,
    moved,
    child,
    pageNode(32, "Grandchild", child),
    pageNode(33, "Leaf", moved),
    guide,
    install,
    linux,
    notes,
  ]);
  const parents = [null, ...deep.byId.keys()];
  const holds = holdersOf(deep, moved.id);
  expect(parents.map((id) => holds(id))).toEqual(parents.map((id) => canHold(deep, id, moved.id)));
  expect(parents.filter((id) => holds(id)).map((id) => (id === null ? "root" : deep.byId.get(id)?.name))).toEqual([
    "root",
    "L1",
    "L2",
    "L3",
    "L4",
    "L5",
    "L6",
    "L7",
    "Guide",
    "Install",
    "Linux",
    "Notes",
  ]);

  // Flat: forty pages at the root, the page moved one of them with a child.
  const flat = Array.from({ length: 40 }, (_, i) => pageNode(40 + i, `P${i.toString()}`));
  const top = flat[5] as (typeof flat)[number];
  const under = pageNode(90, "Under", top);
  const wide = indexTree([...flat, under]);
  const all = [null, ...wide.byId.keys()];
  const held = holdersOf(wide, top.id);
  expect(all.map((id) => held(id))).toEqual(all.map((id) => canHold(wide, id, top.id)));
  expect(all.filter((id) => !held(id))).toEqual([top.id, under.id]);
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
