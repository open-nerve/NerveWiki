import { expect, test } from "vitest";

import { readPages } from "./read-pages";

/** list is a list of three pages, a, b and c, read after a cursor that is the next page's letter; read is what went out. */
function list() {
  const read: string[] = [];
  const pages = new Map([
    ["a", { data: ["a"], next_cursor: "b" }],
    ["b", { data: ["b"], next_cursor: "c" }],
    ["c", { data: ["c"], next_cursor: null }],
  ]);
  return {
    read,
    page: async (cursor?: string) => {
      read.push(cursor ?? "first");
      const page = pages.get(cursor ?? "a");
      if (page === undefined) {
        throw new Error(`no page after ${String(cursor)}`);
      }
      return page;
    },
  };
}

test("reads as many pages as asked, from the first, each after the one before", async () => {
  const { read, page } = list();

  expect((await readPages(page, 2)).map((each) => each.data)).toEqual([["a"], ["b"]]);
  expect(read).toEqual(["first", "b"]);
});

test("reads the first page alone for one, and fewer pages than asked when the list ends", async () => {
  const one = list();
  expect(await readPages(one.page, 1)).toHaveLength(1);
  expect(one.read).toEqual(["first"]);

  const all = list();
  expect((await readPages(all.page, 5)).map((each) => each.data)).toEqual([["a"], ["b"], ["c"]]);
  expect(all.read).toEqual(["first", "b", "c"]);
});

test("a page that fails fails the read", async () => {
  await expect(
    readPages(async (cursor) => {
      if (cursor !== undefined) {
        throw new TypeError("offline");
      }
      return { next_cursor: "b" };
    }, 2)
  ).rejects.toThrow("offline");
});
