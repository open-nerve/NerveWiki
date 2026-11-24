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
    t: translator("en"),
    locale: "en",
    theme: () => theme,
    onThemeChange: (listener) => {
      following.add(listener);
      return () => following.delete(listener);
    },
    reload: () => undefined,
    assetsExpire: null,
    assetAddress: () => Promise.reject(new Error("no attachments")),
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

/** The diagrams' backgrounds, the light theme's and the dark's: mermaid's base theme either way, in the app's colours. */
const lightBackground = "#ffffff";
const darkBackground = "#1f2023";

/** backgroundOf is the background a drawing's options give mermaid's theme: what tells the two themes apart. */
const backgroundOf = (config: MermaidConfig) =>
  (config.themeVariables as { background?: string } | undefined)?.background;

/** A text's directive, whose config is JSON here, as mermaid reads it; and a theme its frontmatter's config names. */
const directiveOf = /%%\{init:\s*(\{.*\})\s*\}%%/;
const frontmatterTheme = /^\s+theme:\s*([\w-]+)$/m;
/** A text's frontmatter, which mermaid's detectType reads past, as it does a directive. */
const frontmatterOf = /^\s*---\n[\s\S]*?\n---\n/;

/**
 * drawer is a mermaid that draws <svg id> of its text and its theme (its
 * background: mermaid's base theme in either, M13/P4/S2), its styles and an
 * element of it named by its id, recording the options it was given, the texts
 * it parsed and each drawing; it cannot draw or parse a text with "bad". A
 * drawing of a text with "held" waits for release.
 */
function drawer() {
  const configs: MermaidConfig[] = [];
  const drawn: string[] = [];
  const parsed: string[] = [];
  const held: (() => void)[] = [];
  const draw: Drawer = {
    initialize: (config) => configs.push(config),
    detectType: (text) =>
      text.replace(frontmatterOf, "").replace(directiveOf, "").trimStart().startsWith("mindmap")
        ? "mindmap"
        : "flowchart",
    parse: async (text) => {
      parsed.push(text);
      if (text.includes("bad")) {
        return false;
      }
      const directive = directiveOf.exec(text)?.[1];
      if (directive !== undefined) {
        return { diagramType: "flowchart", config: JSON.parse(directive) as MermaidConfig };
      }
      const theme = frontmatterTheme.exec(text)?.[1] as MermaidConfig["theme"];
      return { diagramType: "flowchart", config: theme === undefined ? {} : { theme } };
    },
    render: async (id, text) => {
      drawn.push(`${id} ${text}`);
      if (text.includes("held")) {
        await new Promise<void>((resolve) => held.push(resolve));
      }
      if (text.includes("bad")) {
        throw new Error("Parse error");
      }
      const theme = (configs.at(-1)?.themeVariables as { background?: string } | undefined)?.background ?? "";
      return {
        svg: `<svg id="${id}"><style>#${id} text{}</style><g id="${id}-a"><text>${text}</text><desc>${theme}</desc></g></svg>`,
      };
    },
  };
  return { configs, drawn, parsed, held, draw };
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
      dompurifyConfig: {
        FORBID_TAGS: ["style", "form", "input", "button", "select", "option", "textarea"],
        FORBID_ATTR: ["id"],
        ALLOW_DATA_ATTR: false,
      },
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
      flowchart: { padding: 9 },
      theme: "base",
      themeVariables: expect.objectContaining({ background: darkBackground }),
      themeCSS: expect.any(String),
    },
  ]);
});

/** optionsIn are the options a diagram is drawn with in theme. */
async function optionsIn(theme: "light" | "dark", source: string): Promise<MermaidConfig> {
  const { configs, draw } = drawer();
  const { watch, show } = watcher();
  const { blocks, article } = view(source);
  diagrams(async () => draw, watch)(article, context(theme));
  show(blocks[0] as Element);
  await settled();
  expect(configs).toHaveLength(1);
  return configs[0] as MermaidConfig;
}

/** drawnWith are the options a diagram of source was last drawn with in theme, and the texts parsed. */
async function drawnWith(theme: "light" | "dark", source: string) {
  const { configs, parsed, draw } = drawer();
  const { watch, show } = watcher();
  const { blocks, article } = view(source);
  diagrams(async () => draw, watch)(article, context(theme));
  show(blocks[0] as Element);
  await settled();
  return { config: configs.at(-1) as MermaidConfig, parsed, drawn: article.querySelector("svg") !== null };
}

// Decision 904 (review A's I2, revised after the fix): a diagram that names a theme of its own, base's aside, or gives
// variables of its own, is drawn from its own config alone, as before M13: the app's variables, overriding the theme
// it derives its own from, would put the app's text on its fills (its light text on an author's yellow: 1.24:1).
// Decision 906 (review A's F1-M1): variables without a theme, in mermaid's dark theme or its default as the app's is.
test("a diagram naming a theme of its own, in a directive or its frontmatter, is drawn in that theme alone, not in the app's colours", async () => {
  const directive = await drawnWith("light", '%%{init: {"theme": "dark"}}%%\ngraph TD; own904-->b');
  const frontmatter = await drawnWith("dark", "---\nconfig:\n  theme: forest\n---\ngraph TD; own904-->c");
  for (const [shown, named] of [
    [directive, "dark"],
    [frontmatter, "forest"],
  ] as const) {
    expect(shown.drawn, named).toBe(true);
    expect(shown.config.theme, named).toBe(named);
    expect(shown.config.themeVariables, named).toBeUndefined();
    expect(shown.config.themeCSS, named).toBeUndefined();
    // What a diagram cannot change, it cannot still.
    expect(shown.config.securityLevel, named).toBe("strict");
    expect(shown.config.secure, named).toContain("dompurifyConfig");
    expect(shown.config.maxEdges, named).toBe(edgeLimit);
  }
});

test("a diagram naming base with variables of its own is drawn from its config alone; base alone, or nothing, in the app's colours; a mindmap too large is not read, its frontmatter's theme neither", async () => {
  const varied = await drawnWith(
    "dark",
    '%%{init: {"theme": "base", "themeVariables": {"primaryColor": "#ff0000"}}}%%\ngraph TD; base904-->b'
  );
  const based = await drawnWith("dark", '%%{init: {"theme": "base"}}%%\ngraph TD; base904-->c');
  const plain = await drawnWith("light", "graph TD; plain904-->b");
  expect([varied.drawn, varied.config.theme, varied.config.themeVariables, varied.config.themeCSS]).toEqual([
    true,
    "base",
    undefined,
    undefined,
  ]);
  expect([based.config.theme, backgroundOf(based.config)]).toEqual(["base", darkBackground]);
  expect([plain.config.theme, backgroundOf(plain.config)]).toEqual(["base", lightBackground]);
  const large = await drawnWith(
    "light",
    [
      "---",
      "config:",
      "  theme: forest",
      "---",
      "mindmap",
      "  root",
      ...Array.from({ length: mindmapLines }, (_, i) => `    big904n${i.toString()}`),
    ].join("\n")
  );
  expect([large.drawn, large.parsed]).toEqual([false, []]);
});

test("a diagram giving variables of its own without a theme is drawn from its config alone, in mermaid's default theme in the light app and its dark one in the dark", async () => {
  const source =
    '%%{init: {"themeVariables": {"fontSize": "20px", "primaryColor": "#ffcc00"}}}%%\ngraph TD; vars906-->b';
  const light = await drawnWith("light", source);
  const dark = await drawnWith("dark", source);
  for (const [shown, named] of [
    [light, "default"],
    [dark, "dark"],
  ] as const) {
    expect(shown.drawn, named).toBe(true);
    expect([shown.config.theme, shown.config.themeVariables, shown.config.themeCSS], named).toEqual([
      named,
      undefined,
      undefined,
    ]);
    expect(shown.config.securityLevel, named).toBe("strict");
  }
});

// Review A's F1-N1: a diagram's own config is in a directive (%%{…}%%) or its frontmatter (--- first); one of neither
// is drawn in the app's colours without mermaid reading it once more before it draws it.
test("a diagram of neither a directive nor frontmatter is not read before it is drawn; one of frontmatter after blank lines is", async () => {
  const plain = await drawnWith("light", "graph TD; plain906-->b");
  const commented = await drawnWith("dark", "%% a comment, no directive\ngraph TD; plain906-->c");
  const spaced = await drawnWith("dark", "  \n---\nconfig:\n  theme: forest\n---\ngraph TD; plain906-->d");
  expect([plain.drawn, plain.parsed, plain.config.theme]).toEqual([true, [], "base"]);
  expect([commented.drawn, commented.parsed, commented.config.theme]).toEqual([true, [], "base"]);
  expect([spaced.drawn, spaced.parsed.length, spaced.config.theme]).toEqual([true, 1, "forest"]);
});

/** The callouts' blue, the light theme's and the dark's (00-M13 4.1; reading.css's --nw-callout-blue). */
const blue = { light: "#3d6fb8", dark: "#8fb0ec" };

// M13/P4/S2, decisions 871, 888: mermaid's base theme in the app's colours, grey nodes and lines; a series in the
// callouts' families, which base cannot derive from grey; dark derived as dark; what the variables cannot say (a
// node's corners, a store's amber, the gantt's grid and the timeline's line) in the theme's own CSS.
test("a diagram is drawn in mermaid's base theme in the app's colours, its series the callouts' families, dark as dark", async () => {
  const shown = {
    light: await optionsIn("light", "graph TD; light888-->b"),
    dark: await optionsIn("dark", "graph TD; dark888-->b"),
  };
  for (const theme of ["light", "dark"] as const) {
    const { theme: named, themeVariables, themeCSS } = shown[theme];
    const variables = (themeVariables ?? {}) as Record<string, unknown>;
    const xyChart = variables.xyChart as Record<string, unknown> | undefined;
    expect(named, theme).toBe("base");
    expect(variables.background, theme).toBe(theme === "light" ? lightBackground : darkBackground);
    expect([variables.pie1, variables.git0, variables.cScale0], theme).toEqual([blue[theme], blue[theme], blue[theme]]);
    // An object mermaid takes whole: its palette alone given, the rest of it is lost (dark drawn on white).
    expect(xyChart?.backgroundColor, theme).toBe(variables.background);
    expect(String(xyChart?.plotColorPalette).split(",")[0], theme).toBe(blue[theme]);
    expect(themeCSS, theme).toContain("outer-path");
    expect(themeCSS, theme).toContain("rx: 9px");
    expect(themeCSS, theme).toContain(".eventWrapper { filter: none; }");
  }
  const light = (shown.light.themeVariables ?? {}) as Record<string, unknown>;
  const dark = (shown.dark.themeVariables ?? {}) as Record<string, unknown>;
  expect([light.darkMode, dark.darkMode]).toEqual([undefined, true]);
  // The gantt's task in progress seen in light, the critical one's text read in dark (review B's R1-M2); the kanban's
  // columns' names read in dark (R1-M3).
  expect([light.activeTaskBkgColor, light.activeTaskBorderColor]).toEqual(["#e7e7ea", "#91939b"]);
  expect(dark.critBkgColor).toBe("#b33b3b");
  expect(shown.dark.themeCSS).toContain(".cluster-label");
  expect(shown.light.themeCSS).not.toContain(".cluster-label");
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

/** separated is a mindmap of mindmapLines nodes past its root, each after end, ended by a comment: one line of "\n". */
function separated(end: string): string {
  return ["mindmap\n  r", ...Array.from({ length: mindmapLines }, (_, i) => `${end}  a${i}[x]%%`)].join("");
}

test("a mindmap's lines are counted as a comment ends one: at a line or paragraph separator, a carriage return too", async () => {
  const { drawn, draw } = drawer();
  const { watch, show } = watcher();
  const { article, blocks } = view(separated("\u2028"), separated("\u2029"), separated("\r"));
  diagrams(async () => draw, watch)(article, context());
  for (const block of blocks) {
    show(block as Element);
  }
  await settled();

  expect(drawn).toEqual([]);
  expect(article.querySelectorAll("code.language-mermaid")).toHaveLength(3);
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
  expect(themeShown()).toBe(lightBackground);

  change("dark");
  await settled();
  expect(drawn).toHaveLength(1);
  expect(themeShown()).toBe(lightBackground);
  expect(watched.has(wrapper as Element)).toBe(true);

  show(wrapper as Element);
  await settled();
  expect(drawn).toHaveLength(2);
  expect(article.querySelector(".nw-diagram")).toBe(wrapper);
  expect(themeShown()).toBe(darkBackground);

  // Kept in the theme it changes back to: put at once.
  change("light");
  expect(themeShown()).toBe(lightBackground);
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
  expect(configs.map((config) => [config.theme, backgroundOf(config)])).toEqual([
    ["base", lightBackground],
    ["base", darkBackground],
  ]);
  const wrapper = article.querySelector(".nw-diagram");
  expect(wrapper?.querySelector("desc")?.textContent).toBe(darkBackground);

  change("light");
  expect(wrapper?.querySelector("desc")?.textContent).toBe(lightBackground);
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

// M12/P2/S4, 11.1: a sequence diagram's or a sankey's ids are not its drawing's (actor0, linearGradient-1); two of them
// on a page would have them in common, a gradient filled from the other's. Its accessible name, which mermaid names by
// its id, stays its own (review A's M9); an IDREF to one renamed follows it (revision check A's M13).
test("two drawings whose elements' ids are not the drawing's own have no id in common, and each names its own", async () => {
  const draw: Drawer = {
    initialize: () => undefined,
    detectType: () => "sequence",
    parse: async () => ({ config: {} }),
    render: async (id, text) => ({
      svg:
        `<svg id="${id}" aria-labelledby="chart-title-${id}"><title id="chart-title-${id}">${text}</title>` +
        `<style>#actor0{fill:red}#actor0-label{fill:blue}</style><defs><linearGradient id="linearGradient-1"/></defs>` +
        `<rect id="actor0" fill="url(#linearGradient-1)"/><use href="#actor0"/><use xlink:href="#actor0"/>` +
        `<text id="actor0-label">${text}</text><g aria-labelledby="actor0-label" aria-describedby="actor0 none"/></svg>`,
    }),
  };
  const { watch, show } = watcher();
  const { article, blocks } = view("sequenceDiagram; A->>B: one", "sequenceDiagram; A->>B: two");
  diagrams(async () => draw, watch)(article, context());
  show(blocks[0] as Element);
  show(blocks[1] as Element);
  await settled();

  const svgs = [...article.querySelectorAll<SVGSVGElement>(".nw-diagram svg")];
  expect(svgs).toHaveLength(2);
  const ids = [...article.querySelectorAll("[id]")].map((element) => element.id);
  expect(new Set(ids).size).toBe(ids.length);
  for (const svg of svgs) {
    const own = (reference: string | null | undefined) => {
      const target = reference?.replace(/^url\(#(.*)\)$/, "$1").replace(/^#/, "") ?? "";
      return target !== "" && svg.querySelector(`[id="${CSS.escape(target)}"]`) !== null;
    };
    const rect = svg.querySelector("rect");
    const [use, xlinked] = [...svg.querySelectorAll("use")];
    const group = svg.querySelector("g");
    expect([
      own(rect?.getAttribute("fill")),
      own(use?.getAttribute("href")),
      own(xlinked?.getAttribute("xlink:href")),
      own(group?.getAttribute("aria-labelledby")),
      own(group?.getAttribute("aria-describedby")?.split(" ")[0]),
      own(svg.getAttribute("aria-labelledby")),
    ]).toEqual([true, true, true, true, true, true]);
    expect(group?.getAttribute("aria-describedby")?.split(" ")[1]).toBe("none");
    // The styles name its own: the rule of the label, not taken for the actor's.
    expect(svg.querySelector("style")?.textContent).toBe(
      `#${rect?.id ?? ""}{fill:red}#${svg.querySelector("text")?.id ?? ""}{fill:blue}`
    );
    expect(svg.querySelector("title")?.textContent).toMatch(/^sequenceDiagram; A->>B: (one|two)$/);
  }
});

/** sankey is a drawing of a sankey's shape, of links links: each a gradient and a path that fills from it, no id the drawing's. */
function sankey(id: string, links: number): string {
  const each = Array.from(
    { length: links },
    (_, k) =>
      `<g class="link"><linearGradient id="linearGradient-${k}"><stop offset="0%"></stop></linearGradient>` +
      `<path d="M${k},0" stroke="url(#linearGradient-${k})"></path></g>`
  );
  return `<svg id="${id}"><style>#${id} .link{stroke-opacity:0.5}</style><g class="links">${each.join("")}</g></svg>`;
}

// Review B's I1 (M12/P2/S4): a drawing is put again each time the view is written. Renamed an id at a time, a sankey of
// a thousand links took a second, read through again for each id; its ids are read once, the renamed looked up.
test("a drawing of thousands of ids not its own is read through a few times, not once for each id", async () => {
  const links = 2000;
  const draw: Drawer = {
    initialize: () => undefined,
    detectType: () => "sankey",
    parse: async () => ({ config: {} }),
    render: async (id) => ({ svg: sankey(id, links) }),
  };
  const { watch, show } = watcher();
  const { article, blocks } = view("sankey-beta\na,b,1");
  diagrams(async () => draw, watch)(article, context());
  const length = sankey("nw_mermaid_0", links).length;
  const replace = vi.spyOn(String.prototype, "replace");
  try {
    show(blocks[0] as Element);
    await settled();
    // A scan is a replace of the drawing whole: once for its own id, then once for each kind of name.
    const scans = replace.mock.contexts.filter((text) => String(text).length > length / 2).length;
    expect(scans).toBeLessThanOrEqual(10);
  } finally {
    replace.mockRestore();
  }
  const svg = article.querySelector<SVGSVGElement>(".nw-diagram svg");
  const path = svg?.querySelector(`path[stroke="url(#${svg.id}-linearGradient-${(links - 1).toString()})"]`);
  expect(path).not.toBeNull();
  expect(svg?.querySelectorAll("linearGradient")).toHaveLength(links);
});

// Review B's unverified suspicion (M12/P2/S4): a gantt's task has the writer's id, unprefixed; mermaid's styles have
// colours, #fff5ad among them. A style's selectors name elements; its declarations do not.
test("a style's declarations name no element: a task whose id is a colour's leaves the colour, its selector renamed", async () => {
  const draw: Drawer = {
    initialize: () => undefined,
    detectType: () => "gantt",
    parse: async () => ({ config: {} }),
    render: async (id) => ({
      svg:
        `<svg id="${id}"><style>#${id} .task{fill:#fff5ad;stroke:#fff5ad}#fff5ad{stroke-width:2px}` +
        `@media (prefers-contrast:more){#fff5ad{fill:#fff5ad}}</style><rect id="fff5ad" class="task"></rect></svg>`,
    }),
  };
  const { watch, show } = watcher();
  const { article, blocks } = view("gantt\ntask :fff5ad, 2026-01-01, 1d");
  diagrams(async () => draw, watch)(article, context());
  show(blocks[0] as Element);
  await settled();

  const svg = article.querySelector<SVGSVGElement>(".nw-diagram svg");
  const rect = svg?.querySelector("rect");
  expect(rect?.id).toBe(`${svg?.id ?? ""}-fff5ad`);
  expect(svg?.querySelector("style")?.textContent).toBe(
    `#${svg?.id ?? ""} .task{fill:#fff5ad;stroke:#fff5ad}#${rect?.id ?? ""}{stroke-width:2px}` +
      `@media (prefers-contrast:more){#${rect?.id ?? ""}{fill:#fff5ad}}`
  );
});
