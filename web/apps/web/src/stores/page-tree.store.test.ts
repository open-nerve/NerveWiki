import { reaction } from "mobx";
import { expect, test, vi } from "vitest";

import { ApiError } from "../services/api";
import type { LogFilter, NodeMove, PageView, TreeNode } from "../services/page.service";
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
    getPageContent: async (id: string) => ({ content: `${id}\n`, revision: 1, content_hash: "" }),
    versions: async () => ({ data: [], next_cursor: null }),
    version: async () => {
      throw new Error("no version");
    },
    versionView: async () => {
      throw new Error("no version");
    },
    restoreVersion: (id: string, revision: number, base: number) =>
      answer(`restore ${id} ${revision.toString()} on ${base.toString()}`, () => ({
        revision: base + 1,
        changeset_id: "c1",
        changed: true,
      })),
    trash: async (notebookId: string, cursor?: string) => {
      sent.push(`trash ${notebookId} ${cursor ?? ""}`.trim());
      return { data: [], next_cursor: null };
    },
    trashItem: async (id: string) => {
      sent.push(`trash item ${id}`);
      throw new ApiError(404, { status: 404, code: "page.not_found", title: "gone" });
    },
    restoreNode: (id: string) =>
      answer(`restore ${id}`, () => ({
        id,
        kind: "page" as const,
        name: state.nodes.find((n) => n.id === id)?.name ?? "",
        renamed: false,
        placed: { parent_id: null, reason: "original" as const },
        changeset_id: "c2",
      })),
    changesets: async (notebookId: string, filter: LogFilter, cursor?: string) => {
      sent.push(`changesets ${notebookId} ${JSON.stringify(filter)} ${cursor ?? ""}`.trim());
      return { data: [], next_cursor: null };
    },
    changeset: async (id: string) => {
      sent.push(`changeset ${id}`);
      throw new ApiError(404, { status: 404, code: "page.changeset_not_found", title: "gone" });
    },
    changes: async (id: string, cursor?: string) => {
      sent.push(`changes ${id} ${cursor ?? ""}`.trim());
      return { data: [], next_cursor: null };
    },
    revertChangeset: (id: string, message?: string) =>
      answer(`revert ${id}${message === undefined ? "" : ` ${message}`}`, () => ({
        changeset_id: `revert of ${id}`,
        changed: true,
      })),
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
  state.nodes = [notes];
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

test("a page's restores go out one at a time, in the order made, each once the one before settled either way; another page's beside them", async () => {
  const { pages, sent, state } = store();
  const first = held<unknown>();
  state.writes.set(`restore ${guide.id} 2 on 5`, () => first.promise);

  const one = pages.restoreVersion(guide.id, 2, 5);
  const two = pages.restoreVersion(guide.id, 3, 5);
  const other = pages.restoreVersion(notes.id, 1, 2);
  await vi.waitFor(() => expect(sent).toEqual([`restore ${guide.id} 2 on 5`, `restore ${notes.id} 1 on 2`]));
  expect(await other).toEqual({ revision: 3, changeset_id: "c1", changed: true });

  first.reject(new ApiError(409, { status: 409, code: "page.revision_mismatch", title: "changed" }));
  await expect(one).rejects.toThrow(ApiError);
  expect(await two).toEqual({ revision: 6, changeset_id: "c1", changed: true });
  expect(sent).toEqual([`restore ${guide.id} 2 on 5`, `restore ${notes.id} 1 on 2`, `restore ${guide.id} 3 on 5`]);
});

// M8/P2 design 4.4: a page this generation deleted is restored, here or elsewhere.
test("a page deleted is taken out of those deleted by a read begun after the deletion answered that lists it alive: restored since, it is in the tree again", async () => {
  const { pages, state } = store();
  await pages.load();
  state.nodes = [guide, notes];
  await pages.remove(install.id);
  expect(pages.byId(install.id)).toBeUndefined();

  // Restored elsewhere: the next read lists Install, and Linux under it.
  state.nodes = [guide, install, linux, notes];
  await pages.load();

  expect([install, linux].map((n) => pages.removedTo(n.id))).toEqual([undefined, undefined]);
  expect(pages.byId(linux.id)).toBe(linux);
  expect(pages.childrenOf(guide.id)).toEqual([install]);
});

// M8/P2B review C-M1: the rule's boundary, a read begun after the deletion answered.
test("the deletion's own read, begun as it answered, takes a page it lists alive out: restored in between", async () => {
  const { pages } = store();
  await pages.load();

  // Restored elsewhere between the deletion's answer and its read: the read lists Install, and Linux under it.
  await pages.remove(install.id);

  expect([install, linux].map((n) => pages.removedTo(n.id))).toEqual([undefined, undefined]);
  expect(pages.byId(install.id)).toBe(install);
});

test("a read that lists the page alive begun before the deletion answered takes nothing out: the deletion's own read, and one out meanwhile", async () => {
  const { pages, state } = store();
  await pages.load();
  const early = held<TreeNode[]>();
  state.writes.set("list", () => early.promise);
  const reading = pages.load();
  state.writes.delete("list");
  // The deletion's read fails, so that the early one's tree is what is left to keep.
  state.writes.set(`delete ${notes.id}`, async () => {
    state.writes.set("list", () => Promise.reject(new TypeError("offline")));
  });

  await pages.remove(notes.id);
  early.resolve([guide, install, linux, notes]);
  await reading.catch(() => undefined);

  expect(pages.removedTo(notes.id)).toBeNull();
  expect(pages.byId(notes.id)).toBeUndefined();
});

test("a read begun before the deletion answered and kept after it, answered as the answer is, takes nothing out: it may have listed the page before it went", async () => {
  const { pages, state } = store();
  await pages.load();
  const early = held<TreeNode[]>();
  // One step longer than the deletion's answer: the read is kept after the page is marked, before the write's count.
  state.writes.set("list", () => early.promise.then((nodes) => nodes));
  const reading = pages.load();
  state.writes.delete("list");
  state.writes.set(`delete ${notes.id}`, async () => {
    state.nodes = [guide, install, linux];
    early.resolve([guide, install, linux, notes]);
  });

  await pages.remove(notes.id);
  await reading;

  expect(pages.removedTo(notes.id)).toBeNull();
  expect(pages.byId(notes.id)).toBeUndefined();
});

test("a read the same as the tree kept takes a page restored out of those deleted too: the deletion's own read had failed", async () => {
  const { pages, state } = store();
  await pages.load();
  const before = pages.nodes;
  state.writes.set("list", () => Promise.reject(new TypeError("offline")));
  await pages.remove(notes.id);
  expect(pages.byId(notes.id)).toBeUndefined();

  state.writes.delete("list");
  await pages.load();

  expect(pages.nodes).toBe(before);
  expect(pages.removedTo(notes.id)).toBeUndefined();
  expect(pages.byId(notes.id)).toBe(notes);
});

test("a restore goes out in turn with the tree's writes, and reads the tree again once answered, which takes the page out of those deleted", async () => {
  const { pages, sent, state } = store();
  await pages.load();
  state.nodes = [guide, notes];
  await pages.remove(install.id);
  const creation = held<unknown>();
  state.writes.set("create Untitled", () => creation.promise);

  const creating = pages.create(null, "Untitled");
  const restoring = pages.restore(install.id);
  await Promise.resolve();
  expect(sent.at(-1)).toBe("create Untitled");

  creation.resolve({ ...pageNode(9, "Untitled"), ancestors: [], revision: 1, byte_size: 0 });
  await creating;
  state.nodes = [guide, install, linux, notes];
  expect(await restoring).toMatchObject({ id: install.id, changeset_id: "c2" });

  expect(sent.slice(-4)).toEqual(["create Untitled", "list plans", `restore ${install.id}`, "list plans"]);
  expect(pages.removedTo(install.id)).toBeUndefined();
  expect(pages.byId(linux.id)).toBe(linux);
});

// M8/P2B review C, nit 1.
test("a restore whose tree read fails reads the tree once more: the page this generation deleted is in it again", async () => {
  const { pages, sent, state } = store();
  await pages.load();
  state.nodes = [guide, notes];
  await pages.remove(install.id);
  state.nodes = [guide, install, linux, notes];
  let failing = true;
  state.writes.set("list", async () => {
    if (failing) {
      failing = false;
      throw new TypeError("offline");
    }
    return state.nodes;
  });

  await pages.restore(install.id);

  expect(sent.slice(-3)).toEqual([`restore ${install.id}`, "list plans", "list plans"]);
  expect(pages.removedTo(install.id)).toBeUndefined();
  expect(pages.byId(install.id)).toBe(install);
});

test("a node's restores go out one at a time, the second once the first settled; another's beside them, in turn with the writes; a refusal is the caller's and records nothing", async () => {
  const { pages, sent, state } = store();
  await pages.load();
  state.nodes = [guide, install, linux];
  await pages.remove(notes.id);
  const first = held<unknown>();
  state.writes.set(`restore ${notes.id}`, () => first.promise);
  const tooLarge = new ApiError(422, { status: 422, code: "page.restore_too_large", title: "large" });

  const one = pages.restore(notes.id);
  const two = pages.restore(notes.id);
  const other = pages.restore(guide.id);
  await vi.waitFor(() => expect(sent.at(-1)).toBe(`restore ${notes.id}`));
  first.reject(tooLarge);

  await expect(one).rejects.toBe(tooLarge);
  await other;
  state.writes.set(`restore ${notes.id}`, () => Promise.reject(tooLarge));
  await expect(two).rejects.toBe(tooLarge);
  expect(sent.slice(-6)).toEqual([
    `restore ${notes.id}`,
    "list plans",
    `restore ${guide.id}`,
    "list plans",
    `restore ${notes.id}`,
    "list plans",
  ]);
  expect(pages.removedTo(notes.id)).toBeNull();
});

test("the trash and an item of it are read for the notebook", async () => {
  const { pages, sent } = store();

  expect(await pages.trash()).toEqual({ data: [], next_cursor: null });
  await pages.trash("next");
  await expect(pages.trashItem(notes.id)).rejects.toThrow(ApiError);

  expect(sent).toEqual(["trash plans", "trash plans next", `trash item ${notes.id}`]);
});

test("the history is read for the notebook, as its filter keeps it; a changeset and its changes by its id", async () => {
  const { pages, sent } = store();
  const filter = { actor: "", client: "mcp", page: guide.id };

  expect(await pages.changesets(filter)).toEqual({ data: [], next_cursor: null });
  await pages.changesets(filter, "next");
  await expect(pages.changeset("c1")).rejects.toThrow(ApiError);
  await pages.changes("c1");
  await pages.changes("c1", "next");

  expect(sent).toEqual([
    `changesets plans ${JSON.stringify(filter)}`,
    `changesets plans ${JSON.stringify(filter)} next`,
    "changeset c1",
    "changes c1",
    "changes c1 next",
  ]);
});

test("a changeset's reverts go out one at a time, the second once the first settled, each in turn with the tree's writes and the tree read once answered; another's beside them; a refusal is the caller's", async () => {
  const { pages, sent, state } = store();
  await pages.load();
  const first = held<unknown>();
  state.writes.set("revert c1 Why", () => first.promise);
  const conflict = new ApiError(409, { status: 409, code: "page.revert_conflict", title: "conflict" });

  const one = pages.revert("c1", "Why");
  const two = pages.revert("c1");
  const other = pages.revert("c2");
  await vi.waitFor(() => expect(sent.at(-1)).toBe("revert c1 Why"));
  first.reject(conflict);

  await expect(one).rejects.toBe(conflict);
  expect(await other).toEqual({ changeset_id: "revert of c2", changed: true });
  expect(await two).toEqual({ changeset_id: "revert of c1", changed: true });
  expect(sent.slice(-6)).toEqual(["revert c1 Why", "list plans", "revert c2", "list plans", "revert c1", "list plans"]);
  expect(pages.removedTo(guide.id)).toBeUndefined();
});

test("the pages a revert deleted, those of the tree before it its read after it has not, leave the tree as deleted ones, their shells going to the nearest ancestor still there; a later read that lists one alive takes it out", async () => {
  const { pages, state } = store();
  await pages.load();
  state.writes.set("revert c1", async () => {
    // The revert of Install's creation: Install and Linux go.
    state.nodes = [guide, notes];
    return { changeset_id: "r1", changed: true };
  });

  await pages.revert("c1");

  expect(pages.removedTo(install.id)).toBe(guide.id);
  expect(pages.removedTo(linux.id)).toBe(guide.id);
  expect(pages.removedTo(notes.id)).toBeUndefined();
  expect(pages.byId(install.id)).toBeUndefined();

  // Restored since: a read that lists it alive takes it out.
  state.nodes = [guide, install, notes];
  await pages.load();
  expect(pages.removedTo(install.id)).toBeUndefined();
  expect(pages.byId(install.id)).toBe(install);
  expect(pages.removedTo(linux.id)).toBe(guide.id);
});

test("a page a revert deleted with what was above it goes to the notebook's home; one whose answer changed nothing records nothing", async () => {
  const { pages, state } = store();
  await pages.load();
  state.writes.set("revert c1", async () => {
    state.nodes = [notes];
    return { changeset_id: "r1", changed: true };
  });

  await pages.revert("c1");

  expect(pages.removedTo(guide.id)).toBeNull();
  expect(pages.removedTo(linux.id)).toBeNull();

  const { pages: other, state: otherState } = store();
  await other.load();
  otherState.writes.set("revert c1", async () => {
    otherState.nodes = [guide, notes];
    return { changeset_id: null, changed: false };
  });
  await other.revert("c1");
  expect(other.removedTo(install.id)).toBeUndefined();
});

test("a revert whose tree read fails reads the tree once more: the page this generation deleted it brought back is in it again, and those it deleted leave", async () => {
  const { pages, sent, state } = store();
  await pages.load();
  state.nodes = [guide, install, linux];
  await pages.remove(notes.id);
  let failing = true;
  state.writes.set("list", async () => {
    if (failing) {
      failing = false;
      throw new TypeError("offline");
    }
    return state.nodes;
  });
  state.writes.set("revert c1", async () => {
    // The revert of Notes' deletion and of Linux's creation.
    state.nodes = [guide, install, notes];
    return { changeset_id: "r1", changed: true };
  });

  await pages.revert("c1");

  expect(sent.slice(-3)).toEqual(["revert c1", "list plans", "list plans"]);
  expect(pages.removedTo(notes.id)).toBeUndefined();
  expect(pages.byId(notes.id)).toBe(notes);
  expect(pages.removedTo(linux.id)).toBe(install.id);
});

test("a revert whose tree cannot be read records nothing it deleted: the tree is as it was", async () => {
  const { pages, sent, state } = store();
  await pages.load();
  state.writes.set("list", async () => {
    throw new TypeError("offline");
  });
  state.writes.set("revert c1", async () => ({ changeset_id: "r1", changed: true }));

  await pages.revert("c1");

  // Nothing this generation deleted: no read once more.
  expect(sent.slice(-2)).toEqual(["revert c1", "list plans"]);
  expect(pages.byId(linux.id)).toBe(linux);
  expect(pages.removedTo(linux.id)).toBeUndefined();
});

// The negative controls (M8/P3 design section 6).
test("a revert whose tree read fails records nothing it deleted, though a read out as it was took a page deleted elsewhere: no page of the revert's", async () => {
  const { pages, state } = store();
  await pages.load();
  state.writes.set("revert c1", async () => {
    // Linux deleted elsewhere, read as the revert is out; the revert's own read then fails.
    state.nodes = [guide, install, notes];
    await pages.load();
    state.writes.set("list", () => Promise.reject(new TypeError("offline")));
    return { changeset_id: "r1", changed: true };
  });

  await pages.revert("c1");

  expect(pages.byId(linux.id)).toBeUndefined();
  expect(pages.removedTo(linux.id)).toBeUndefined();
});
