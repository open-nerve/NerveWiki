import { expect, test } from "vitest";

import { ApiError } from "../services/api";
import type { NodeMove, PageView, TreeNode } from "../services/page.service";
import { guide, install, linux, notes, pageNode } from "../test/page-server";
import { PageTreeStore } from "./page-tree.store";

/** A promise the test settles. */
function held<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

/**
 * store is a tree over a fake service: the tree it lists is nodes, and
 * what went out is in sent; a write answers what writes says for its
 * page, by default at once.
 */
function store(nodes: TreeNode[] = [guide, install, linux, notes]) {
  const sent: string[] = [];
  const state = { nodes, writes: new Map<string, () => Promise<unknown>>() };
  const answer = async <T>(key: string, ok: () => T): Promise<T> => {
    sent.push(key);
    const custom = state.writes.get(key);
    return custom === undefined ? ok() : ((await custom()) as T);
  };
  const service = {
    listNodes: async (notebookId: string) => {
      sent.push(`list ${notebookId}`);
      const custom = state.writes.get("list");
      return custom === undefined ? state.nodes : ((await custom()) as TreeNode[]);
    },
    getPageView: async (id: string): Promise<PageView> => ({ html: `<p>${id}</p>`, revision: 1 }),
    createPage: (notebookId: string, parent: string | null, title: string) =>
      answer(`create ${title}`, () => ({
        ...pageNode(9, title),
        parent_id: parent,
        notebook_id: notebookId,
        ancestors: [],
        revision: 1,
        byte_size: 0,
        content_updated_at: "",
        content_updated_by: "",
      })),
    renameNode: (id: string, name: string) =>
      answer(`rename ${name}`, () => ({ ...(state.nodes.find((n) => n.id === id) ?? guide), name })),
    moveNode: (id: string, move: NodeMove) => answer(`move ${id} ${String(move.parent_id)}`, () => guide),
    deleteNode: (id: string) => answer(`delete ${id}`, () => undefined),
    lock: async () => ({ holder: null, expires_in: null }),
    releaseLock: async () => undefined,
    toggleTask: async () => ({
      ...guide,
      ancestors: [],
      revision: 2,
      byte_size: 0,
      content_updated_at: "",
      content_updated_by: "",
    }),
  };
  return { pages: new PageTreeStore(service, "plans"), sent, state };
}

const titleTaken = new ApiError(409, { status: 409, code: "page.title_taken", title: "taken" });

test("the tree is read for its notebook; before, it has no page", async () => {
  const { pages, sent } = store();
  expect(pages.byId(guide.id)).toBeUndefined();
  expect(pages.childrenOf(null)).toEqual([]);

  await pages.load();

  expect(sent).toEqual(["list plans"]);
  expect(pages.byId(install.id)).toBe(install);
  expect(pages.childrenOf(null)).toEqual([guide, notes]);
  expect(pages.ancestorsOf(linux.id)).toEqual([guide, install]);
});

test("a page is opened and closed; opening to a page opens its ancestors", async () => {
  const { pages } = store();
  await pages.load();

  pages.toggle(notes.id);
  expect(pages.isOpen(notes.id)).toBe(true);
  pages.toggle(notes.id);
  expect(pages.isOpen(notes.id)).toBe(false);

  pages.openTo(linux.id);
  expect([guide, install, linux].map((n) => pages.isOpen(n.id))).toEqual([true, true, false]);
});

test("the writes go out one at a time, whichever page each is of, each answer reading the tree again", async () => {
  const { pages, sent, state } = store();
  await pages.load();
  const first = held<unknown>();
  state.writes.set("rename Handbook", () => first.promise);

  const renamed = pages.rename(guide.id, "Handbook");
  const moved = pages.move(notes.id, { parent_id: guide.id });
  await Promise.resolve();
  expect(sent).toEqual(["list plans", "rename Handbook"]);

  state.nodes = [{ ...guide, name: "Handbook" }, install, linux, notes];
  first.resolve({ ...guide, name: "Handbook" });
  await renamed;
  expect(pages.byId(guide.id)?.name).toBe("Handbook");
  await moved;

  expect(sent).toEqual(["list plans", "rename Handbook", "list plans", `move ${notes.id} ${guide.id}`, "list plans"]);
});

test("a refusal reads the tree again too, and is the caller's", async () => {
  const { pages, sent, state } = store();
  await pages.load();
  state.writes.set("rename Notes", () => Promise.reject(titleTaken));

  await expect(pages.rename(guide.id, "Notes")).rejects.toBe(titleTaken);

  expect(sent).toEqual(["list plans", "rename Notes", "list plans"]);
});

test("a read that a write's answer overlaps keeps the tree read after the write", async () => {
  const { pages, state } = store();
  await pages.load();
  const stale = held<TreeNode[]>();
  state.writes.set("list", () => stale.promise);
  const reading = pages.load();

  state.writes.delete("list");
  state.nodes = [guide, install, linux];
  await pages.remove(notes.id);
  stale.resolve([guide, install, linux, notes]);
  await reading;

  expect(pages.byId(notes.id)).toBeUndefined();
});

test("a read that a rename's answer overlaps keeps the tree read after the rename, the new title", async () => {
  const { pages, state } = store();
  await pages.load();
  const stale = held<TreeNode[]>();
  state.writes.set("list", () => stale.promise);
  const reading = pages.load();

  state.writes.delete("list");
  state.nodes = [{ ...guide, name: "Handbook" }, install, linux, notes];
  await pages.rename(guide.id, "Handbook");
  stale.resolve([guide, install, linux, notes]);
  await reading;

  expect(pages.byId(guide.id)?.name).toBe("Handbook");
});

// Events make reads overlap (M5/P3 design 3.9): another tab's writes come one after another.
test("of reads that overlap, one answered after a later one does not replace the later one's tree", async () => {
  const { pages, state } = store();
  await pages.load();
  const early = held<TreeNode[]>();
  state.writes.set("list", () => early.promise);
  const first = pages.load();
  state.writes.delete("list");
  state.nodes = [guide, install, linux];

  await pages.load();
  early.resolve([guide, install, linux, notes]);
  await first;

  expect(pages.byId(notes.id)).toBeUndefined();
});

test("a first read that a write's answer overlaps, with no tree read since, reads it again", async () => {
  const { pages, sent, state } = store();
  const stale = held<TreeNode[]>();
  state.writes.set("list", () => stale.promise);
  const reading = pages.load();

  // The write's own read fails: the tree is still to be read when the first one comes back.
  state.writes.set("list", () => Promise.reject(new TypeError("offline")));
  await pages.rename(guide.id, "Handbook");
  state.writes.delete("list");
  state.nodes = [{ ...guide, name: "Handbook" }, install, linux, notes];
  stale.resolve([guide, install, linux, notes]);
  await reading;

  expect(pages.byId(guide.id)?.name).toBe("Handbook");
  expect(sent).toEqual(["list plans", "rename Handbook", "list plans", "list plans"]);
});

test("a page created is in the tree once create answers its id", async () => {
  const { pages, state } = store();
  await pages.load();
  const created = { ...pageNode(9, "Untitled"), parent_id: guide.id };
  state.nodes = [guide, install, linux, created, notes];

  const id = await pages.create(guide.id, "Untitled");

  expect(id).toBe(created.id);
  expect(pages.childrenOf(guide.id).map((n) => n.name)).toEqual(["Install", "Untitled"]);
});

test("a page deleted, or deleted already, sends its subtree's shells to its parent", async () => {
  const { pages, state } = store();
  await pages.load();
  state.writes.set(`delete ${install.id}`, () =>
    Promise.reject(new ApiError(404, { status: 404, code: "page.not_found", title: "gone" }))
  );
  state.nodes = [guide, notes];

  await pages.remove(install.id);

  expect([install, linux, guide].map((n) => pages.removedTo(n.id))).toEqual([guide.id, guide.id, undefined]);
  await pages.remove(guide.id);
  expect(pages.removedTo(guide.id)).toBeNull();
});

test("a deletion refused otherwise is the caller's, and marks nothing", async () => {
  const { pages, state } = store();
  await pages.load();
  const forbidden = new ApiError(403, { status: 403, code: "forbidden", title: "no" });
  state.writes.set(`delete ${notes.id}`, () => Promise.reject(forbidden));

  await expect(pages.remove(notes.id)).rejects.toBe(forbidden);
  expect(pages.removedTo(notes.id)).toBeUndefined();
});

test("a deletion queued behind a creation takes the page created too", async () => {
  const { pages, state } = store();
  await pages.load();
  const creation = held<unknown>();
  state.writes.set("create Untitled", () => creation.promise);
  const created = { ...pageNode(9, "Untitled"), parent_id: guide.id };
  state.writes.set(`delete ${guide.id}`, async () => {
    state.nodes = [notes];
  });

  const creating = pages.create(guide.id, "Untitled");
  const removing = pages.remove(guide.id);
  state.nodes = [guide, install, linux, created, notes];
  creation.resolve(created);
  await Promise.all([creating, removing]);

  expect([guide, install, linux, created].map((n) => pages.removedTo(n.id))).toEqual([null, null, null, null]);
});

test("a page deleted leaves the tree at once, though the tree cannot be read again", async () => {
  const { pages, state } = store();
  await pages.load();
  state.writes.set("list", () => Promise.reject(new TypeError("offline")));

  await pages.remove(install.id);

  expect(pages.childrenOf(guide.id)).toEqual([]);
  expect(pages.byId(linux.id)).toBeUndefined();
  expect(pages.removedTo(linux.id)).toBe(guide.id);
});

test("a tree read the same as before is kept as it was; one changed replaces it", async () => {
  const { pages, state } = store();
  await pages.load();
  const before = pages.nodes;

  state.nodes = structuredClone(state.nodes);
  await pages.load();
  expect(pages.nodes).toBe(before);

  state.nodes = [guide, install, linux, { ...notes, name: "Notes 2" }];
  await pages.load();
  expect(pages.nodes).not.toBe(before);
  expect(pages.byId(notes.id)?.name).toBe("Notes 2");
});
