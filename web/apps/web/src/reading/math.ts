import type { KatexOptions } from "katex";

import type { Enhancement } from "./enhancement";

/** The longest formula typeset, in bytes of UTF-8: a longer one shows its TeX (M6/P6 design 10). */
export const formulaLimit = 4000;

/**
 * The deepest a typeset formula's elements may nest: past about 900,
 * Chromium's layout crashes the page (135 levels of x^{x^…}, in 541
 * bytes). A deeper one shows its TeX (M6/P6 B fix check f2-2).
 */
const formulaDepth = 500;

/** How long, in milliseconds, formulas are typeset in one task before the page's thread is given back. */
export const taskTime = 50;

/** What typesetting uses of KaTeX; the tests give their own. */
export type Typesetter = { render: (tex: string, element: HTMLElement, options: KatexOptions) => void };

/**
 * KaTeX's options (M6 design 4.9): no command that loads, links or styles
 * from the TeX (trust false), sizes and macro expansions bounded, and an
 * error thrown, never rendered, so that the formula shows its TeX instead.
 * What strict mode would warn of is let be: the console stays quiet.
 */
const options: KatexOptions = { trust: false, maxSize: 50, maxExpand: 1000, throwOnError: true, strict: false };

/** loadKatex loads KaTeX and its stylesheet, with its fonts, in chunks of their own, once a view has a formula. */
export async function loadKatex(): Promise<Typesetter> {
  const [katex] = await Promise.all([import("katex"), import("katex/dist/katex.min.css")]);
  return katex.default;
}

const encoder = new TextEncoder();

/** The control words that define a macro in the TeX KaTeX reads. */
const defining = new Set([
  "def",
  "gdef",
  "edef",
  "xdef",
  "let",
  "futurelet",
  "global",
  "long",
  "newcommand",
  "renewcommand",
  "providecommand",
]);

/** A control sequence as KaTeX's lexer reads one: a word of letters and "@", or a symbol (\\ among them). */
const controlSequence = /\\(?:[a-zA-Z@]+|[^])/g;

/**
 * definesMacros tells whether tex defines a macro (\def and its kin,
 * \let, \global, \newcommand and its kin) or names one of KaTeX's own (a
 * control word with "@": what \tag defines among them): it is not
 * typeset. A macro used again and again makes a short formula expand
 * without end, past what maxExpand bounds, which counts the expansions,
 * not what they expand to (a formula of 290 bytes took 37 s, M6/P6 B
 * review). KaTeX has no other way to make a control word (no \csname),
 * and none of its own macros repeats what it is given. The control
 * sequences are read as KaTeX's lexer reads them: \\@ is a row's end,
 * then "@".
 */
function definesMacros(tex: string): boolean {
  for (const [sequence] of tex.matchAll(controlSequence)) {
    const name = sequence.slice(1);
    if (defining.has(name) || name.includes("@")) {
      return true;
    }
  }
  return false;
}

/** What of KaTeX mermaid typesets a label's formulas with. */
export type LabelTypesetter = { renderToString: (tex: string, options?: KatexOptions) => string };

const guarded = new WeakSet<LabelTypesetter>();

/**
 * guardLabels has katex's renderToString, which mermaid calls for a
 * label's $$…$$ with options of its own (no maxSize), refuse what math
 * refuses and take math's options. What it is given is the label as
 * mermaid sanitized it, which a look at the diagram's source cannot see:
 * `\d<x></x>ef` is `\def` by then (M6/P6 B fix check f2-3). A refusal
 * fails the drawing, and the diagram shows its source. mermaid puts what
 * it answers in as HTML, whose parser bounds its depth.
 */
export function guardLabels(katex: LabelTypesetter): void {
  if (guarded.has(katex)) {
    return;
  }
  guarded.add(katex);
  const { renderToString } = katex;
  katex.renderToString = (tex, given) => {
    if (definesMacros(tex)) {
      throw new Error("A formula that defines a macro is not typeset");
    }
    return renderToString(tex, { ...given, ...options });
  };
}

/**
 * math typesets the reading view's formulas with KaTeX, which load loads
 * (M6/P6 design 10): each .nw-math, the TeX the server wrote, a block's
 * (nw-math-block) displayed. One KaTeX cannot read, longer than
 * formulaLimit, that defines a macro (definesMacros), or whose typesetting
 * nests deeper than formulaDepth, shows its TeX. They are typeset for
 * taskTime in a task, KaTeX working on the page's thread, which is given
 * back between (now is the clock). Undone, the formulas typeset show their
 * TeX again, and those not reached yet stay as they are.
 */
export function math(load: () => Promise<Typesetter>, now: () => number = () => performance.now()): Enhancement {
  return (container) => {
    const formulas = [...container.querySelectorAll<HTMLElement>(".nw-math")];
    if (formulas.length === 0) {
      return undefined;
    }
    let undone = false;
    const typeset = new Map<HTMLElement, string>();
    void (async () => {
      let katex: Typesetter;
      try {
        katex = await load();
      } catch (error) {
        console.error("KaTeX could not be loaded: the formulas show their TeX", error);
        return;
      }
      let started = now();
      for (const formula of formulas) {
        if (now() - started >= taskTime) {
          // oxlint-disable-next-line no-await-in-loop -- the page's thread is given back between the tasks
          await new Promise((resolve) => setTimeout(resolve, 0));
          started = now();
        }
        if (undone) {
          return;
        }
        const tex = formula.textContent;
        if (encoder.encode(tex).length <= formulaLimit && !definesMacros(tex) && render(katex, tex, formula)) {
          typeset.set(formula, tex);
        }
      }
    })();
    return () => {
      undone = true;
      for (const [formula, tex] of typeset) {
        formula.textContent = tex;
      }
    };
  };
}

/**
 * render puts tex typeset in formula, displayed for a block's, and tells
 * whether KaTeX could, within formulaDepth; otherwise formula is as it
 * was. It is typeset out of the page, where nothing is laid out.
 */
function render(katex: Typesetter, tex: string, formula: HTMLElement): boolean {
  const typeset = document.createElement("span");
  try {
    katex.render(tex, typeset, { ...options, displayMode: formula.classList.contains("nw-math-block") });
  } catch {
    return false;
  }
  if (deeperThan(typeset, formulaDepth)) {
    return false;
  }
  formula.replaceChildren(...typeset.childNodes);
  return true;
}

/** deeperThan tells whether element's descendants nest more than levels deep. */
function deeperThan(element: Element, levels: number): boolean {
  let level = Array.from(element.children);
  for (let depth = 1; level.length > 0; depth += 1) {
    if (depth > levels) {
      return true;
    }
    level = level.flatMap((each) => Array.from(each.children));
  }
  return false;
}
