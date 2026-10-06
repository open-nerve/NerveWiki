import type { MermaidConfig } from "mermaid";
import { afterEach, expect, test, vi } from "vitest";

import { translator } from "../i18n/i18n";
import { diagramLimit, diagrams, edgeLimit, keptDrawings, type Drawer, type Watch } from "./diagrams";
import type { ReadingContext } from "./enhancement";

// The diagrams, drawn by mermaid (M6/P6 design 10). The drawings kept are
// the module's, across the tests: each test draws sources of its own.

afterEach(() => document.body.replaceChildren());

const context = (theme: "light" | "dark" = "light"): ReadingContext => ({
  workspace: "lab",
  notebook: "n",
  page: "p",
  revision: 1,
  role: "reader",
  t: translator("en"),
  theme,
  reload: () => undefined,
  navigate: () => undefined,
  report: () => undefined,
  unresolved: () => undefined,
});

/**
 * drawer is a mermaid that draws <svg id> of its text, recording the
 * options it was given and each drawing; it cannot draw a text with
 * "bad". A drawing of a text with "held" waits for release.
 */
function drawer() {
  const configs: MermaidConfig[] = [];
  const drawn: string[] = [];
  const held: (() => void)[] = [];
  const draw: Drawer = {
    initialize: (config) => configs.push(config),
    render: async (id, text) => {
      drawn.push(`${id} ${text}`);
      if (text.includes("held")) {
        await new Promise<void>((resolve) => held.push(resolve));
      }
      if (text.includes("bad")) {
        throw new Error("Parse error");
      }
      return { svg: `<svg id="${id}"><text>${text}</text></svg>` };
    },
  };
  return { configs, drawn, held, draw };
}

/** watcher is a Watch the test shows each element through; it records which it watches and which it stopped. */
function watcher() {
  const watched = new Map<Element, () => void>();
  const stopped: Element[] = [];
  const watch: Watch = (element, see) => {
    watched.set(element, see);
    return () => stopped.push(element);
  };
  return { watched, stopped, watch, show: (element: Element) => watched.get(element)?.() };
}

/** view is an article of a mermaid block of each source, and another block, in the page. */
function view(...sources: string[]) {
  const article = document.createElement("article");
  for (const source of sources) {
    const pre = document.createElement("pre");
    const code = Object.assign(document.createElement("code"), { className: "language-mermaid", textContent: source });
    pre.append(code);
    article.append(pre);
  }
  article.insertAdjacentHTML("beforeend", '<pre><code class="language-go">go</code></pre>');
  document.body.append(article);
  return { article, blocks: [...article.querySelectorAll("pre")].slice(0, sources.length) };
}

/** settled lets what is out finish. */
const settled = () => new Promise((resolve) => setTimeout(resolve, 0));

test("a diagram is drawn once it shows, in a wrapper of its own in the block's place, with options a diagram cannot change", async () => {
  const { configs, drawn, draw } = drawer();
  const { watched, watch, show } = watcher();
  const { article, blocks } = view("graph TD; a1-->b1");

  diagrams(async () => draw, watch)(article, context("dark"));
  await settled();
  expect(drawn).toEqual([]);
  expect([...watched.keys()]).toEqual(blocks);

  show(blocks[0] as Element);
  await settled();

  const wrapper = article.firstElementChild;
  expect(wrapper?.className).toBe("nw-scroll nw-diagram");
  expect(wrapper?.querySelector("svg text")?.textContent).toBe("graph TD; a1-->b1");
  expect(article.querySelectorAll("code.language-mermaid")).toHaveLength(0);
  expect(article.querySelector("code.language-go")).not.toBeNull();
  expect(drawn[0]).toMatch(/^nw_mermaid_\d+ graph TD; a1-->b1$/);
  expect(configs).toEqual([
    {
      startOnLoad: false,
      securityLevel: "strict",
      suppressErrorRendering: true,
      maxTextSize: diagramLimit,
      maxEdges: edgeLimit,
      secure: ["secure", "securityLevel", "startOnLoad", "maxTextSize", "suppressErrorRendering", "maxEdges", "layout"],
      theme: "dark",
    },
  ]);
});

test("the diagrams are drawn one at a time, each with ids of its own", async () => {
  const { drawn, held, draw } = drawer();
  const { watch, show } = watcher();
  const { article, blocks } = view("held a2", "b2");
  diagrams(async () => draw, watch)(article, context());

  show(blocks[0] as Element);
  show(blocks[1] as Element);
  await settled();
  expect(drawn).toHaveLength(1);

  held[0]?.();
  await settled();
  expect(drawn).toHaveLength(2);
  const [first, second] = drawn.map((each) => each.split(" ")[0]);
  expect(first).not.toBe(second);
  expect(article.querySelectorAll(".nw-diagram")).toHaveLength(2);
});

test("a diagram mermaid cannot draw shows its source; one over the limit is not even watched", async () => {
  const { drawn, draw } = drawer();
  const { watched, watch, show } = watcher();
  const over = `graph TD; ${"中".repeat(Math.ceil(diagramLimit / 3))}`;
  const { article, blocks } = view("bad c3", over);

  diagrams(async () => draw, watch)(article, context());
  show(blocks[0] as Element);
  await settled();

  expect([...watched.keys()]).toEqual([blocks[0]]);
  expect(drawn).toEqual([expect.stringMatching(/ bad c3$/)]);
  expect(article.querySelectorAll("code.language-mermaid")).toHaveLength(2);
  expect(article.querySelector(".nw-diagram")).toBeNull();
});

test("a view read again puts a drawing of the same theme and source at once; another theme draws it again", async () => {
  const { drawn, draw } = drawer();
  const { watch, show } = watcher();
  const first = view("d4");
  diagrams(async () => draw, watch)(first.article, context());
  show(first.blocks[0] as Element);
  await settled();

  const again = view("d4");
  diagrams(async () => draw, watch)(again.article, context());
  expect(again.article.querySelector(".nw-diagram svg text")?.textContent).toBe("d4");

  const dark = view("d4");
  diagrams(async () => draw, watch)(dark.article, context("dark"));
  show(dark.blocks[0] as Element);
  await settled();
  expect(drawn.map((each) => each.split(" ")[1])).toEqual(["d4", "d4"]);
});

test("the latest drawings are kept, the one used longest ago dropped first", async () => {
  const { drawn, draw } = drawer();
  const { watch, show } = watcher();
  const sources = Array.from({ length: keptDrawings + 1 }, (_, i) => `e5 ${i}`);
  const all = view(...sources);
  diagrams(async () => draw, watch)(all.article, context());
  for (const block of all.blocks) {
    show(block);
  }
  await settled();
  expect(drawn).toHaveLength(keptDrawings + 1);

  // e5 0 went first; e5 1 is used again, and stays.
  const again = view("e5 1", "e5 0");
  diagrams(async () => draw, watch)(again.article, context());
  expect(again.article.querySelectorAll(".nw-diagram")).toHaveLength(1);
  show(again.blocks[1] as Element);
  await settled();
  expect(drawn.slice(keptDrawings + 1).map((each) => each.split(" ").slice(1).join(" "))).toEqual(["e5 0"]);
  const third = view("e5 1");
  diagrams(async () => draw, watch)(third.article, context());
  expect(third.article.querySelectorAll(".nw-diagram")).toHaveLength(1);
});

test("undone, the blocks are back, the watching stops, and a drawing out is not put", async () => {
  const { held, draw } = drawer();
  const { stopped, watch, show } = watcher();
  const { article, blocks } = view("f6", "held f6");
  const undo = diagrams(async () => draw, watch)(article, context());
  show(blocks[0] as Element);
  await settled();
  show(blocks[1] as Element);
  await settled();

  undo?.();
  held[0]?.();
  await settled();

  expect([...article.querySelectorAll("pre")].slice(0, 2)).toEqual(blocks);
  expect(article.querySelector(".nw-diagram")).toBeNull();
  expect(stopped).toEqual(blocks);
});

test("mermaid that cannot be loaded leaves the source, says so on the console, and the next diagram is still drawn", async () => {
  const error = vi.spyOn(console, "error").mockImplementation(() => undefined);
  const { draw } = drawer();
  const { watch, show } = watcher();
  let loads = 0;
  const load = async () => {
    loads += 1;
    if (loads === 1) {
      throw new Error("offline");
    }
    return draw;
  };
  const { article, blocks } = view("g7", "h8");
  diagrams(load, watch)(article, context());

  show(blocks[0] as Element);
  await settled();
  show(blocks[1] as Element);
  await settled();

  expect(article.querySelector("code.language-mermaid")?.textContent).toBe("g7");
  expect(article.querySelector(".nw-diagram svg text")?.textContent).toBe("h8");
  expect(error).toHaveBeenCalledTimes(1);
});

test("a view without diagrams loads nothing", () => {
  const load = vi.fn(async () => drawer().draw);
  const { article } = view();
  expect(diagrams(load, watcher().watch)(article, context())).toBeUndefined();
  expect(load).not.toHaveBeenCalled();
});
