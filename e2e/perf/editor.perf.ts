import path from "node:path";
import { setTimeout as sleep } from "node:timers/promises";

import { createClient } from "@nervewiki/api-client";
import { expect, test, type BrowserContext, type Page } from "@playwright/test";

import { login, signInContext } from "../fixtures/auth";
import { watchPage } from "../fixtures/browser";
import { goto } from "../fixtures/navigation";
import { createNotebook } from "../fixtures/notebooks";
import { createPage } from "../fixtures/pages";
import { startEditing } from "../fixtures/wiki-pages";
import { buildParse } from "./parse/build";
import type { NwParse, Parsed } from "./parse/entry";
import { median, record } from "./record";
import { perfEnv } from "./setup";

// B9, the editor's parse at its limits (M12/P4 design 4.4, decision 465): each costly shape of
// W/editor/markdown.test.ts grown to the limit, in a paragraph between two ordinary ones, parsed in Chromium by the
// parse page. Three cells judged: the whole document parsed once, as a paste, the paragraph inlineLimit long; and the
// change of one character, the paragraph inlineLimit - 1 long before it and inlineLimit after, in its middle and at
// its end (where one types most). Each measure asserts it parsed within the limit: the paragraph's length, and the
// link, emphasis and code span of its head (inline() of markdown.test.ts), which a paragraph past the limit, plain
// text, does not have. Two warm-ups and five measures a shape and a cell; the median of five, the larger of two
// rounds, against 80 ms. Besides, only recorded: the editor's parser alone on the whole document, a check on the
// first cell; and a key pressed in the real editor on the three costliest shapes, from its keydown to the frame after
// next, and where the key went.
//
// Changed after review B on M12/P4/S2: the whole document is parsed once (parse/entry.ts; it was twice, I1); the
// definitions of many lines that never close, or close on their last, are among the shapes (I2: the first rounds had
// only one over two lines); a character typed at the paragraph's end besides its middle (M2: in its middle a run of
// spaces is split in two, half the cost); the line with a pipe has the head too (N3). Each cell of each shape is
// parsed in a page of its own, its heap collected first: in one page for all, a definition's cells read up to twice
// what they read alone, after the shapes before it had filled the heap (each of its lines builds the paragraph's text
// anew; the check of the fixes, B's C1-M1).

const round = process.env.NW_PERF_ROUND ?? "1";
const warmUps = 2;
const times = 5;

/** markdown.test.ts's inline() begins its paragraph with this: a link, emphasis and a code span. */
const head = "[a](b) *e* `c` ";
const intro = "An ordinary paragraph before it.\n\n";
const outro = "\n\nAn ordinary paragraph after it.\n";

/**
 * A shape: its document with its costly block, length long where its paragraph is the limit's (the two-line
 * definition's first line); the offset of what to look at in it, and where a character goes in its middle and at its
 * end; and what a parse within the limit has.
 */
interface Shape {
  name: string;
  doc(length: number): { doc: string; at: number; middle: number; end: number };
  within(parsed: Parsed, length: number): boolean;
}

/** markdown.test.ts's costly shapes, each a paragraph about length long whose inline parse costs its square. */
const costly: [string, (length: number) => string][] = [
  ["links", (length) => "[[Page]] ".repeat(length / 9)],
  ["links and brackets that close none", (length) => "[a](b) ] ".repeat(length / 9)],
  ["emphasis that closes none", (length) => "a* ".repeat(length / 3)],
  ["emphasis closed in part", (length) => "*a** ".repeat(length / 5)],
  ["strong emphasis in emphasis", (length) => "***a*** ".repeat(length / 8)],
  ["a run of spaces", (length) => `x${" ".repeat(length - 2)}y`],
  ["an escape, then a run of spaces", (length) => `\\*${" ".repeat(length - 3)}y`],
  ["processing instructions", (length) => `x${"<?".repeat(length / 2 - 1)}`],
  ["images not closed", (length) => "![a](".repeat(length / 5)],
  ["autolinks not closed", (length) => "<http://a".repeat(length / 9)],
  ["link titles not closed", (length) => "[a](b (x ".repeat(length / 9)],
  [
    "code spans of distinct lengths",
    (length) => {
      let text = "";
      for (let ticks = 1; text.length < length; ticks++) {
        text += `${"`".repeat(ticks)}a`;
      }
      return text;
    },
  ],
  ["nested emphasis", (length) => "*a ".repeat(length / 6) + "b* ".repeat(length / 6)],
  ["nested brackets", (length) => "[".repeat(length / 2) + "]".repeat(length / 2)],
  ["marks of emphasis and code only", (length) => "*`*`".repeat(length / 4)],
];

/** text cut or padded with "x" to length, its last character not a space: a paragraph's trailing spaces are not its. */
function exactly(text: string, length: number): string {
  const fitted = text.length >= length ? text.slice(0, length) : text + "x".repeat(length - text.length);
  return /\s$/.test(fitted) ? `${fitted.slice(0, -1)}x` : fitted;
}

/** Within the limit: the paragraph is length long and has its head's link, emphasis and code span. */
function headed(parsed: Parsed, length: number): boolean {
  return parsed.paragraph === length && parsed.inline.join() === "Emphasis,InlineCode,Link";
}

/** A paragraph of head and fill, length long, after prefix on its line (the marks of the blocks it is in). */
function paragraph(prefix: string, fill: (length: number) => string) {
  return (length: number) => {
    const text = head + exactly(fill(length - head.length), length - head.length);
    const at = intro.length + prefix.length;
    return { doc: `${intro}${prefix}${text}${outro}`, at, middle: at + Math.floor(length / 2), end: at + length };
  };
}

/**
 * A block of lines of one "x" after first, the shortest a line can be, then last; its last "x" line padded to make it
 * length long.
 */
function lines(first: string, length: number, last = ""): string {
  const body = first + "\nx".repeat(Math.floor((length - first.length - last.length) / 2));
  return body + "x".repeat(length - last.length - body.length) + last;
}

/** A JSON array of objects a line, markdown.test.ts's, its last object padded to make it length long. */
function json(length: number): string {
  const rows = ["["];
  let size = 1;
  for (let i = 0; ; i++) {
    const row = `  {"id": ${i}, "name": "item ${i}"},`;
    if (size + 1 + row.length + 2 > length) break;
    rows.push(row);
    size += 1 + row.length;
  }
  const padded = (rows.at(-1) ?? "").replace(/"\},$/, `${"x".repeat(length - size - 2)}"},`);
  return [...rows.slice(0, -1), padded, "]"].join("\n");
}

/**
 * A paragraph that may be a link reference definition, block length long: markdown.test.ts's three that never close,
 * each line reading the paragraph again from its start (M6 closeout FB3-I1). Within the limit: the paragraph is
 * length long and no definition. That it was read as one may be to its last line is markdown.ts's: a paragraph stops
 * being one once its lines before the next pass inlineLimit (LongReferenceAsText), which these, the limit long, never
 * do; the same lines closed on the last are a definition (the shape after these three).
 */
function neverClosed(block: (length: number) => string): Pick<Shape, "doc" | "within"> {
  return {
    doc: (length) => {
      const at = intro.length;
      return { doc: `${intro}${block(length)}${outro}`, at, middle: at + Math.floor(length / 2), end: at + length };
    },
    within: (parsed, length) => parsed.paragraph === length && parsed.blocks.LinkReference === undefined,
  };
}

const shapes: Shape[] = [
  ...costly.map(([name, fill]) => ({ name, doc: paragraph("", fill), within: headed })),
  {
    // 49 lists one in another, the deepest whose paragraph the editor still reads inline: a list and its item are two
    // blocks, the document one; at 50 the paragraph is plain text (blockDepth).
    name: "lists to blockDepth",
    doc: paragraph("- ".repeat(49), (length) => "x".repeat(length)),
    within: (parsed, length) => headed(parsed, length) && parsed.blocks.ListItem === 49,
  },
  {
    // 98 quotes, the same: at 99 the paragraph is plain text.
    name: "quotes to blockDepth",
    doc: paragraph("> ".repeat(98), (length) => "x".repeat(length)),
    within: (parsed, length) => headed(parsed, length) && parsed.blocks.Blockquote === 98,
  },
  {
    // A definition whose title ends on its second line, its first line length long: one up to the limit. Its end is
    // its first line's. It has no head: a definition begins with its label.
    name: "a link reference definition over two lines",
    doc: (length) => {
      const at = intro.length;
      return {
        doc: `${intro}[a]: http://x "${"t".repeat(length - 15)}\nt"${outro}`,
        at,
        middle: at + 15 + Math.floor((length - 15) / 2),
        end: at + length,
      };
    },
    within: (parsed) => parsed.blocks.LinkReference === 1,
  },
  { name: "a definition's bracket never closed, over lines", ...neverClosed((length) => lines("[", length)) },
  { name: "a definition's title never closed, over lines", ...neverClosed((length) => lines('[a]: b "x', length)) },
  { name: "a JSON array, a definition's bracket never closed", ...neverClosed(json) },
  {
    // The title never closed but on the last line: a definition the limit long, each line having read it again. Its
    // end is before the closing quote.
    name: "a definition's title closed on its last line",
    doc: (length) => {
      const at = intro.length;
      return {
        doc: `${intro}${lines('[a]: b "x', length, '"')}${outro}`,
        at,
        middle: at + Math.floor(length / 2),
        end: at + length - 1,
      };
    },
    within: (parsed, length) => parsed.blocks.LinkReference === 1 && parsed.definition === length,
  },
  {
    // A line with a pipe, then one of spaces: whether it heads a table cost the square of the spaces.
    name: "a line with a pipe, then spaces",
    doc: (length) => {
      const at = intro.length;
      return {
        doc: `${intro}${head}a\nb|c\n${" ".repeat(length - head.length - 7)}x${outro}`,
        at,
        middle: at + Math.floor(length / 2),
        end: at + length,
      };
    },
    within: (parsed, length) => headed(parsed, length) && parsed.blocks.Table === undefined,
  },
];

/** The medians of the whole parse this run measured, by shape, and the limit: the editor's keys go to the costliest. */
const wholeMedians = new Map<string, number>();
let parsedLimit = 0;

/** A page of its own with the parse page's script, its heap collected: no parse before weighs on what it measures. */
async function parsePage(context: BrowserContext, script: string): Promise<Page> {
  const page = await context.newPage();
  await page.setContent("<!doctype html><html><body></body></html>");
  await page.addScriptTag({ path: script });
  const cdp = await context.newCDPSession(page);
  await cdp.send("HeapProfiler.collectGarbage");
  await cdp.detach();
  return page;
}

/** In a page of its own, count parses of doc as cell has them. */
async function parsesIn(
  context: BrowserContext,
  script: string,
  arg: { doc: string; at: number; where: number; cell: string; count: number }
): Promise<Parsed[]> {
  const page = await parsePage(context, script);
  try {
    return await page.evaluate((each) => {
      const parse = (window as unknown as { nwParse: NwParse }).nwParse;
      return Array.from({ length: each.count }, () => {
        if (each.cell === "whole") return parse.whole(each.doc, each.at);
        if (each.cell === "parser") return parse.parser(each.doc, each.at);
        return parse.insert(each.doc, each.at, each.where);
      });
    }, arg);
  } finally {
    await page.close();
  }
}

test("the editor's parse at its limits", async ({ browser }) => {
  const script = await buildParse(path.join(import.meta.dirname, "../test-results/perf/parse"));
  const context = await browser.newContext();
  try {
    const first = await parsePage(context, script);
    const limit = await first.evaluate(() => (window as unknown as { nwParse: NwParse }).nwParse.inlineLimit);
    await first.close();
    parsedLimit = limit;
    // Without the limits, a paragraph of 200,000 spaces takes seconds: the page parses what the editor does.
    const [spaces] = await parsesIn(context, script, {
      doc: `x${" ".repeat(199_998)}y`,
      at: 0,
      where: 0,
      cell: "whole",
      count: 1,
    });
    record("b9-spaces", { round, ms: spaces?.ms });
    expect(spaces?.ms, "a paragraph of 200,000 spaces parsed whole, in ms").toBeLessThan(1_000);
    await measureShapes(context, script, limit);
  } finally {
    await context.close();
  }
});

/** Each shape's four cells, each in a page of its own. */
async function measureShapes(context: BrowserContext, script: string, limit: number): Promise<void> {
  for (const shape of shapes) {
    for (const cell of ["whole", "parser", "middle", "end"] as const) {
      // The change of one character takes the paragraph from one under the limit to the limit (design 4.4).
      const length = cell === "whole" || cell === "parser" ? limit : limit - 1;
      const { doc, at, middle, end } = shape.doc(length);
      // oxlint-disable-next-line no-await-in-loop -- one shape and cell at a time
      const parses = await parsesIn(context, script, {
        doc,
        at,
        where: cell === "end" ? end : middle,
        cell,
        count: warmUps + times,
      });
      const measured = parses.slice(warmUps);
      const values = measured.map((parsed) => parsed.ms);
      const last = measured.at(-1);
      record("b9", {
        round,
        shape: shape.name,
        cell,
        values,
        median: median(values),
        paragraph: last?.paragraph,
        definition: last?.definition,
        inline: last?.inline,
        blocks: last?.blocks,
      });
      for (const parsed of measured) {
        expect(shape.within(parsed, limit), `${shape.name}, ${cell}: within the limit ${JSON.stringify(parsed)}`).toBe(
          true
        );
      }
      if (cell === "whole") wholeMedians.set(shape.name, median(values));
    }
  }
}

test("a key in the editor, on the costliest shapes", async ({ browser }) => {
  const env = perfEnv();
  const api = createClient({ baseUrl: env.baseURL });
  const admin = await login(api, env.admin);
  const limit = parsedLimit;
  const costliest = [...wholeMedians].toSorted((a, b) => b[1] - a[1]).slice(0, 3);
  test.skip(costliest.length === 0, "the parse's test runs first, in the same run");
  const notebook = await createNotebook(api, admin.access_token, env.workspace, `Editor ${round}`, "editor");
  const context = await browser.newContext({ baseURL: env.baseURL });
  try {
    await signInContext(context, env.baseURL, admin);
    const page = await context.newPage();
    await watchPage(page);
    for (const [name] of costliest) {
      const shape = shapes.find((each) => each.name === name);
      if (shape === undefined) continue;
      // oxlint-disable-next-line no-await-in-loop -- one page at a time
      const created = await createPage(
        api,
        admin.access_token,
        notebook.id,
        `B9 ${name}`,
        null,
        shape.doc(limit - 1).doc
      );
      // oxlint-disable-next-line no-await-in-loop -- the same
      await goto(page, `/${env.workspace}/notebooks/${notebook.id}/pages/${created.id}`);
      // oxlint-disable-next-line no-await-in-loop -- the same
      const content = await startEditing(page);
      // The block's line: the intro, a blank line, then it.
      // oxlint-disable-next-line no-await-in-loop -- the same
      await content.locator(".cm-line").nth(2).click();
      // oxlint-disable-next-line no-await-in-loop -- the same
      const caret = await caretOf(page);
      const keys: { ms: number; longest: number }[] = [];
      for (let i = 1; i <= times; i++) {
        // oxlint-disable-next-line no-await-in-loop -- one key at a time, the editor at rest before it
        keys.push(await keyOnce(page));
        // oxlint-disable-next-line no-await-in-loop -- undone: the next key again takes the paragraph to the limit
        await page.keyboard.press("ControlOrMeta+z");
      }
      record("b9-key", { round, shape: name, caret, values: keys, median: median(keys.map((key) => key.ms)) });
    }
  } finally {
    await context.close();
  }
});

/**
 * Where the key goes: the caret's line, its index among the lines the editor drew, its offset in it and its length;
 * the cost of a key turns on where it splits the paragraph (review B's M2 on M12/P4/S2).
 */
async function caretOf(page: Page): Promise<{ line: number; offset: number; length: number }> {
  return page.evaluate(() => {
    const selection = getSelection();
    const node = selection?.anchorNode ?? null;
    const line = (node instanceof Element ? node : node?.parentElement)?.closest(".cm-line");
    if (selection === null || node === null || line === null || line === undefined) throw new Error("no caret");
    const range = document.createRange();
    range.setStart(line, 0);
    range.setEnd(node, selection.anchorOffset);
    return {
      line: [...document.querySelectorAll(".cm-content .cm-line")].indexOf(line),
      offset: range.toString().length,
      length: line.textContent?.length ?? 0,
    };
  });
}

/** One key: once the page has rested a second, "a" pressed; from its keydown to two frames later, and the long tasks. */
async function keyOnce(page: Page): Promise<{ ms: number; longest: number }> {
  await sleep(1_000);
  await page.evaluate(() => {
    const w = window as unknown as {
      nwKey?: { at: number; painted: number };
      nwLong?: { start: number; end: number }[];
    };
    w.nwKey = undefined;
    if (w.nwLong === undefined) {
      const tasks: { start: number; end: number }[] = [];
      w.nwLong = tasks;
      new PerformanceObserver((list) => {
        for (const entry of list.getEntries())
          tasks.push({ start: entry.startTime, end: entry.startTime + entry.duration });
      }).observe({ type: "longtask", buffered: true });
    }
    addEventListener(
      "keydown",
      (event) => {
        const at = event.timeStamp;
        requestAnimationFrame(() =>
          requestAnimationFrame(() => {
            w.nwKey = { at, painted: performance.now() };
          })
        );
      },
      { capture: true, once: true }
    );
  });
  await page.keyboard.press("a");
  await page.waitForFunction(() => (window as unknown as { nwKey?: unknown }).nwKey !== undefined, undefined, {
    timeout: 30_000,
  });
  return page.evaluate(() => {
    const w = window as unknown as { nwKey: { at: number; painted: number }; nwLong: { start: number; end: number }[] };
    const during = w.nwLong.filter((task) => task.start >= w.nwKey.at && task.start <= w.nwKey.painted);
    return { ms: w.nwKey.painted - w.nwKey.at, longest: Math.max(0, ...during.map((task) => task.end - task.start)) };
  });
}
