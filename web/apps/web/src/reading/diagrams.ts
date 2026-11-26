import type { MermaidConfig } from "mermaid";

import type { Enhancement } from "./enhancement";
import { guardLabels } from "./math";

/** The longest diagram drawn, in bytes of UTF-8: a longer one shows its source (M6/P6 design 10). */
export const diagramLimit = 20_000;

/** The most edges a diagram may have: mermaid draws on the page's thread, 450 edges in more than a second. */
export const edgeLimit = 200;

/**
 * The most lines a mindmap may have, a node each: its layout grows faster
 * than its nodes (100 in 0.44 s, 800 in 14.6 s, 2,000 in 116 s, within
 * diagramLimit: M6/P6 B second fix check). A larger one shows its source.
 * Its lines are counted as they are written, comments among them.
 */
export const mindmapLines = 150;

/** How many drawings are kept, by theme and source, for a view read again. */
export const keptDrawings = 50;

/** What drawing uses of mermaid; the tests give their own. */
export type Drawer = {
  initialize: (config: MermaidConfig) => void;
  /** detectType is the type of diagram text is, as mermaid reads it; it throws for none. */
  detectType: (text: string) => string;
  /** parse reads text as mermaid does, its own directives' and frontmatter's config with it; false for what it cannot. */
  parse: (text: string, options: { suppressErrors: true }) => Promise<{ config: MermaidConfig } | false>;
  render: (id: string, text: string) => Promise<{ svg: string }>;
};

/**
 * loadMermaid loads mermaid, in chunks of its own, once a view has a
 * diagram to draw, and KaTeX, which mermaid typesets a label's formulas
 * with, guarded as math's are (guardLabels) before mermaid can use it.
 */
export async function loadMermaid(): Promise<Drawer> {
  const [mermaid, katex] = await Promise.all([import("mermaid"), import("katex")]);
  guardLabels(katex.default);
  return mermaid.default;
}

/** A Watch calls see once element shows, and answers what stops watching it. */
export type Watch = (element: Element, see: () => void) => () => void;

/** whenShown watches an element until it comes near the window's view. */
const whenShown: Watch = (element, see) => {
  const observer = new IntersectionObserver(
    (entries) => {
      if (entries.some((entry) => entry.isIntersecting)) {
        observer.disconnect();
        see();
      }
    },
    { rootMargin: "200px" }
  );
  observer.observe(element);
  return () => observer.disconnect();
};

/** The series' colours, the callouts' seven families (00-M13 4.1): blue, orange, teal, purple, red, green, grey. */
const series = {
  light: ["#3d6fb8", "#b26a12", "#2f8f6a", "#7a52b3", "#d64545", "#3e8a4f", "#6b6e76"],
  dark: ["#8fb0ec", "#e0a656", "#5cc49a", "#c3a6f2", "#ff7b7b", "#7fcf8f", "#9a9ca3"],
} as const;

/**
 * The diagrams' colours, mermaid's base theme in the app's (00-M13 4.1; v0.1 design 13.2 item 32): hexadecimal alone,
 * mermaid takes no other. Grey nodes and lines, the note amber; dark derived as dark (darkMode). A series in the
 * callouts' families, which base cannot derive from grey: a pie, a git graph, a mindmap, a timeline, a kanban, a radar.
 * The gantt's rows, grid, tasks and today line; an edge's label, an ER row, an architecture's lines; the xychart and
 * the radar given whole, as mermaid takes them (M13/P4/S2, decisions 871, 888; review B's I1, R1-M2).
 */
function themeVariables(theme: "light" | "dark"): Record<string, unknown> {
  const light = theme === "light";
  const colours = series[theme];
  const background = light ? "#ffffff" : "#1f2023";
  const text = light ? "#18191c" : "#e8e8ea";
  const line = light ? "#91939b" : "#74767e";
  // Text on a series' colour: white on the light theme's, near black on the dark's lighter shade.
  const onSeries = light ? "#ffffff" : "#18191c";
  const variables: Record<string, unknown> = {
    background,
    primaryColor: light ? "#f4f4f5" : "#25262a",
    primaryBorderColor: light ? "#d4d4d9" : "#3a3b41",
    primaryTextColor: text,
    secondaryColor: light ? "#f4f4f5" : "#25262a",
    tertiaryColor: background,
    lineColor: line,
    textColor: text,
    noteBkgColor: light ? "#f8f1e4" : "#39332a",
    noteBorderColor: light ? "#c2871f" : "#d9a84e",
    noteTextColor: text,
    fontFamily: "inherit",
    fontSize: "13.5px",
  };
  if (!light) {
    variables.darkMode = true;
  }
  // Round the seven families: an eighth slice is blue again.
  for (let i = 1; i <= 12; i++) {
    variables[`pie${i.toString()}`] = colours[(i - 1) % colours.length];
  }
  for (let i = 0; i < 8; i++) {
    variables[`git${i.toString()}`] = colours[i % colours.length];
    variables[`gitInv${i.toString()}`] = colours[i % colours.length];
    variables[`gitBranchLabel${i.toString()}`] = onSeries;
  }
  for (let i = 0; i < 12; i++) {
    variables[`cScale${i.toString()}`] = colours[i % colours.length];
    variables[`cScalePeer${i.toString()}`] = colours[i % colours.length];
    variables[`cScaleLabel${i.toString()}`] = onSeries;
    // A timeline's upright lines: the series' inverse otherwise, which light does not show.
    variables[`cScaleInv${i.toString()}`] = line;
  }
  Object.assign(
    variables,
    light
      ? { pieStrokeColor: "#ffffff", pieOuterStrokeColor: "#d4d4d9", pieSectionTextColor: "#ffffff", pieOpacity: "1" }
      : { pieStrokeColor: "#1f2023", pieOuterStrokeColor: "#3a3b41", pieSectionTextColor: "#18191c", pieOpacity: "1" },
    // The gantt: its rows, grid, done, critical (dark's a shade deeper: its text is light), today's and a marker's line.
    light
      ? {
          altSectionBkgColor: "#ffffff",
          gridColor: "#e7e7ea",
          doneTaskBkgColor: "#d4d4d9",
          critBkgColor: "#d64545",
          todayLineColor: "#c2871f",
          vertLineColor: "#c2871f",
          // A task in progress: base derives white on a light border, which the white chart does not show.
          activeTaskBkgColor: "#e7e7ea",
          activeTaskBorderColor: "#91939b",
        }
      : {
          altSectionBkgColor: "#1f2023",
          gridColor: "#2b2c31",
          doneTaskBkgColor: "#3a3b41",
          critBkgColor: "#b33b3b",
          todayLineColor: "#d9a84e",
          vertLineColor: "#d9a84e",
          excludeBkgColor: "#25262a",
          // An edge's label, near black otherwise; an ER entity's rows, near black under darkMode.
          edgeLabelBackground: "#1f2023",
          rowOdd: "#1f2023",
          rowEven: "#25262a",
        },
    { archEdgeColor: line, archEdgeArrowColor: line, archGroupBorderColor: line }
  );
  variables.xyChart = {
    backgroundColor: background,
    titleColor: text,
    dataLabelColor: text,
    legendTextColor: text,
    xAxisTitleColor: text,
    xAxisLabelColor: text,
    xAxisTickColor: line,
    xAxisLineColor: line,
    yAxisTitleColor: text,
    yAxisLabelColor: text,
    yAxisTickColor: line,
    yAxisLineColor: line,
    plotColorPalette: colours.join(","),
  };
  variables.radar = {
    axisColor: line,
    axisStrokeWidth: 1,
    axisLabelFontSize: 12,
    curveOpacity: 0.5,
    curveStrokeWidth: 2,
    graticuleColor: light ? "#e7e7ea" : "#3a3b41",
    graticuleStrokeWidth: 1,
    graticuleOpacity: light ? 0.3 : 0.5,
    legendBoxSize: 12,
    legendFontSize: 12,
  };
  return variables;
}

/**
 * What the variables cannot say (decision 888): a node's corners, 9px as the mockup's; a store's (a cylinder's)
 * amber, its fill the note's; the gantt's grid lines, which mermaid draws in the page's colour, and the timeline's
 * line, in the series' label colour otherwise; a timeline's event not brightened. Dark, an ER's circle drawn white
 * and a kanban column's name, light on its darkened series. Mermaid scopes it by the drawing's id: it outranks its
 * own rules, not an author's style or classDef.
 */
function themeCSS(theme: "light" | "dark"): string {
  const light = theme === "light";
  return [
    ".node > rect.basic.label-container { rx: 9px; ry: 9px; }",
    `.node > path.basic.label-container.outer-path { fill: ${light ? "#f8f1e4" : "#39332a"}; stroke: ${light ? "#c2871f" : "#d9a84e"}; }`,
    `.grid .tick line { stroke: ${light ? "#e7e7ea" : "#2b2c31"}; }`,
    `.lineWrapper line { stroke: ${light ? "#91939b" : "#74767e"}; }`,
    ".eventWrapper { filter: none; }",
    ...(light
      ? []
      : [
          'marker circle[fill="white"] { fill: #1f2023; }',
          ".sections > .cluster > .cluster-label { color: #18191c; fill: #18191c; }",
        ]),
  ].join("\n");
}

/**
 * mermaid's options (M6 design 4.9): strict, its labels sanitized and no
 * script of the diagram's run, a label's HTML without a style element
 * (mermaid's own rule), a form or its controls, which no label needs and
 * the server takes out of a page's own HTML (a password asked for in a
 * diagram), an id, which could take an anchor of the page's, nor a data
 * attribute: the server's marks (data-task, data-nw-…) are the
 * enhancements' to act on, and a drawing kept is put before they run (a
 * label's link loses its target with them, which mermaid keeps in one: it
 * opens where it is, as the page's own do);
 * an error not drawn but thrown, so that the diagram shows its source; the
 * text and the edges bounded (the edges of a flowchart: mermaid counts no
 * other's); and none of these, nor the layout (which would load another
 * engine), changed by a diagram's own directives. A flowchart's node is
 * reading.html's, 9px about its label (M13/P4/S2). Its theme is mermaid's
 * base in the app's colours (themeVariables, themeCSS: M13/P4/S2), or the
 * diagram's own config's alone, a theme it names (own: decision 904), or,
 * where it names none, mermaid's dark or default as the app's theme is, as
 * before M13 (decision 906).
 */
function options(theme: "light" | "dark", own?: { theme?: MermaidConfig["theme"] }): MermaidConfig {
  return {
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
    // Its colours, which a diagram's own directives may change, as its theme ever could: colours, no trust. A diagram
    // with colours of its own is drawn as mermaid draws it: mermaid derives a theme's from the variables given here,
    // which would put the app's text on its fills (review A's I2, decision 904); its variables without a theme in
    // mermaid's theme of the app's light or dark: its default's #333 lines on the dark frame were 1.29:1 (decision
    // 906, review A's F1-M1).
    ...(own === undefined
      ? { theme: "base", themeVariables: themeVariables(theme), themeCSS: themeCSS(theme) }
      : { theme: own.theme ?? (theme === "dark" ? "dark" : "default") }),
  };
}

/**
 * ownColours are the colours a diagram's own directive or frontmatter's config gives, as mermaid reads them: a theme
 * it names other than base, or variables of its own, base's or none's (then its theme, if any); or none, base alone
 * the app's colours too (decisions 904, 906). A diagram of neither a directive nor frontmatter has none, and is not
 * read: mermaid reads it as it draws it, once is enough (review A's F1-N1: a long sequence's reading, a third of its
 * drawing's time).
 */
async function ownColours(mermaid: Drawer, source: string): Promise<{ theme?: MermaidConfig["theme"] } | undefined> {
  if (!source.includes("%%{") && !source.trimStart().startsWith("---")) {
    return undefined;
  }
  const read = await mermaid.parse(source, { suppressErrors: true });
  if (read === false) {
    return undefined;
  }
  const { theme, themeVariables: variables } = read.config;
  const varied = variables !== undefined && Object.keys(variables).length > 0;
  return varied || (theme !== undefined && theme !== "base") ? { theme } : undefined;
}

/** A drawing: mermaid's SVG, and the id its elements' ids begin with. */
type Drawing = { id: string; svg: string };

/** The drawings kept, by theme and source, the one used last last. */
const drawings = new Map<string, Drawing>();

/** The drawings out, one at a time across the views: mermaid keeps its state in the page. */
let drawingsOut: Promise<void> = Promise.resolve();

/** How many drawings were drawn or put: the ids of each one's elements are its own (nw_mermaid_n, no heading's id). */
let drawn = 0;

const encoder = new TextEncoder();

/** A diagram of the view: its block, its source, and, once drawn, its wrapper and the drawing's key in it. */
type Diagram = {
  block: Element;
  source: string;
  wrapper: HTMLElement | undefined;
  shown: string | undefined;
  /** What stops watching it, while it waits to show. */
  stop: (() => void) | undefined;
};

/**
 * diagrams draws the reading view's mermaid code blocks (```mermaid) with
 * mermaid, which load loads (M6/P6 design 10): each once it shows (watch),
 * one at a time, in the theme shown as it is drawn. A drawing takes the
 * block's place in a wrapper of its own (nw-scroll nw-diagram), which
 * scrolls sideways; a diagram over diagramLimit, a mindmap over
 * mindmapLines, or one mermaid cannot draw (a label's formula refused
 * among them: loadMermaid), shows its source. The latest keptDrawings drawings are kept, by
 * theme and source: a view read again, its HTML replaced whole, puts a
 * diagram whose source did not change at once. As the theme changes, each
 * diagram is drawn again in it, in its wrapper, once it shows: the old
 * drawing shows until then, and the view is not run again (its focus, its
 * scroll). Undone, the blocks are back, and what is out is dropped.
 */
export function diagrams(load: () => Promise<Drawer>, watch: Watch = whenShown): Enhancement {
  return (container, { theme, onThemeChange }) => {
    const found: Diagram[] = [];
    for (const code of container.querySelectorAll<HTMLElement>("pre > code.language-mermaid")) {
      const block = code.parentElement;
      const source = code.textContent;
      if (block !== null && encoder.encode(source).length <= diagramLimit) {
        found.push({ block, source, wrapper: undefined, shown: undefined, stop: undefined });
      }
    }
    if (found.length === 0) {
      return undefined;
    }
    let undone = false;
    const put = (diagram: Diagram, key: string, drawing: Drawing) => {
      if (diagram.shown === key) {
        return;
      }
      const wrapper = diagram.wrapper ?? document.createElement("div");
      wrapper.className = "nw-scroll nw-diagram";
      // mermaid's own SVG, strict, its labels sanitized: the exception to adding no markup from elsewhere (M6 design
      // 4.9). The pages' CSP runs no inline script either: no handler, no javascript: address.
      wrapper.innerHTML = withOwnIds(drawing);
      if (diagram.wrapper === undefined) {
        diagram.block.replaceWith(wrapper);
        diagram.wrapper = wrapper;
      }
      diagram.shown = key;
    };
    // Put at once if kept in the theme shown, or drawn once it shows.
    const show = (diagram: Diagram) => {
      diagram.stop?.();
      diagram.stop = undefined;
      const key = keyOf(theme(), diagram.source);
      const kept = drawings.get(key);
      if (kept !== undefined) {
        keep(key, kept);
        put(diagram, key, kept);
        return;
      }
      diagram.stop = watch(diagram.wrapper ?? diagram.block, () => {
        diagram.stop = undefined;
        draw(async () => {
          // The theme as it is drawn: changed meanwhile, the drawing in it may be kept by now.
          const shown = theme();
          const drawnKey = keyOf(shown, diagram.source);
          if (undone || diagram.shown === drawnKey) {
            return;
          }
          const made = drawings.get(drawnKey) ?? (await render(load, shown, diagram.source));
          if (made === undefined) {
            return;
          }
          keep(drawnKey, made);
          if (!undone && shown === theme()) {
            put(diagram, drawnKey, made);
          }
        });
      });
    };
    for (const diagram of found) {
      show(diagram);
    }
    const stopFollowing = onThemeChange(() => {
      for (const diagram of found) {
        show(diagram);
      }
    });
    return () => {
      undone = true;
      stopFollowing();
      for (const diagram of found) {
        diagram.stop?.();
        diagram.wrapper?.replaceWith(diagram.block);
      }
    };
  };
}

/** keyOf is what a drawing is kept by: its theme and its source. */
function keyOf(theme: "light" | "dark", source: string): string {
  return `${theme}\n${source}`;
}

/** draw runs task after the drawings out, whichever way they end. */
function draw(task: () => Promise<void>) {
  drawingsOut = drawingsOut.then(task).catch(() => undefined);
}

/** render is mermaid's drawing of source in theme, or undefined: mermaid could not load, or not draw it. */
async function render(
  load: () => Promise<Drawer>,
  theme: "light" | "dark",
  source: string
): Promise<Drawing | undefined> {
  let mermaid: Drawer;
  try {
    mermaid = await load();
  } catch (error) {
    console.error("mermaid could not be loaded: the diagrams show their source", error);
    return undefined;
  }
  mermaid.initialize(options(theme));
  if (tooLarge(mermaid, source)) {
    return undefined;
  }
  const id = nextId();
  try {
    // Read once it is known not too large: a mindmap's layout is not all that grows faster than its nodes.
    const own = await ownColours(mermaid, source);
    if (own !== undefined) {
      mermaid.initialize(options(theme, own));
    }
    return { id, svg: (await mermaid.render(id, source)).svg };
  } catch {
    // A diagram mermaid cannot read, or past its bounds: its source shows.
    return undefined;
  }
}

/**
 * A line's end as JavaScript's "." ends a line. A mindmap's comment (%%…),
 * which mermaid reads with ".*", ends a node's line at any of them, and
 * the next node begins after it: nodes a U+2028 apart are on one line of
 * "\n" (1,000 of them in 9 KB took 19 s; M6/P6 B fix check 3).
 */
const lineEnd = /\r\n?|[\n\u2028\u2029]/;

/** tooLarge tells whether source is a mindmap of more than mindmapLines lines, as mermaid tells its type. */
function tooLarge(mermaid: Drawer, source: string): boolean {
  let type: string;
  try {
    type = mermaid.detectType(source);
  } catch {
    // No type: mermaid cannot draw it either.
    return false;
  }
  return type === "mindmap" && source.split(lineEnd).filter((line) => line.trim() !== "").length > mindmapLines;
}

/** nextId is an id no drawing's elements' ids begin with. */
function nextId(): string {
  drawn += 1;
  return `nw_mermaid_${drawn.toString()}`;
}

/**
 * withOwnIds is drawing's SVG, its ids its own: a drawing put twice, two
 * diagrams of a source, or one kept, has no id of another. Those mermaid
 * makes of the drawing's id (its own, its title's and description's, its
 * markers') have it replaced, and what names them with it. Those it does
 * not (a sequence diagram's actor0, a sankey's linearGradient-1: M12/P2/S4,
 * 11.1) take its new id before them, and so does what names them: an
 * address (url(#…), href, xlink:href), its styles' selectors (not their
 * declarations: a gantt task's id may be a colour's, #fff5ad), and the
 * IDREFs that name an element (aria-labelledby, aria-describedby). Only
 * mermaid's markup: the writer's text has no id (its dompurifyConfig).
 * Each kind of name is read once through the SVG, the renamed looked up:
 * a sankey has an id of each link and each node, and a drawing is put
 * again each time the view is written (review B's I1).
 */
function withOwnIds({ id, svg }: Drawing): string {
  const own = nextId();
  const out = svg.replace(new RegExp(`${id}(?![0-9])`, "g"), own);
  const renamed = new Map<string, string>();
  for (const [, other = ""] of out.matchAll(/\sid="([^"]+)"/g)) {
    if (!other.includes(own)) {
      renamed.set(other, `${own}-${other}`);
    }
  }
  if (renamed.size === 0) {
    return out;
  }
  const to = (name: string) => renamed.get(name) ?? name;
  return out
    .replace(
      /(\sid="|\s(?:xlink:)?href="#)([^"]+)(")/g,
      (_, before: string, name: string, after: string) => `${before}${to(name)}${after}`
    )
    .replace(/url\(#([^)]+)\)/g, (_, name: string) => `url(#${to(name)})`)
    .replace(/<style[^>]*>[\s\S]*?<\/style>/g, (style) =>
      // A selector is what comes before a block, after the style's start or another block's end.
      style.replace(
        /(^|[{}])([^{}]*)(?=\{)/g,
        (_, end: string, selector: string) =>
          `${end}${selector.replace(/#([\w-]+)/g, (whole, name: string) => (renamed.has(name) ? `#${to(name)}` : whole))}`
      )
    )
    .replace(
      /(\s(?:aria-labelledby|aria-describedby)=")([^"]*)(")/g,
      (_, before: string, names: string, after: string) => `${before}${names.split(/(\s+)/).map(to).join("")}${after}`
    );
}

/** keep keeps the drawing of key as the one used last, the oldest dropped past keptDrawings. */
function keep(key: string, drawing: Drawing) {
  drawings.delete(key);
  drawings.set(key, drawing);
  for (const oldest of drawings.keys()) {
    if (drawings.size <= keptDrawings) {
      break;
    }
    drawings.delete(oldest);
  }
}
