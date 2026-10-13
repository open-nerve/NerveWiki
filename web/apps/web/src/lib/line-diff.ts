import { presentableDiff, type Change } from "@codemirror/merge";

/**
 * A change between two texts: the range of the first, from fromA to toA,
 * that the range of the second from fromB to toB replaces, in UTF-16 code
 * units of the texts compared. An insertion's A range is empty, a
 * deletion's B range.
 */
export type TextChange = { fromA: number; toA: number; fromB: number; toB: number };

/**
 * What lineDiff found (M8/P1 design 4.2): the changes from the first text
 * to the second, in order, with how many lines the second adds and the
 * first loses; or that the texts are too far apart to compare line by
 * line, with those counts when the comparison got as far as them, null
 * when it gave up before.
 */
export type LineDiffResult =
  | { kind: "changes"; changes: TextChange[]; added: number; removed: number }
  | { kind: "tooLarge"; added: number | null; removed: number | null };

/** The bounds of a comparison. */
export type LineDiffLimits = {
  /** The work the line comparison may do, in steps along the diagonals and their snakes. */
  maxSteps: number;
  /** The most lines added and removed together that are shown line by line. */
  maxChangedLines: number;
  /** A change of at most so many code units on each side is refined to the characters that differ. */
  maxRefined: number;
  /** The code units refined in all: the changes past them stay whole lines. */
  maxRefinedTotal: number;
};

/**
 * The comparison's bounds (M8 design 4.3): 20 million steps take some
 * 150 milliseconds on a laptop.
 */
export const lineDiffLimits: LineDiffLimits = {
  maxSteps: 20_000_000,
  maxChangedLines: 5000,
  maxRefined: 2000,
  maxRefinedTotal: 1 << 20,
};

/**
 * lineDiff compares two texts line by line, each line with its line
 * break, an LF (the caller splits them as the editor does), so that a
 * last line without one differs from the same line with it: Myers'
 * algorithm on the lines, in linear space, after their common first and
 * last lines; an insertion or a deletion then moves down past the lines
 * after it that its own lines repeat, so that none starts on an empty line
 * the view would draw it under. A change of a few lines on each side is
 * then refined to the words that differ by @codemirror/merge's own
 * presentable diff (presentable), with the scan limit its views use, until
 * maxRefinedTotal code units are; a larger one, and those past it, stay
 * whole lines. Changes less than three code units apart are merged, as the
 * view merges them, and empty ones dropped, so that the view, on the
 * page's thread, finds none left to merge (M8/P1 review A, I1). It answers tooLarge once the work
 * passes maxSteps, or when more than maxChangedLines lines change.
 */
export function lineDiff(a: string, b: string, limits: LineDiffLimits = lineDiffLimits): LineDiffResult {
  const linesA = splitLines(a);
  const linesB = splitLines(b);
  const ids = new Map<string, number>();
  const idsOf = (lines: readonly string[]) =>
    Int32Array.from(lines, (line) => {
      let id = ids.get(line);
      if (id === undefined) {
        id = ids.size;
        ids.set(line, id);
      }
      return id;
    });
  const idsA = idsOf(linesA);
  const idsB = idsOf(linesB);
  const found = lineEdits(idsA, idsB, limits.maxSteps);
  if (found === undefined) {
    return { kind: "tooLarge", added: null, removed: null };
  }
  const edits = slide(found, idsA, idsB);
  let added = 0;
  let removed = 0;
  for (const edit of edits) {
    added += edit.toB - edit.fromB;
    removed += edit.toA - edit.fromA;
  }
  if (added + removed > limits.maxChangedLines) {
    return { kind: "tooLarge", added, removed };
  }
  const startsA = lineStarts(linesA);
  const startsB = lineStarts(linesB);
  const changes: TextChange[] = [];
  const push = (change: TextChange) => {
    if (change.fromA === change.toA && change.fromB === change.toB) {
      return;
    }
    const last = changes.at(-1);
    if (last !== undefined && last.toA > change.fromA - 3 && last.toB > change.fromB - 3) {
      last.toA = change.toA;
      last.toB = change.toB;
    } else {
      changes.push(change);
    }
  };
  let refined = 0;
  for (const edit of edits) {
    const fromA = startsA[edit.fromA] ?? 0;
    const toA = startsA[edit.toA] ?? 0;
    const fromB = startsB[edit.fromB] ?? 0;
    const toB = startsB[edit.toB] ?? 0;
    const size = toA - fromA + toB - fromB;
    if (toA - fromA > limits.maxRefined || toB - fromB > limits.maxRefined || refined + size > limits.maxRefinedTotal) {
      push({ fromA, toA, fromB, toB });
      continue;
    }
    refined += size;
    for (const c of presentable(a.slice(fromA, toA), b.slice(fromB, toB))) {
      push({ fromA: fromA + c.fromA, toA: fromA + c.toA, fromB: fromB + c.fromB, toB: fromB + c.toB });
    }
  }
  return { kind: "changes", changes, added, removed };
}

/**
 * presentable is @codemirror/merge's presentable diff of a and b, made
 * presentable again until a pass merges nothing more: a pass merges short
 * insertions and deletions into a replacement it does not widen to its
 * words, which the view, on the page's thread, would widen and merge on,
 * splicing the whole text's array (M8/P1 fix check 1, N2).
 */
function presentable(a: string, b: string): readonly Change[] {
  const again = (changes: readonly Change[]) => presentableDiff(a, b, { override: () => changes.slice() });
  let found = presentableDiff(a, b, { scanLimit: 500 });
  for (let next = again(found); next.length < found.length; next = again(found)) {
    found = next;
  }
  return found;
}

/** splitLines is text's lines, each with its LF; the last without one when the text does not end in one. */
function splitLines(text: string): string[] {
  const lines: string[] = [];
  let start = 0;
  for (let end = text.indexOf("\n"); end !== -1; end = text.indexOf("\n", start)) {
    lines.push(text.slice(start, end + 1));
    start = end + 1;
  }
  if (start < text.length) {
    lines.push(text.slice(start));
  }
  return lines;
}

/** lineStarts is where each line starts, and after them the text's end. */
function lineStarts(lines: readonly string[]): number[] {
  const starts = [0];
  let at = 0;
  for (const line of lines) {
    at += line.length;
    starts.push(at);
  }
  return starts;
}

/** An edit of lines: the first text's lines fromA to toA replaced by the second's fromB to toB. */
type LineEdit = { fromA: number; toA: number; fromB: number; toB: number };

/** A part of the comparison left to do: the lines aLo to aHi of the first against bLo to bHi of the second. */
type Part = { aLo: number; aHi: number; bLo: number; bHi: number };

/**
 * lineEdits are the edits from a to b, lines given as numbers, in order;
 * undefined once the work passes maxSteps. Each
 * part of the comparison is split where the forward and the backward
 * searches of Myers' linear space refinement (1986, 4b) meet, until one
 * side is empty; the parts wait on a stack, the leftmost on top, rather
 * than in recursive calls, whose depth an unlucky input could take to its
 * number of edits.
 */
function lineEdits(a: Int32Array, b: Int32Array, maxSteps: number): LineEdit[] | undefined {
  const edits: LineEdit[] = [];
  const budget = { left: maxSteps };
  const stack: Part[] = [{ aLo: 0, aHi: a.length, bLo: 0, bHi: b.length }];
  for (let part = stack.pop(); part !== undefined; part = stack.pop()) {
    let { aLo, aHi, bLo, bHi } = part;
    while (aLo < aHi && bLo < bHi && a[aLo] === b[bLo]) {
      aLo++;
      bLo++;
    }
    while (aLo < aHi && bLo < bHi && a[aHi - 1] === b[bHi - 1]) {
      aHi--;
      bHi--;
    }
    if (aLo === aHi || bLo === bHi) {
      if (aLo < aHi || bLo < bHi) {
        edits.push({ fromA: aLo, toA: aHi, fromB: bLo, toB: bHi });
      }
      continue;
    }
    const split = bisect(a.subarray(aLo, aHi), b.subarray(bLo, bHi), budget);
    if (split === undefined) {
      return undefined;
    }
    if (split === null) {
      edits.push({ fromA: aLo, toA: aHi, fromB: bLo, toB: bHi });
      continue;
    }
    // The right part waits under the left, which is taken first: the edits come in order.
    stack.push(
      { aLo: aLo + split.x, aHi, bLo: bLo + split.y, bHi },
      { aLo, aHi: aLo + split.x, bLo, bHi: bLo + split.y }
    );
  }
  return edits;
}

/**
 * slide is edits, in order, those that meet joined, with each insertion
 * and each deletion moved down a line while its first line is the same as
 * the line after it, as far as the next edit, which it then joins: the
 * same lines changed, but none starting on an empty line followed by an
 * empty line. @codemirror/merge draws a change that starts where a line
 * ends in both texts, as on an empty line, from the next line on: the
 * empty line would go unmarked, and a change of that line alone show
 * nothing (M8/P1 review A, M1).
 */
function slide(edits: readonly LineEdit[], a: Int32Array, b: Int32Array): LineEdit[] {
  const slid: LineEdit[] = [];
  let edit = edits[0];
  for (let i = 1; edit !== undefined; i++) {
    const next = edits[i];
    const end = next?.fromA ?? a.length;
    let { fromA, toA, fromB, toB } = edit;
    while (toA < end && ((fromB === toB && a[fromA] === a[toA]) || (fromA === toA && b[fromB] === b[toB]))) {
      fromA++;
      toA++;
      fromB++;
      toB++;
    }
    if (next !== undefined && toA === end) {
      edit = { fromA, toA: next.toA, fromB, toB: next.toB };
    } else {
      slid.push({ fromA, toA, fromB, toB });
      edit = next;
    }
  }
  return slid;
}

/**
 * bisect is a point of a shortest edit script from a to b where it may be
 * split in two, both parts smaller (Myers 1986, 4b, as diff-match-patch's
 * bisect finds it): the forward and the backward searches run in turn, d
 * edits at a time, each keeping the furthest x it reached on each
 * diagonal k = x − y, until the two reach past each other on one
 * diagonal; the point is the forward search's there. A search that left
 * the grid stops trying the diagonals beyond. Each diagonal tried, and
 * each line a snake follows, spends a step of budget: undefined once none
 * are left; null if the searches never met, which happens only when a and
 * b have no line in common: then replacing the whole of a is the shortest
 * script.
 */
function bisect(a: Int32Array, b: Int32Array, budget: { left: number }): { x: number; y: number } | null | undefined {
  const n = a.length;
  const m = b.length;
  const maxD = Math.ceil((n + m) / 2);
  const offset = maxD;
  const forward = new Int32Array(2 * maxD + 2).fill(-1);
  // The backward search's x and y are counted back from the ends.
  const backward = new Int32Array(2 * maxD + 2).fill(-1);
  forward[offset + 1] = 0;
  backward[offset + 1] = 0;
  const delta = n - m;
  // With an odd delta the forward search finds the meeting, otherwise the backward.
  const front = delta % 2 !== 0;
  let fStart = 0;
  let fEnd = 0;
  let bStart = 0;
  let bEnd = 0;
  const at = (v: Int32Array, k: number) => v[offset + k] ?? -1;
  for (let d = 0; d < maxD; d++) {
    for (let k = -d + fStart; k <= d - fEnd; k += 2) {
      if (--budget.left < 0) {
        return undefined;
      }
      let x =
        k === -d || (k !== d && at(forward, k - 1) < at(forward, k + 1)) ? at(forward, k + 1) : at(forward, k - 1) + 1;
      let y = x - k;
      while (x < n && y < m && a[x] === b[y]) {
        x++;
        y++;
        budget.left--;
      }
      forward[offset + k] = x;
      if (x > n) {
        fEnd += 2;
      } else if (y > m) {
        fStart += 2;
      } else if (front) {
        const back = at(backward, delta - k);
        if (back !== -1 && x >= n - back) {
          return { x, y };
        }
      }
    }
    for (let k = -d + bStart; k <= d - bEnd; k += 2) {
      if (--budget.left < 0) {
        return undefined;
      }
      let x =
        k === -d || (k !== d && at(backward, k - 1) < at(backward, k + 1))
          ? at(backward, k + 1)
          : at(backward, k - 1) + 1;
      let y = x - k;
      while (x < n && y < m && a[n - x - 1] === b[m - y - 1]) {
        x++;
        y++;
        budget.left--;
      }
      backward[offset + k] = x;
      if (x > n) {
        bEnd += 2;
      } else if (y > m) {
        bStart += 2;
      } else if (!front) {
        const fx = at(forward, delta - k);
        if (fx !== -1 && fx >= n - x) {
          return { x: fx, y: fx - (delta - k) };
        }
      }
    }
  }
  return null;
}
