import { unstable_serialize, type Cache } from "swr";
import { expect, onTestFinished, test, vi } from "vitest";

import { eventHandlers, TREE_INTERVAL_MS } from "./handlers";
import type { Refresher } from "./refresher";

// The handlers' reads through the refresher (M5 design 4.11, M6/P7 design 11).

/**
 * context is a handler's, the keys of read cached (their first read out: no data yet), and what it asks the refresher
 * for, with the intervals it names.
 */
function context(read: unknown[][]) {
  const cached = new Set(read.map((key) => unstable_serialize(key)));
  const requested: string[] = [];
  const intervals = new Map<string, number | undefined>();
  return {
    requested,
    intervals,
    context: {
      cache: {
        get: (key: string) => (cached.has(key) ? { isLoading: true, isValidating: true } : undefined),
      } as unknown as Cache,
      mutate: vi.fn(),
      refresher: {
        request: (key: string, _read: () => void, interval?: number) => {
          requested.push(key);
          intervals.set(key, interval);
        },
      } as unknown as Refresher,
      stopped: new AbortController().signal,
      trees: new Map(),
    },
  };
}

test("a links event asks the refresher for what of the pages it names was read, its first read out too, a view, backlinks, properties: not for a page never read, which it would keep", () => {
  const { requested, context: handled } = context([
    ["page-view", "n1", "a"],
    ["backlinks", "n1", "a"],
    ["page-properties", "n1", "a"],
  ]);
  eventHandlers.get("links")?.(
    { workspace_id: "w1", notebook_id: "n1", pages: ["a", "b"], targets: ["a", "b"] },
    handled
  );

  expect(requested.toSorted()).toEqual(
    [
      ["backlinks", "n1", "a"],
      ["page-properties", "n1", "a"],
      ["page-view", "n1", "a"],
    ].map((key) => unstable_serialize(key))
  );
});

test("a pages event that changed the tree asks the refresher for the tree, at most once in half a second (M7/P2 design 3.10, review C1); one that did not, not", () => {
  expect(TREE_INTERVAL_MS).toBe(500);
  const { requested, intervals, context: handled } = context([]);
  const event = { workspace_id: "w1", notebook_id: "n1", pages: [] };
  eventHandlers.get("pages")?.({ ...event, tree: false }, handled);
  expect(requested.filter((key) => key === unstable_serialize(["pages", "n1"]))).toEqual([]);

  eventHandlers.get("pages")?.({ ...event, tree: true }, handled);
  expect(requested.filter((key) => key === unstable_serialize(["pages", "n1"]))).toEqual([
    unstable_serialize(["pages", "n1"]),
  ]);
  expect(intervals.get(unstable_serialize(["pages", "n1"]))).toBe(TREE_INTERVAL_MS);
  expect(handled.mutate).not.toHaveBeenCalled();
});

test("a pages event asks the refresher for the properties of the pages written that were read", () => {
  const { requested, context: handled } = context([["page-properties", "n1", "a"]]);
  eventHandlers.get("pages")?.(
    {
      workspace_id: "w1",
      notebook_id: "n1",
      tree: false,
      pages: [
        { id: "a", revision: 2 },
        { id: "b", revision: 3 },
      ],
    },
    handled
  );

  expect(requested.filter((key) => key.includes("page-properties"))).toEqual([
    unstable_serialize(["page-properties", "n1", "a"]),
  ]);
});

test("a pages event that changed the tree reads, the tree once read and shown, the notebook's attachments' lists again (M7/P4 design 3.3)", async () => {
  const reads = new Map<string, () => void>();
  const mutate = vi.fn(async (_key: unknown): Promise<unknown[]> => []);
  const handled = {
    ...context([]).context,
    mutate,
    refresher: {
      request: (key: string, read: () => void) => reads.set(key, read),
    } as unknown as Refresher,
  };

  vi.useFakeTimers();
  onTestFinished(() => void vi.useRealTimers());
  eventHandlers.get("pages")?.({ workspace_id: "w1", notebook_id: "n1", tree: true, pages: [] }, handled);
  reads.get(unstable_serialize(["pages", "n1"]))?.();
  expect(mutate.mock.calls).toEqual([[["pages", "n1"]]]);
  // The tree read, React shows it first: a page gone unmounts with its list, which is not read.
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
  expect(mutate).toHaveBeenCalledTimes(1);
  await vi.advanceTimersByTimeAsync(0);
  expect(mutate).toHaveBeenCalledTimes(2);

  const matches = (mutate.mock.calls[1] as unknown[])[0] as (key: unknown) => boolean;
  expect(
    [["assets", "n1", "root"], ["assets", "n1", "p1"], ["assets", "n2", "root"], ["pages", "n1"], "assets"].map(matches)
  ).toEqual([true, true, false, false, false]);
});

test("a stream stopped as the tree is read reads no attachments' lists", async () => {
  const reads = new Map<string, () => void>();
  const mutate = vi.fn(async (_key: unknown): Promise<unknown[]> => []);
  const stop = new AbortController();
  const handled = {
    ...context([]).context,
    mutate,
    stopped: stop.signal,
    refresher: {
      request: (key: string, read: () => void) => reads.set(key, read),
    } as unknown as Refresher,
  };

  vi.useFakeTimers();
  onTestFinished(() => void vi.useRealTimers());
  eventHandlers.get("pages")?.({ workspace_id: "w1", notebook_id: "n1", tree: true, pages: [] }, handled);
  reads.get(unstable_serialize(["pages", "n1"]))?.();
  stop.abort();
  await vi.runAllTimersAsync();

  expect(mutate).toHaveBeenCalledTimes(1);
});

test("a pages event asks the refresher for the versions of the pages written that were read, and for their versions being edited; a tree changed, for every version's view: not for a page whose versions were never read, which it would keep", () => {
  const { requested, context: handled } = context([["page-versions", "n1", "a"]]);
  eventHandlers.get("pages")?.(
    {
      workspace_id: "w1",
      notebook_id: "n1",
      tree: true,
      pages: [
        { id: "a", revision: 2 },
        { id: "b", revision: 3 },
      ],
    },
    handled
  );

  expect(requested.filter((key) => key.includes("version"))).toEqual([
    unstable_serialize(["page-versions", "n1", "a"]),
    "page-version edited n1 a",
    "page-version views n1",
  ]);
});

test("the versions of a page being edited are read again by the refresher: their content, by the content cached, not their view, which follows its revision; another's, none", () => {
  const reads = new Map<string, () => void>();
  const edited = { editing: true };
  const states = new Map<string, unknown>([
    [unstable_serialize(["page-versions", "n1", "a"]), { data: [] }],
    [unstable_serialize(["page-version", "n1", "a", 5]), { data: edited }],
    [unstable_serialize(["page-version", "n1", "a", 2]), { data: { editing: false } }],
  ]);
  const handled = {
    ...context([]).context,
    cache: { get: (key: string) => states.get(key) } as unknown as Cache,
    refresher: {
      request: (key: string, read: () => void) => reads.set(key, read),
    } as unknown as Refresher,
  };

  eventHandlers.get("pages")?.(
    { workspace_id: "w1", notebook_id: "n1", tree: false, pages: [{ id: "a", revision: 6 }] },
    handled
  );
  reads.get("page-version edited n1 a")?.();

  const matches = (handled.mutate.mock.calls[0] as unknown[])[0] as (key: unknown) => boolean;
  expect(
    [
      ["page-version", "n1", "a", 5],
      ["page-version-view", "n1", "a", 5],
      ["page-version", "n1", "a", 2],
      ["page-version-view", "n1", "a", 2],
      ["page-version", "n1", "b", 5],
      ["page-versions", "n1", "a"],
    ].map(matches)
  ).toEqual([true, false, false, false, false, false]);
});

test("a pages event that changed the tree asks the refresher for the notebook's trash and its items, a deletion or a restore being a change of it (M8/P2 design 4.6); one that did not, not", () => {
  const reads = new Map<string, () => void>();
  const handled = {
    ...context([]).context,
    refresher: {
      request: (key: string, read: () => void) => reads.set(key, read),
    } as unknown as Refresher,
  };
  const event = { workspace_id: "w1", notebook_id: "n1", pages: [{ id: "a", revision: 2 }] };

  eventHandlers.get("pages")?.({ ...event, tree: false }, handled);
  expect(reads.has("trash n1")).toBe(false);
  eventHandlers.get("pages")?.({ ...event, tree: true }, handled);
  reads.get("trash n1")?.();

  const matches = (handled.mutate.mock.calls[0] as unknown[])[0] as (key: unknown) => boolean;
  expect(
    [
      ["trash", "n1"],
      ["trash-item", "n1", "x"],
      ["trash", "n2"],
      ["trash-item", "n2", "x"],
      ["pages", "n1"],
      "trash",
    ].map(matches)
  ).toEqual([true, true, false, false, false, false]);
});

test("any pages event asks the refresher for the notebook's history, its lists, changesets and their nodes, at its own interval: every write is a changeset or joins one (M8/P3 design 4.6)", () => {
  const reads = new Map<string, () => void>();
  const intervals = new Map<string, number | undefined>();
  const handled = {
    ...context([]).context,
    refresher: {
      request: (key: string, read: () => void, interval?: number) => {
        reads.set(key, read);
        intervals.set(key, interval);
      },
    } as unknown as Refresher,
  };

  eventHandlers.get("pages")?.({ workspace_id: "w1", notebook_id: "n1", pages: [], tree: false }, handled);
  expect(intervals.has("history n1")).toBe(true);
  expect(intervals.get("history n1")).toBeUndefined();
  reads.get("history n1")?.();

  const matches = (handled.mutate.mock.calls[0] as unknown[])[0] as (key: unknown) => boolean;
  expect(
    [
      ["changesets", "n1", "", "", ""],
      ["changesets", "n1", "u1", "web", "p1"],
      ["changeset", "n1", "c1"],
      ["changeset-changes", "n1", "c1"],
      ["changesets", "n2", "", "", ""],
      ["changeset", "n2", "c1"],
      ["changeset-changes", "n2", "c1"],
      ["pages", "n1"],
      ["trash", "n1"],
      "changesets",
    ].map(matches)
  ).toEqual([true, true, true, true, false, false, false, false, false, false]);
});

test("a mode event reads again the workspace's notebooks, by the workspace, and the notebook's upgrade, by the notebook (M11/P1 design 4.4)", () => {
  const { requested, context: handled } = context([]);
  eventHandlers.get("mode")?.({ workspace_id: "w1", notebook_id: "n1" }, handled);

  expect(handled.mutate.mock.calls).toEqual([[["notebooks", "w1"]], [["upgrade", "n1"]]]);
  expect(requested).toEqual([]);
});

test("a links event after a pages event that changed the tree asks for what it names only once that tree is read and shown: a page the unit deleted has left first (M5-polish 24.8)", async () => {
  const reads = new Map<string, () => void>();
  const requested: string[] = [];
  const mutate = vi.fn(async (_key: unknown): Promise<unknown[]> => []);
  const handled = {
    ...context([["backlinks", "n1", "a"]]).context,
    mutate,
    refresher: {
      request: (key: string, read: () => void) => {
        requested.push(key);
        reads.set(key, read);
      },
    } as unknown as Refresher,
  };
  const backlinks = unstable_serialize(["backlinks", "n1", "a"]);

  vi.useFakeTimers();
  onTestFinished(() => void vi.useRealTimers());
  eventHandlers.get("pages")?.({ workspace_id: "w1", notebook_id: "n1", tree: true, pages: [] }, handled);
  eventHandlers.get("links")?.(
    { workspace_id: "w1", notebook_id: "n1", pages: [], targets: ["a"], aliases: false },
    handled
  );
  expect(requested).not.toContain(backlinks);
  reads.get(unstable_serialize(["pages", "n1"]))?.();
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
  expect(requested).not.toContain(backlinks);
  await vi.advanceTimersByTimeAsync(0);
  expect(requested).toContain(backlinks);
});

test("a pages event during the tree read has the links event after it wait for its own read", async () => {
  const reads = new Map<string, () => void>();
  const requested: string[] = [];
  const mutate = vi.fn(async (_key: unknown): Promise<unknown[]> => []);
  const handled = {
    ...context([["backlinks", "n1", "a"]]).context,
    mutate,
    refresher: {
      request: (key: string, read: () => void) => {
        requested.push(key);
        reads.set(key, read);
      },
    } as unknown as Refresher,
  };
  const backlinks = unstable_serialize(["backlinks", "n1", "a"]);
  const tree = unstable_serialize(["pages", "n1"]);
  const pages = { workspace_id: "w1", notebook_id: "n1", tree: true, pages: [] };

  vi.useFakeTimers();
  onTestFinished(() => void vi.useRealTimers());
  eventHandlers.get("pages")?.(pages, handled);
  const first = reads.get(tree);
  first?.();
  // Another unit changed the tree as the first read is out: its links event waits for the read its pages event asks.
  eventHandlers.get("pages")?.(pages, handled);
  const second = reads.get(tree);
  expect(second).not.toBe(first);
  eventHandlers.get("links")?.(
    { workspace_id: "w1", notebook_id: "n1", pages: [], targets: ["a"], aliases: false },
    handled
  );
  await vi.advanceTimersByTimeAsync(0);
  expect(mutate.mock.calls.filter(([key]) => JSON.stringify(key) === JSON.stringify(["pages", "n1"]))).toHaveLength(1);
  expect(requested).not.toContain(backlinks);
  second?.();
  await vi.advanceTimersByTimeAsync(0);
  expect(requested).toContain(backlinks);
});

test("a links event after a pages event whose tree read the refresher started at once, the tree not read lately, waits for that read", async () => {
  const requested: string[] = [];
  let answer!: () => void;
  const mutate = vi.fn(async (key: unknown): Promise<unknown[]> => {
    if (JSON.stringify(key) === JSON.stringify(["pages", "n1"])) {
      await new Promise<void>((resolve) => {
        answer = resolve;
      });
    }
    return [];
  });
  const handled = {
    ...context([["backlinks", "n1", "a"]]).context,
    mutate,
    // As the refresher does for a key it read no sooner than its interval ago: at once.
    refresher: {
      request: (key: string, read: () => void) => {
        requested.push(key);
        read();
      },
    } as unknown as Refresher,
  };
  const backlinks = unstable_serialize(["backlinks", "n1", "a"]);

  vi.useFakeTimers();
  onTestFinished(() => void vi.useRealTimers());
  eventHandlers.get("pages")?.({ workspace_id: "w1", notebook_id: "n1", tree: true, pages: [] }, handled);
  expect(mutate.mock.calls[0]).toEqual([["pages", "n1"]]);
  eventHandlers.get("links")?.(
    { workspace_id: "w1", notebook_id: "n1", pages: [], targets: ["a"], aliases: false },
    handled
  );
  await vi.advanceTimersByTimeAsync(0);
  expect(requested).not.toContain(backlinks);
  answer();
  await vi.advanceTimersByTimeAsync(0);
  expect(requested).toContain(backlinks);
});
