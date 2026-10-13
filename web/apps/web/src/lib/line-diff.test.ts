import { describe, expect, test } from "vitest";

import { lineDiff, lineDiffLimits, type LineDiffResult, type TextChange } from "./line-diff";

/** apply is a with the changes made, which must come in order and not overlap. */
function apply(a: string, b: string, made: readonly TextChange[]): string {
  let out = "";
  let at = 0;
  for (const c of made) {
    expect(c.fromA).toBeGreaterThanOrEqual(at);
    out += a.slice(at, c.fromA) + b.slice(c.fromB, c.toB);
    at = c.toA;
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
    expect(changes(lineDiff(c, d, { ...lineDiffLimits, maxRefinedTotal: 6 }))).toEqual([
      { fromA: 6, toA: 7, fromB: 6, toB: 7 },
      { fromA: 13, toA: 16, fromB: 13, toB: 16 },
    ]);
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
