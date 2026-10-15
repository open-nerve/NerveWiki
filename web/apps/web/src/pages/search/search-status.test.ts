import { expect, test } from "vitest";

import { translator } from "../../i18n/i18n";
import type { SearchCount, SearchPage } from "../../services/page.service";
import { moreText, statusText } from "./search-status";

// What a search's status line says, and what More and Keep looking say (M8/P5 design 4.3, 4.4).

const en = translator("en");
const zh = translator("zh-CN");

/** page is a page of a search holding hits hits, with a next page or not, its count, and building. */
function page(hits: number, more: boolean, count: SearchCount | null = null, building = false): SearchPage {
  return {
    data: Array.from({ length: hits }, () => ({}) as SearchPage["data"][number]),
    next_cursor: more ? "next" : null,
    count,
    building,
  };
}

test("of the first page, its count: exact, about, at least, 1,000 and more, or none found yet", () => {
  const cases: [SearchCount | null, number, boolean, string, string][] = [
    [{ value: 23, kind: "exact" }, 20, true, "Results: 23.", "23 条。"],
    [{ value: 0, kind: "exact" }, 0, false, "Nothing found.", "没有找到。"],
    [{ value: 240, kind: "about" }, 20, true, "Results: about 240.", "约 240 条。"],
    [{ value: 800, kind: "at_least" }, 20, true, "Results: at least 800.", "至少 800 条。"],
    [{ value: 1000, kind: "at_least" }, 20, true, "Results: 1,000+.", "1,000+ 条。"],
    [null, 0, true, "None found yet; you can keep looking.", "还没有找到，可以继续查找。"],
  ];
  for (const [count, hits, more, english, chinese] of cases) {
    const pages = [page(hits, more, count)];
    expect(statusText(pages, hits, en, "en")).toBe(english);
    expect(statusText(pages, hits, zh, "zh-CN")).toBe(chinese);
  }
});

test("of more pages, what was read: all of it, how many; a count of none or fewer than found, so far; else the first page's", () => {
  // Read to the end: what was found, each once.
  expect(statusText([page(20, true, { value: 240, kind: "about" }), page(3, false)], 23, en, "en")).toBe(
    "Results: 23."
  );
  expect(statusText([page(20, true, { value: 240, kind: "about" }), page(3, false)], 23, zh, "zh-CN")).toBe("23 条。");
  expect(statusText([page(0, true, null), page(0, false)], 0, en, "en")).toBe("Nothing found.");
  // A count of none, or fewer than found.
  expect(statusText([page(0, true, null), page(2, true)], 2, en, "en")).toBe("Found 2 so far; there may be more.");
  expect(statusText([page(0, true, null), page(2, true)], 2, zh, "zh-CN")).toBe("已找到 2 条，可能还有更多。");
  expect(statusText([page(20, true, { value: 30, kind: "about" }), page(20, true)], 40, en, "en")).toBe(
    "Found 40 so far; there may be more."
  );
  // Nothing yet after Keep looking: the first page's.
  expect(statusText([page(0, true, null), page(0, true)], 0, en, "en")).toBe("None found yet; you can keep looking.");
  // A count past those found: the first page's.
  expect(statusText([page(20, true, { value: 240, kind: "about" }), page(20, true)], 40, en, "en")).toBe(
    "Results: about 240."
  );
  expect(statusText([page(20, true, { value: 1000, kind: "at_least" }), page(20, true)], 1040, en, "en")).toBe(
    "Found 1,040 so far; there may be more."
  );
});

test("while the index is being built, it says that the hits may be missing some, of any page read", () => {
  expect(statusText([page(2, false, { value: 2, kind: "exact" }, true)], 2, en, "en")).toBe(
    "Results: 2. The index is being built; results may be incomplete."
  );
  expect(statusText([page(2, false, { value: 2, kind: "exact" }, true)], 2, zh, "zh-CN")).toBe(
    "2 条。索引正在建立，结果可能不全。"
  );
  expect(statusText([page(20, true, { value: 30, kind: "about" }), page(10, false, null, true)], 30, en, "en")).toBe(
    "Results: 30. The index is being built; results may be incomplete."
  );
});

test("More says how many it added, or that it found none, a number as the language writes it", () => {
  expect(moreText(3, en, "en")).toBe("3 more found.");
  expect(moreText(3, zh, "zh-CN")).toBe("又找到 3 条。");
  expect(moreText(1500, en, "en")).toBe("1,500 more found.");
  expect(moreText(0, en, "en")).toBe("Nothing found in this stretch; you can keep looking.");
  expect(moreText(0, zh, "zh-CN")).toBe("这一段没有找到，可以继续查找。");
});
