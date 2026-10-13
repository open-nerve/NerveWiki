import { Change, getChunks, presentableDiff, unifiedMergeView } from "@codemirror/merge";
import { EditorState } from "@codemirror/state";
import { describe, expect, test } from "vitest";

import { lineDiff, lineDiffLimits, type LineDiffResult, type TextChange } from "./line-diff";

/** apply is a with the changes made, which must come in order, none empty, and not overlap or meet. */
function apply(a: string, b: string, made: readonly TextChange[]): string {
  let out = "";
  let at = 0;
  for (const [i, c] of made.entries()) {
    expect(c.fromA).toBeGreaterThanOrEqual(i === 0 ? 0 : at + 1);
    expect(c.toA - c.fromA + c.toB - c.fromB).toBeGreaterThan(0);
    out += a.slice(at, c.fromA) + b.slice(c.fromB, c.toB);
    at = c.toA;
  }
  return out + a.slice(at);
}

/** changesOf is the changes as @codemirror/merge takes them. */
const changesOf = (found: readonly TextChange[]) => found.map((c) => new Change(c.fromA, c.toA, c.fromB, c.toB));

/** chunksOf is the chunks of the unified view of b against a, given the changes. */
function chunksOf(a: string, b: string, found: readonly TextChange[]) {
  const state = EditorState.create({
    doc: b,
    extensions: [
      unifiedMergeView({ original: a, mergeControls: false, diffConfig: { override: () => changesOf(found) } }),
    ],
  });
  return getChunks(state)?.chunks ?? [];
}

/** shown is what the unified view, given the changes, shows changed: the text of a its chunks remove and that of b they put instead. */
const shown = (a: string, b: string, found: readonly TextChange[]) =>
  chunksOf(a, b, found).map((chunk) => ({
    removed: a.slice(chunk.fromA, chunk.toA),
    added: b.slice(chunk.fromB, chunk.toB),
  }));

/** rebuilt is a with the chunks the view shows made: b, when they show every change. */
function rebuilt(a: string, b: string, found: readonly TextChange[]): string {
  let out = "";
  let at = 0;
  for (const chunk of chunksOf(a, b, found)) {
    out += a.slice(at, chunk.fromA) + b.slice(chunk.fromB, Math.min(chunk.toB, b.length));
    at = Math.min(chunk.toA, a.length);
  }
  return out + a.slice(at);
}

function changes(result: LineDiffResult): TextChange[] {
  if (result.kind !== "changes") {
    throw new Error(`expected changes, got ${JSON.stringify(result)}`);
  }
  return result.changes;
}

/** paragraph is the paragraph i of a long page of some 120 characters each (M8 design review B-I1). */
const paragraph = (i: number) => `段落 ${i.toString()}：${"这是一段用来比较的文字，".repeat(10)}\n`;

/** numbered is a text of count numbered lines. */
const numbered = (count: number) => Array.from({ length: count }, (_, i) => `line ${i.toString()}\n`).join("");

/** endingIn is a line of so many code units with its line break: words, the last one word. */
const endingIn = (units: number, word: string) => `${"x ".repeat((units - 2) / 2)}${word}\n`;

/** paragraphs is a text of the paragraphs, an empty line apart. */
const paragraphs = (each: readonly string[]) => `${each.join("\n\n")}\n`;

/** dense is a page of a thousand lines of words, every other letter of them shifted so far, between lines that stay. */
const dense = (shift: number) =>
  Array.from({ length: 1000 }, (_, i) => {
    const words = Array.from({ length: 40 }, (_word, w) =>
      Array.from({ length: 4 }, (_letter, l) => "abcdefghij".charAt((i + w + l + (l % 2 === 1 ? shift : 0)) % 10)).join(
        ""
      )
    );
    return `same ${i.toString()}\n${words.join(" ")}\n`;
  }).join("");

/** reworded is a thousand paragraphs, each the word numbered. */
const reworded = (word: string) => paragraphs(Array.from({ length: 1000 }, (_, i) => `${word}${i.toString()}`));

/** lines splits a text as lineDiff does: each line with its LF. */
const lines = (text: string) => text.match(/[^\n]*\n|[^\n]+$/g) ?? [];

/** fewestEdits is how many lines a shortest edit script from a to b adds and removes: by the longest common subsequence. */
function fewestEdits(a: string, b: string): number {
  const x = lines(a);
  const y = lines(b);
  const row = Array.from({ length: y.length + 1 }, () => 0);
  for (const line of x) {
    let diagonal = 0;
    for (let j = 1; j <= y.length; j++) {
      const up = row[j] ?? 0;
      row[j] = line === y[j - 1] ? diagonal + 1 : Math.max(up, row[j - 1] ?? 0);
      diagonal = up;
    }
  }
  return x.length + y.length - 2 * (row[y.length] ?? 0);
}

/** A pseudo-random generator, seeded, so that a failing case repeats: mulberry32. */
function random(seed: number): () => number {
  let state = seed;
  return () => {
    state = (state + 0x6d_2b_79_f5) | 0;
    let t = Math.imul(state ^ (state >>> 15), 1 | state);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4_294_967_296;
  };
}

describe("lineDiff", () => {
  test("changes a into b line by line, refining a small change to its characters", () => {
    const a = "# Notes\nhello world\nkeep\n";
    const b = "# Notes\nhello there\nkeep\nadded\n";
    const result = lineDiff(a, b);
    expect(result).toMatchObject({ kind: "changes", added: 2, removed: 1 });
    const found = changes(result);
    expect(apply(a, b, found)).toBe(b);
    expect(found.every((c) => c.toA - c.fromA < "hello world\n".length)).toBe(true);
  });

  test("tells a last line without its line break from the same line with it", () => {
    const result = lineDiff("x", "x\ny");
    expect(result).toMatchObject({ kind: "changes", added: 2, removed: 1 });
    expect(apply("x", "x\ny", changes(result))).toBe("x\ny");
  });

  test("answers no change for the same text, and an empty side whole", () => {
    expect(lineDiff("a\nb\n", "a\nb\n")).toEqual({ kind: "changes", changes: [], added: 0, removed: 0 });
    expect(lineDiff("", "")).toEqual({ kind: "changes", changes: [], added: 0, removed: 0 });
    expect(lineDiff("", "a\nb\n")).toMatchObject({ kind: "changes", added: 2, removed: 0 });
    expect(apply("a\nb\n", "", changes(lineDiff("a\nb\n", "")))).toBe("");
  });

  test("drops an empty change, which @codemirror/merge's diff may give", () => {
    const a = `${String.fromCodePoint(0x1f600)}中- [ ] `;
    const b = "- [ ]  ";
    expect(apply(a, b, changes(lineDiff(a, b)))).toBe(b);
  });

  test("keeps the common first and last lines out of a change", () => {
    const a = "head\nold\ntail\n";
    const b = "head\nnew one\nnew two\ntail\n";
    const found = changes(lineDiff(a, b, { ...lineDiffLimits, maxRefined: 0 }));
    expect(found).toEqual([{ fromA: 5, toA: 9, fromB: 5, toB: 21 }]);
  });

  test("keeps a change larger than the refinement's bound whole lines, and those past its total", () => {
    const a = `${"x".repeat(3000)}\n`;
    const b = `${"x".repeat(2999)}y\n`;
    expect(changes(lineDiff(a, b))).toEqual([{ fromA: 0, toA: 3001, fromB: 0, toB: 3001 }]);
    const c = "same\nab\nsame\ncd\n";
    const d = "same\nax\nsame\ncx\n";
    // The word "ab" becomes "ax"; "cd\n" stays whole.
    expect(changes(lineDiff(c, d, { ...lineDiffLimits, maxRefinedTotal: 6 }))).toEqual([
      { fromA: 5, toA: 7, fromB: 5, toB: 7 },
      { fromA: 13, toA: 16, fromB: 13, toB: 16 },
    ]);
  });

  test("counts toward the refinement's total only the changes it refines", () => {
    const a = "same\nan old block of lines\nsame\nab\n";
    const b = "same\na new block of lines\nsame\nax\n";
    // The block is too large to refine; "ab" becomes "ax" all the same.
    expect(changes(lineDiff(a, b, { ...lineDiffLimits, maxRefined: 10, maxRefinedTotal: 6 })).at(-1)).toEqual({
      fromA: 32,
      toA: 34,
      fromB: 31,
      toB: 33,
    });
  });

  test("refines a change of up to 2,000 code units on each side, and 1 MiB of them in all", () => {
    expect(changes(lineDiff(endingIn(2000, "A"), endingIn(2000, "B")))).toEqual([
      { fromA: 1998, toA: 1999, fromB: 1998, toB: 1999 },
    ]);
    expect(changes(lineDiff(`y${endingIn(2000, "A")}`, `y${endingIn(2000, "B")}`))).toEqual([
      { fromA: 0, toA: 2001, fromB: 0, toB: 2001 },
    ]);
    // 512 changes of 1,024 code units on each side make 1 MiB; the 513th stays whole.
    const page = (word: string) =>
      Array.from({ length: 513 }, (_, i) => `part ${i.toString()}\n${endingIn(1024, word)}`).join("");
    const found = changes(lineDiff(page("A"), page("B")));
    expect(found).toHaveLength(513);
    expect(found.slice(0, 512).every((c) => c.toA - c.fromA === 1)).toBe(true);
    const end = page("A").length;
    expect(found.at(-1)).toEqual({ fromA: end - 1024, toA: end, fromB: end - 1024, toB: end });
  });

  test("finds a shortest script, and makes b, on random texts", () => {
    const next = random(7);
    const text = (count: number) =>
      Array.from({ length: count }, () => `${"abcde".charAt(Math.floor(next() * 5))}\n`).join("") +
      (next() < 0.3 ? "z" : "");
    for (let i = 0; i < 300; i++) {
      const a = text(Math.floor(next() * 40));
      const b = text(Math.floor(next() * 40));
      const result = lineDiff(a, b, { ...lineDiffLimits, maxRefined: 0 });
      if (result.kind !== "changes") {
        throw new Error("too large");
      }
      expect(apply(a, b, result.changes)).toBe(b);
      expect(result.added + result.removed).toBe(fewestEdits(a, b));
    }
  });

  test("answers tooLarge, without counts, once the work passes maxSteps", () => {
    const next = random(3);
    const text = () => Array.from({ length: 2000 }, () => `${Math.floor(next() * 1000).toString()}\n`).join("");
    expect(lineDiff(text(), text(), { ...lineDiffLimits, maxSteps: 10_000 })).toEqual({
      kind: "tooLarge",
      added: null,
      removed: null,
    });
  });

  test("shows 5,000 lines added or removed line by line, not 5,001", () => {
    expect(lineDiff("", numbered(5000))).toMatchObject({ kind: "changes", added: 5000, removed: 0 });
    expect(lineDiff(numbered(5000), "")).toMatchObject({ kind: "changes", added: 0, removed: 5000 });
    expect(lineDiff("", numbered(5001))).toEqual({ kind: "tooLarge", added: 5001, removed: 0 });
  });

  test("answers tooLarge, with the counts, past maxChangedLines", () => {
    const a = Array.from({ length: 30 }, (_, i) => `a${i.toString()}\n`).join("");
    const b = Array.from({ length: 30 }, (_, i) => `b${i.toString()}\n`).join("");
    expect(lineDiff(a, b, { ...lineDiffLimits, maxChangedLines: 59 })).toEqual({
      kind: "tooLarge",
      added: 30,
      removed: 30,
    });
    expect(lineDiff(a, b, { ...lineDiffLimits, maxChangedLines: 60 })).toMatchObject({
      kind: "changes",
      added: 30,
      removed: 30,
    });
  });

  // M8/P1 review A, M1: the view draws a change that starts on an empty line in both texts from the next line on.
  test("gives the view changes that show every line they change", () => {
    const a = paragraphs(["p1", "p2", "p3", "p4", "p5"]);
    const b = paragraphs(["p1", "p3", "p5"]);
    expect(shown(a, b, changes(lineDiff(a, b)))).toEqual([
      { removed: "p2\n\n", added: "" },
      { removed: "p4\n\n", added: "" },
    ]);
    expect(shown("\nc\n\n\n", "a\n\nc\n\n", changes(lineDiff("\nc\n\n\n", "a\n\nc\n\n")))).toEqual([
      { removed: "", added: "a\n" },
      { removed: "\n", added: "" },
    ]);
    const next = random(11);
    const text = (count: number) =>
      Array.from({ length: count }, () => `${["", "", "a", "b"][Math.floor(next() * 4)] ?? ""}\n`).join("");
    for (let i = 0; i < 500; i++) {
      const c = text(Math.floor(next() * 12));
      const d = text(Math.floor(next() * 12));
      for (const limits of [lineDiffLimits, { ...lineDiffLimits, maxRefined: 0 }]) {
        const found = changes(lineDiff(c, d, limits));
        expect(rebuilt(c, d, found), JSON.stringify([c, d])).toBe(d);
        expect(shown(c, d, found).every((chunk) => chunk.removed !== "" || chunk.added !== "")).toBe(true);
      }
    }
  });

  // M8/P1 review A, I1: the view merges changes that nearly meet, splicing its array: quadratic, on the page's thread.
  test("gives the view changes it finds nothing left to merge in, however dense", () => {
    // Every other letter changed; paragraphs an empty line apart, every one of them a word that changes.
    for (const [a, b] of [
      [dense(0), dense(1)],
      [reworded("old"), reworded("new")],
    ] as const) {
      const found = changes(lineDiff(a, b));
      expect(apply(a, b, found)).toBe(b);
      expect(found.length).toBeLessThanOrEqual(1000);
      expect(presentableDiff(a, b, { override: () => changesOf(found) })).toHaveLength(found.length);
    }
  });

  // M8 design review B-I1: a page of some 30,000 characters with eight words changed, which @codemirror/merge's
  // own settings showed as the whole page removed and added.
  test("shows eight words changed in a long page as eight changes", () => {
    const a = Array.from({ length: 250 }, (_, i) => paragraph(i)).join("");
    let b = a;
    for (const i of [3, 40, 77, 110, 150, 190, 220, 249]) {
      b = b.replace(paragraph(i), paragraph(i).replace("用来比较", "供作对照"));
    }
    expect(a.length).toBeGreaterThan(30_000);
    const found = changes(lineDiff(a, b));
    expect(found).toHaveLength(8);
    expect(apply(a, b, found)).toBe(b);
  });
});
