import { unstable_serialize, type Cache } from "swr";
import { expect, test, vi } from "vitest";

import { eventHandlers } from "./handlers";
import type { Refresher } from "./refresher";

// The handlers' reads through the refresher (M5 design 4.11, M6/P7 design 11).

/** context is a handler's, the keys of read cached (their first read out: no data yet), and what it asks the refresher for. */
function context(read: unknown[][]) {
  const cached = new Set(read.map((key) => unstable_serialize(key)));
  const requested: string[] = [];
  return {
    requested,
    context: {
      cache: {
        get: (key: string) => (cached.has(key) ? { isLoading: true, isValidating: true } : undefined),
      } as unknown as Cache,
      mutate: vi.fn(),
      refresher: { request: (key: string) => void requested.push(key) } as unknown as Refresher,
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

test("a pages event that changed the tree asks the refresher for the tree, which a run of uploads reads once (M7/P2 design 3.10); one that did not, not", () => {
  const { requested, context: handled } = context([]);
  const event = { workspace_id: "w1", notebook_id: "n1", pages: [] };
  eventHandlers.get("pages")?.({ ...event, tree: false }, handled);
  expect(requested.filter((key) => key === unstable_serialize(["pages", "n1"]))).toEqual([]);

  eventHandlers.get("pages")?.({ ...event, tree: true }, handled);
  expect(requested.filter((key) => key === unstable_serialize(["pages", "n1"]))).toEqual([
    unstable_serialize(["pages", "n1"]),
  ]);
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
