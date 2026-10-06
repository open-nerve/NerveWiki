import type { MermaidConfig } from "mermaid";
import { afterEach, expect, test, vi } from "vitest";

import { translator } from "../i18n/i18n";
import { diagramLimit, diagrams, edgeLimit, keptDrawings, mindmapLines, type Drawer, type Watch } from "./diagrams";
import type { ReadingContext } from "./enhancement";

// The diagrams, drawn by mermaid (M6/P6 design 10). The drawings kept are
// the module's, across the tests: each test draws sources of its own.

afterEach(() => document.body.replaceChildren());

/** themed is a view's context in theme, and a way to change the theme shown, telling those who follow it. */
function themed(initial: "light" | "dark" = "light") {
  let theme = initial;
  const following = new Set<() => void>();
  const context: ReadingContext = {
    workspace: "lab",
    notebook: "n",
    page: "p",
    revision: 1,
    role: "reader",
    t: translator("en"),
    theme: () => theme,
    onThemeChange: (listener) => {
      following.add(listener);
      return () => following.delete(listener);
    },
    reload: () => undefined,
    navigate: () => undefined,
    report: () => undefined,
    unresolved: () => undefined,
  };
  const change = (to: "light" | "dark") => {
    theme = to;
    for (const listener of following) {
      listener();
    }
  };
  return { context, following, change };
}

const context = (theme: "light" | "dark" = "light") => themed(theme).context;

/**
 * drawer is a mermaid that draws <svg id> of its text and its theme, its
 * styles and an element of it named by its id, recording the options it
 * was given and each drawing; it cannot draw a text with "bad". A drawing
 * of a text with "held" waits for release.
 */
function drawer() {
  const configs: MermaidConfig[] = [];
  const drawn: string[] = [];
  const held: (() => void)[] = [];
  const draw: Drawer = {
    initialize: (config) => configs.push(config),
    detectType: (text) => (text.startsWith("mindmap") ? "mindmap" : "flowchart"),
    render: async (id, text) => {
      drawn.push(`${id} ${text}`);
      if (text.includes("held")) {
        await new Promise<void>((resolve) => held.push(resolve));
      }
      if (text.includes("bad")) {
        throw new Error("Parse error");
      }
      const theme = configs.at(-1)?.theme ?? "";
      return {
        svg: `<svg id="${id}"><style>#${id} text{}</style><g id="${id}-a"><text>${text}</text><desc>${theme}</desc></g></svg>`,
      };
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
      dompurifyConfig: { FORBID_TAGS: ["style"], FORBID_ATTR: ["id"], ALLOW_DATA_ATTR: false },
      secure: [
        "secure",
        "securityLevel",
        "startOnLoad",
        "maxTextSize",
        "suppressErrorRendering",
        "maxEdges",
        "dompurifyConfig",
        "layout",
      ],
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

test("each drawing put has ids of its own, and its styles and elements follow: two diagrams of a source are drawn once", async () => {
  const { drawn, draw } = drawer();
  const { watch, show } = watcher();
  const { article, blocks } = view("o14", "o14");
  diagrams(async () => draw, watch)(article, context());
  show(blocks[0] as Element);
  show(blocks[1] as Element);
  await settled();
  const again = view("o14");
  diagrams(async () => draw, watch)(again.article, context());

  expect(drawn).toHaveLength(1);
  const svgs = [...document.querySelectorAll(".nw-diagram svg")];
  const ids = svgs.map((svg) => svg.id);
  expect(new Set(ids).size).toBe(3);
  for (const svg of svgs) {
    expect([svg.querySelector("g")?.id, svg.querySelector("style")?.textContent]).toEqual([
      `${svg.id}-a`,
      `#${svg.id} text{}`,
    ]);
  }
});

/** mindmap is a mindmap's source of lines lines. */
function mindmap(lines: number): string {
  return ["mindmap", ...Array.from({ length: lines - 1 }, (_, i) => `  q${i}`)].join("\n");
}

test("a mindmap of more lines than mindmapLines shows its source: mermaid's layout of it grows faster than its nodes", async () => {
  const { drawn, draw } = drawer();
  const { watch, show } = watcher();
  const { article, blocks } = view(mindmap(mindmapLines), mindmap(mindmapLines + 1));
  diagrams(async () => draw, watch)(article, context());
  show(blocks[0] as Element);
  show(blocks[1] as Element);
  await settled();

  expect(drawn).toHaveLength(1);
  expect(article.querySelectorAll(".nw-diagram")).toHaveLength(1);
  expect(article.querySelectorAll("code.language-mermaid")).toHaveLength(1);
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

test("undone, the blocks are back, the watching and the theme's following stop, and a drawing out is kept, not put", async () => {
  const { held, draw } = drawer();
  const { stopped, watch, show } = watcher();
  const { context: undoneContext, following } = themed();
  const { article, blocks } = view("f6", "held f6", "f6 waits");
  const undo = diagrams(async () => draw, watch)(article, undoneContext);
  show(blocks[0] as Element);
  await settled();
  show(blocks[1] as Element);
  await settled();
  expect(following.size).toBe(1);

  undo?.();
  held[0]?.();
  await settled();

  expect([...article.querySelectorAll("pre")].slice(0, 3)).toEqual(blocks);
  expect(article.querySelector(".nw-diagram")).toBeNull();
  expect(stopped).toEqual([blocks[2]]);
  expect(following.size).toBe(0);
  // The drawing out is kept: the view read again puts it at once.
  const again = view("held f6");
  diagrams(async () => draw, watch)(again.article, context());
  expect(again.article.querySelector(".nw-diagram svg text")?.textContent).toBe("held f6");
});

test("as the theme changes, a drawing is drawn again in it, in its wrapper, once it shows; until then the old one shows", async () => {
  const { drawn, draw } = drawer();
  const { watched, watch, show } = watcher();
  const { context: shown, change } = themed();
  const { article, blocks } = view("m12");
  diagrams(async () => draw, watch)(article, shown);
  show(blocks[0] as Element);
  await settled();
  const wrapper = article.querySelector(".nw-diagram");
  const themeShown = () => wrapper?.querySelector("desc")?.textContent;
  expect(themeShown()).toBe("default");

  change("dark");
  await settled();
  expect(drawn).toHaveLength(1);
  expect(themeShown()).toBe("default");
  expect(watched.has(wrapper as Element)).toBe(true);

  show(wrapper as Element);
  await settled();
  expect(drawn).toHaveLength(2);
  expect(article.querySelector(".nw-diagram")).toBe(wrapper);
  expect(themeShown()).toBe("dark");

  // Kept in the theme it changes back to: put at once.
  change("light");
  expect(themeShown()).toBe("default");
  expect(drawn).toHaveLength(2);
});

test("a diagram is drawn in the theme shown as it is drawn: a drawing out as it changes is kept, not put", async () => {
  const { configs, drawn, held, draw } = drawer();
  const { watch, show } = watcher();
  const { context: shown, change } = themed();
  const { article, blocks } = view("held n13");
  diagrams(async () => draw, watch)(article, shown);
  show(blocks[0] as Element);
  await settled();

  change("dark");
  held[0]?.();
  await settled();
  expect(article.querySelector(".nw-diagram")).toBeNull();

  show(blocks[0] as Element);
  await settled();
  held[1]?.();
  await settled();
  expect(configs.map((config) => config.theme)).toEqual(["default", "dark"]);
  const wrapper = article.querySelector(".nw-diagram");
  expect(wrapper?.querySelector("desc")?.textContent).toBe("dark");

  change("light");
  expect(wrapper?.querySelector("desc")?.textContent).toBe("default");
  expect(drawn).toHaveLength(2);
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
