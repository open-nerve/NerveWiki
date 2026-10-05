import type { MermaidConfig } from "mermaid";

import type { Enhancement } from "./enhancement";

/** The longest diagram drawn, in bytes of UTF-8: a longer one shows its source (M6/P6 design 10). */
export const diagramLimit = 20_000;

/** The most edges a diagram may have: mermaid draws on the page's thread, 450 edges in more than a second. */
export const edgeLimit = 200;

/** How many drawings are kept, by theme and source, for a view read again. */
export const keptDrawings = 50;

/** What drawing uses of mermaid; the tests give their own. */
export type Drawer = {
  initialize: (config: MermaidConfig) => void;
  render: (id: string, text: string) => Promise<{ svg: string }>;
};

/** loadMermaid loads mermaid, in chunks of its own, once a view has a diagram to draw. */
export async function loadMermaid(): Promise<Drawer> {
  return (await import("mermaid")).default;
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
 * script of the diagram's run; an error not drawn but thrown, so that the
 * diagram shows its source; the text and the edges bounded; and none of
 * these, nor the layout (which would load another engine), changed by a
 * diagram's own directives.
 */
function options(theme: "light" | "dark"): MermaidConfig {
  return {
    startOnLoad: false,
    securityLevel: "strict",
    suppressErrorRendering: true,
    maxTextSize: diagramLimit,
    maxEdges: edgeLimit,
    secure: ["secure", "securityLevel", "startOnLoad", "maxTextSize", "suppressErrorRendering", "maxEdges", "layout"],
    theme: theme === "dark" ? "dark" : "default",
  };
}

/** The drawings kept, by theme and source, the one used last last. */
const kept = new Map<string, string>();

/** The drawings out, one at a time across the views: mermaid keeps its state in the page. */
let drawing: Promise<void> = Promise.resolve();

/** How many diagrams were drawn: the ids of each drawing's elements are its own (nw_mermaid_n, no heading's id). */
let drawn = 0;

const encoder = new TextEncoder();

/**
 * diagrams draws the reading view's mermaid code blocks (```mermaid) with
 * mermaid, which load loads (M6/P6 design 10): each once it shows (watch),
 * one at a time, in the view's theme. A drawing takes the block's place in
 * a wrapper of its own (nw-scroll nw-diagram), which scrolls sideways; a
 * diagram over diagramLimit, or one mermaid cannot draw, shows its source.
 * The latest keptDrawings drawings are kept, by theme and source: a view
 * read again, its HTML replaced whole, puts a diagram whose source did not
 * change at once. Undone, the blocks are back, and what is out is dropped.
 */
export function diagrams(load: () => Promise<Drawer>, watch: Watch = whenShown): Enhancement {
  return (container, { theme }) => {
    const blocks = [...container.querySelectorAll<HTMLElement>("pre > code.language-mermaid")];
    if (blocks.length === 0) {
      return undefined;
    }
    let undone = false;
    const placed: { block: Element; wrapper: Element }[] = [];
    const watching: (() => void)[] = [];
    const place = (block: Element, svg: string) => {
      const wrapper = document.createElement("div");
      wrapper.className = "nw-scroll nw-diagram";
      // mermaid's own SVG, strict, its labels sanitized: the exception to adding no markup from elsewhere (M6 design
      // 4.9). The pages' CSP runs no inline script either: no handler, no javascript: address.
      wrapper.innerHTML = svg;
      block.replaceWith(wrapper);
      placed.push({ block, wrapper });
    };
    for (const code of blocks) {
      const block = code.parentElement;
      const source = code.textContent;
      const key = `${theme}\n${source}`;
      const keptDrawing = kept.get(key);
      if (block === null) {
        continue;
      }
      if (keptDrawing !== undefined) {
        keep(key, keptDrawing);
        place(block, keptDrawing);
        continue;
      }
      if (encoder.encode(source).length > diagramLimit) {
        continue;
      }
      watching.push(
        watch(block, () => {
          draw(async () => {
            if (undone) {
              return;
            }
            const svg = await render(load, theme, source);
            if (svg !== undefined && !undone) {
              keep(key, svg);
              place(block, svg);
            }
          });
        })
      );
    }
    return () => {
      undone = true;
      for (const stop of watching) {
        stop();
      }
      for (const { block, wrapper } of placed) {
        wrapper.replaceWith(block);
      }
    };
  };
}

/** draw runs task after the drawings out, whichever way they end. */
function draw(task: () => Promise<void>) {
  drawing = drawing.then(task).catch(() => undefined);
}

/** render is mermaid's drawing of source in theme, or undefined: mermaid could not load, or not draw it. */
async function render(
  load: () => Promise<Drawer>,
  theme: "light" | "dark",
  source: string
): Promise<string | undefined> {
  let mermaid: Drawer;
  try {
    mermaid = await load();
  } catch (error) {
    console.error("mermaid could not be loaded: the diagrams show their source", error);
    return undefined;
  }
  mermaid.initialize(options(theme));
  try {
    return (await mermaid.render(`nw_mermaid_${(++drawn).toString()}`, source)).svg;
  } catch {
    // A diagram mermaid cannot read, or past its bounds: its source shows.
    return undefined;
  }
}

/** keep keeps the drawing of key as the one used last, the oldest dropped past keptDrawings. */
function keep(key: string, svg: string) {
  kept.delete(key);
  kept.set(key, svg);
  for (const oldest of kept.keys()) {
    if (kept.size <= keptDrawings) {
      break;
    }
    kept.delete(oldest);
  }
}
