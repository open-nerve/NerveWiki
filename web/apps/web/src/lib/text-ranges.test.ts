import { expect, test } from "vitest";

import { partsOf } from "./text-ranges";

// A search's hits in a name or an excerpt (M8/P5 design 4.3): the server's UTF-16 ranges, cut as given.

const hits = (text: string, ranges: { start: number; end: number }[]) =>
  partsOf(text, ranges).map(({ text: part, hit }) => (hit ? `[${part}]` : part));

test("the hits are cut where the ranges say, the text between them kept", () => {
  expect(hits("Go语言入门", [{ start: 0, end: 4 }])).toEqual(["[Go语言]", "入门"]);
  expect(hits("rapid api", [{ start: 1, end: 4 }])).toEqual(["r", "[api]", "d api"]);
  expect(hits("数据库与全文检索", [{ start: 6, end: 8 }])).toEqual(["数据库与全文", "[检索]"]);
  expect(hits("No hit", [])).toEqual(["No hit"]);
});

test("ranges count UTF-16 code units: a character past the BMP takes two", () => {
  // "📘" is a surrogate pair: the hit after it starts at 3.
  const text = "📘 数据库";
  expect(text.length).toBe(6);
  expect(hits(text, [{ start: 3, end: 6 }])).toEqual(["📘 ", "[数据库]"]);
  expect(hits(text, [{ start: 0, end: 2 }])).toEqual(["[📘]", " 数据库"]);
});

test("ranges side by side are hits side by side; one past the text is cut at its end, one left empty is none", () => {
  expect(
    hits("AI and ML", [
      { start: 0, end: 2 },
      { start: 2, end: 6 },
    ])
  ).toEqual(["[AI]", "[ and]", " ML"]);
  expect(hits("abc", [{ start: 1, end: 10 }])).toEqual(["a", "[bc]"]);
  expect(
    hits("abc", [
      { start: 1, end: 1 },
      { start: 3, end: 5 },
    ])
  ).toEqual(["abc"]);
  expect(hits("", [{ start: 0, end: 2 }])).toEqual([]);
});
