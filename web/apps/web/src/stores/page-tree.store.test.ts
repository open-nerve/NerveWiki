import { reaction } from "mobx";
import { expect, test, vi } from "vitest";

import { ApiError } from "../services/api";
import type { NodeMove, PageView, TreeNode } from "../services/page.service";
import { assetNode, guide, install, linux, notes, pageNode } from "../test/page-server";
import { PageTreeStore, toggleLimit } from "./page-tree.store";

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
 * page, by default at once. The tag's pages are Guide, and a landing is
 * its target at the root.
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
    getPageView: async (id: string): Promise<PageView> => ({
      html: `<p>${id}</p>`,
      revision: 1,
      assets_expire_at: null,
    }),
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
  const linking = {
    tagPages: async (notebook: string, tag: string) => {
      sent.push(`tag ${notebook} ${tag}`);
      return [guide.id];
    },
    linkLanding: async (id: string, target: string) => {
      sent.push(`landing ${id} ${target}`);
      return { node_id: null, landing: { parent_id: null, title: target }, reason: null };
    },
    linkTargets: async (notebook: string) => {
      sent.push(`link targets ${notebook}`);
      return [];
    },
    tags: async (notebook: string) => {
      sent.push(`tags ${notebook}`);
      return [];
    },
    backlinks: async (id: string, cursor?: string) => {
      sent.push(`backlinks ${id} ${cursor ?? ""}`);
      return { data: [], next_cursor: null };
    },
    properties: async (id: string) => {
      sent.push(`properties ${id}`);
      return { valid: true, properties: [], links: [], assets_expire_at: null };
    },
  };
  return { pages: new PageTreeStore(service, "plans", linking), sent, state };
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

test("a first read out as an upload answers (wrote) is not kept, though it answers first: the tree read after it is (M7/P4 design 3.3)", async () => {
  const { pages, state } = store();
  const stale = held<TreeNode[]>();
  state.writes.set("list", () => stale.promise);
  const reading = pages.load();
  const fresh = held<TreeNode[]>();
  state.writes.set("list", () => fresh.promise);
  const upload = assetNode(20, "a.png", guide);

  const wrote = pages.wrote();
  stale.resolve([guide, install, linux, notes]);
  await new Promise((resolve) => setTimeout(resolve, 0));
  expect(pages.nodes).toBeUndefined();
  fresh.resolve([guide, install, linux, notes, upload]);
  await Promise.all([reading, wrote]);

  expect(pages.siblingsOf(guide.id).map((node) => node.name)).toEqual(["Install", "a.png"]);
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

test("a page created is in the tree once create answers, an upload's answer overlapping the read after it (M7/P4A review A1)", async () => {
  const { pages, state } = store();
  await pages.load();
  const created = { ...pageNode(9, "Untitled"), parent_id: guide.id };
  const before = held<TreeNode[]>();
  state.writes.set("list", () => before.promise);

  const creating = pages.create(guide.id, "Untitled");
  await new Promise((resolve) => setTimeout(resolve, 0));
  // The upload answers as the creation's read is out; the read after it, which has the page, answers last.
  const after = held<TreeNode[]>();
  state.writes.set("list", () => after.promise);
  const wrote = pages.wrote();
  before.resolve([guide, install, linux, notes]);
  await new Promise((resolve) => setTimeout(resolve, 0));
  after.resolve([guide, install, linux, created, notes]);
  const id = await creating;

  expect(id).toBe(created.id);
  expect(pages.byId(created.id)).toBeDefined();
  await wrote;
});

test("uploads answered as a read after one is out have one more read after it, not one each", async () => {
  const { pages, sent, state } = store();
  await pages.load();
  const out = held<TreeNode[]>();
  state.writes.set("list", () => out.promise);
  sent.length = 0;

  const first = pages.wrote();
  const second = pages.wrote();
  const third = pages.wrote();
  state.writes.delete("list");
  state.nodes = [guide, install, linux, notes, assetNode(20, "a.png", guide)];
  out.resolve([guide, install, linux, notes]);
  await Promise.all([first, second, third]);

  expect(sent).toEqual(["list plans", "list plans"]);
  expect(pages.siblingsOf(guide.id).map((node) => node.name)).toEqual(["Install", "a.png"]);
});

test("each upload's answer settles once the first tree read begun after it has: one answered as a read is out waits for the next", async () => {
  const { pages, state } = store();
  await pages.load();
  const one = held<TreeNode[]>();
  state.writes.set("list", () => one.promise);
  const settled: string[] = [];

  const first = pages.wrote().then(() => settled.push("first"));
  const two = held<TreeNode[]>();
  state.writes.set("list", () => two.promise);
  const second = pages.wrote().then(() => settled.push("second"));
  one.resolve([guide, install, linux, notes]);
  await first;
  await new Promise((resolve) => setTimeout(resolve, 0));

  expect(settled).toEqual(["first"]);
  two.resolve([guide, install, linux, notes]);
  await second;
  expect(settled).toEqual(["first", "second"]);
});

test("an upload's answer settles though the tree read after it fails; the next is read all the same", async () => {
  const { pages, sent, state } = store();
  await pages.load();
  state.writes.set("list", () => Promise.reject(new TypeError("offline")));
  sent.length = 0;

  await pages.wrote();
  state.writes.delete("list");
  state.nodes = [guide, install, linux, notes, assetNode(20, "a.png", guide)];
  await pages.wrote();

  expect(sent).toEqual(["list plans", "list plans"]);
  expect(pages.siblingsOf(guide.id).map((node) => node.name)).toEqual(["Install", "a.png"]);
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

test("attachments are nodes but no level of the tree: no page, child or ancestor of it; a new page's siblings have them", async () => {
  const picture = assetNode(30, "untitled.png", install);
  const sheet = assetNode(31, "sheet.csv");
  const { pages } = store([guide, install, picture, linux, notes, sheet]);
  await pages.load();

  expect(pages.nodes).toHaveLength(6);
  expect(pages.byId(picture.id)).toBeUndefined();
  expect(pages.childrenOf(null)).toEqual([guide, notes]);
  expect(pages.childrenOf(install.id)).toEqual([linux]);
  expect(pages.ancestorsOf(linux.id)).toEqual([guide, install]);
  expect(pages.siblingsOf(install.id)).toEqual([picture, linux]);
  expect(pages.siblingsOf(null)).toEqual([guide, notes, sheet]);
});

test("a read that changes attachments alone keeps the tree, rendering nothing again; one that changes a page replaces it", async () => {
  const { pages, state } = store();
  await pages.load();
  let renders = 0;
  const stop = reaction(
    () => pages.tree,
    () => (renders += 1)
  );
  const before = pages.tree;

  state.nodes = [guide, install, linux, notes, assetNode(30, "a.png", guide)];
  await pages.load();
  state.nodes = [guide, install, linux, notes, assetNode(30, "b.png", guide)];
  await pages.load();
  expect(renders).toBe(0);
  expect(pages.tree).toBe(before);
  expect(pages.siblingsOf(guide.id).map((node) => node.name)).toEqual(["Install", "b.png"]);

  state.nodes = [guide, install, linux, { ...notes, name: "Notes 2" }, assetNode(30, "b.png", guide)];
  await pages.load();
  expect(renders).toBe(1);
  expect(pages.byId(notes.id)?.name).toBe("Notes 2");
  stop();
});

test("one toggle of a task item is out per page at a time, the view read after it included; another page's runs beside it", async () => {
  const { pages } = store();
  const ran: string[] = [];
  let finish: (() => void) | undefined;
  const out = new Promise<void>((resolve) => {
    finish = resolve;
  });

  const first = pages.oneToggle(guide.id, async () => {
    ran.push("first");
    await out;
  });
  const meanwhile = await pages.oneToggle(guide.id, async () => {
    ran.push("meanwhile");
  });
  const beside = await pages.oneToggle(install.id, async () => {
    ran.push("beside");
  });
  finish?.();

  expect([await first, meanwhile, beside]).toEqual([true, false, true]);
  const refused = pages.oneToggle(guide.id, async () => {
    ran.push("after");
    throw new Error("refused");
  });
  await expect(refused).rejects.toThrow("refused");
  expect(await pages.oneToggle(guide.id, async () => void ran.push("again"))).toBe(true);
  expect(ran).toEqual(["first", "beside", "after", "again"]);
});

test("a toggle holds the page's others back a minute at most (M5/P6 design 3.5)", () => {
  expect(toggleLimit).toBe(60_000);
});

test("a toggle out longer than toggleLimit holds the page's others back no longer, and its end leaves the next one's hold", async () => {
  vi.useFakeTimers();
  try {
    const { pages } = store();
    let finish: (() => void) | undefined;
    const lost = pages.oneToggle(guide.id, () => new Promise<void>((resolve) => (finish = resolve)));
    expect(await pages.oneToggle(guide.id, async () => undefined)).toBe(false);

    vi.advanceTimersByTime(toggleLimit);
    let next: (() => void) | undefined;
    const later = pages.oneToggle(guide.id, () => new Promise<void>((resolve) => (next = resolve)));
    finish?.();
    expect(await lost).toBe(true);

    expect(await pages.oneToggle(guide.id, async () => undefined)).toBe(false);
    next?.();
    expect(await later).toBe(true);
  } finally {
    vi.useRealTimers();
  }
});

test("the pages of a tag and a link's landing are read for the notebook", async () => {
  const { pages, sent } = store();
  expect(await pages.tagPages("a/b")).toEqual([guide.id]);
  expect(await pages.landing(guide.id, "Plans/x")).toEqual({
    node_id: null,
    landing: { parent_id: null, title: "Plans/x" },
    reason: null,
  });
  expect(sent).toEqual(["tag plans a/b", `landing ${guide.id} Plans/x`]);
});
