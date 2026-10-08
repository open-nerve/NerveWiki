import { unstable_serialize, type Cache } from "swr";
import { expect, test, vi } from "vitest";

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

  eventHandlers.get("pages")?.({ workspace_id: "w1", notebook_id: "n1", tree: true, pages: [] }, handled);
  reads.get(unstable_serialize(["pages", "n1"]))?.();
  expect(mutate.mock.calls).toEqual([[["pages", "n1"]]]);
  await vi.waitFor(() => expect(mutate).toHaveBeenCalledTimes(2));

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

  eventHandlers.get("pages")?.({ workspace_id: "w1", notebook_id: "n1", tree: true, pages: [] }, handled);
  reads.get(unstable_serialize(["pages", "n1"]))?.();
  stop.abort();
  await new Promise((resolve) => setTimeout(resolve, 10));

  expect(mutate).toHaveBeenCalledTimes(1);
});
