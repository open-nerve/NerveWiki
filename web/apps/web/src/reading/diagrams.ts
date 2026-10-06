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

/**
 * mermaid's options (M6 design 4.9): strict, its labels sanitized and no
 * script of the diagram's run, a label's HTML without a style element
 * (mermaid's own rule), an id, which could take an anchor of the page's,
 * nor a data attribute: the server's marks (data-task, data-nw-…) are the
 * enhancements' to act on, and a drawing kept is put before they run (a
 * label's link loses its target with them, which mermaid keeps in one: it
 * opens where it is, as the page's own do);
 * an error not drawn but thrown, so that the diagram shows its source; the
 * text and the edges bounded (the edges of a flowchart: mermaid counts no
 * other's); and none of these, nor the layout (which would load another
 * engine), changed by a diagram's own directives.
 */
function options(theme: "light" | "dark"): MermaidConfig {
  return {
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
    theme: theme === "dark" ? "dark" : "default",
  };
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
 * withOwnIds is drawing's SVG, the ids its elements' begin with, and what
 * names them (its styles, its markers' addresses), another's: a drawing
 * put twice, two diagrams of a source, or one kept, has no id of another.
 */
function withOwnIds({ id, svg }: Drawing): string {
  return svg.replace(new RegExp(`${id}(?![0-9])`, "g"), nextId());
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
