import type { KatexOptions } from "katex";

import type { Enhancement } from "./enhancement";

/** The longest formula typeset, in bytes of UTF-8: a longer one shows its TeX (M6/P6 design 10). */
export const formulaLimit = 4000;

/**
 * The deepest a typeset formula's elements may nest; a deeper one shows
 * its TeX. Past about 800, Chromium's layout crashes the page (135 levels
 * of x^{x^…}, in 541 bytes: M6/P6 B fix check f2-2), and well before, a
 * chain of styled groups takes the layout a time that grows faster than
 * its depth: 1.3 s at 489, 129 ms at 150 (second fix check). Ten nested
 * fractions are 76 deep, a 30 by 30 matrix 21.
 */
const formulaDepth = 150;

/** How long, in milliseconds, formulas are typeset in one task before the page's thread is given back. */
export const taskTime = 50;

/**
 * How long, in milliseconds, a view's formulas may take to lay out in
 * their places; past it, the rest show their TeX. The view laid out again
 * (zoomed, printed) lays them out all at once, in one task: 40 formulas of
 * styled groups 150 deep, each laid out in a task of its own as it was
 * typeset, took 5 s at once when the page was zoomed (M6/P6 B fix check
 * 3). Laid out in a box of its own, a wide formula takes a third of what
 * it takes in its paragraph (fix check 4): the budget counts them in their
 * places.
 */
export const layoutBudget = 1000;

/** What typesetting uses of KaTeX; the tests give their own. */
export type Typesetter = { render: (tex: string, element: HTMLElement, options: KatexOptions) => void };

/**
 * KaTeX's options (M6 design 4.9): no command that loads, links or styles
 * from the TeX (trust false), sizes and macro expansions bounded, and an
 * error thrown, never rendered, so that the formula shows its TeX instead.
 * What strict mode would warn of, with the writer's TeX, is let be. KaTeX
 * still warns of a character it has no metrics for (€, Hebrew, an emoji):
 * one character, in a message of its own.
 */
const options: KatexOptions = { trust: false, maxSize: 50, maxExpand: 1000, throwOnError: true, strict: false };

/** loadKatex loads KaTeX and its stylesheet, with its fonts, in chunks of their own, once a view has a formula. */
export async function loadKatex(): Promise<Typesetter> {
  const [katex] = await Promise.all([import("katex"), import("katex/dist/katex.min.css")]);
  return katex.default;
}

const encoder = new TextEncoder();

/** The control words refused in the TeX KaTeX reads: those that define a macro, and those that write to the console. */
const refused = new Set([
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
  "message",
  "errmessage",
  "show",
]);

/** A control sequence as KaTeX's lexer reads one: a word of letters and "@", or a symbol (\\ among them). */
const controlSequence = /\\(?:[a-zA-Z@]+|[^])/g;

/**
 * definesMacros tells whether tex defines a macro (\def and its kin,
 * \let, \global, \newcommand and its kin) or names one of KaTeX's own (a
 * control word with "@": what \tag defines among them): it is not
 * typeset; nor is one that writes to the reader's console (\message,
 * \errmessage, \show). A macro used again and again makes a short formula expand
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
    if (refused.has(name) || name.includes("@")) {
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
 * `\d<x></x>ef` is `\def` by then (M6/P6 B fix check f2-3). Longer than
 * formulaLimit, or what it answers, parsed, nesting deeper than
 * formulaDepth, is refused too. A refusal fails the drawing, and the
 * diagram shows its source.
 */
export function guardLabels(katex: LabelTypesetter): void {
  if (guarded.has(katex)) {
    return;
  }
  guarded.add(katex);
  const { renderToString } = katex;
  katex.renderToString = (tex, given) => {
    if (encoder.encode(tex).length > formulaLimit || definesMacros(tex)) {
      throw new Error("A formula past the limits is not typeset");
    }
    const html = renderToString(tex, { ...given, ...options });
    const parsed = document.createElement("template");
    parsed.innerHTML = html;
    if (deeperThan(parsed.content, formulaDepth)) {
      throw new Error("A formula past the limits is not typeset");
    }
    return html;
  };
}

/**
 * math typesets the reading view's formulas with KaTeX, which load loads
 * (M6/P6 design 10): each .nw-math, the TeX the server wrote, a block's
 * (nw-math-block) displayed. One KaTeX cannot read, longer than
 * formulaLimit, that defines a macro (definesMacros), or whose typesetting
 * nests deeper than formulaDepth, shows its TeX. They are typeset for
 * taskTime in a task, KaTeX working on the page's thread, which is given
 * back between (now is the clock). Each is laid out first on its own, in a
 * box of the view out of its flow and unseen (nw-math-measure), so that
 * the task's clock counts the layout, which can take far longer than
 * KaTeX, and counts it once: laid out in its paragraph, each formula would
 * lay out the paragraph again (2,000 in one took 12.8 s; M6/P6 B fix check
 * 3). The formulas of a task are put in their places together as it ends,
 * and laid out there, which layoutBudget counts; past it, the rest show
 * their TeX. What else the view has to lay out as a task begins, what
 * other enhancements changed, is laid out before the formulas are,
 * counted to the task, not to the formulas. Undone, the formulas typeset
 * show their TeX again, and those not reached yet stay as they are.
 */
export function math(load: () => Promise<Typesetter>, now: () => number = () => performance.now()): Enhancement {
  return (container) => {
    const formulas = [...container.querySelectorAll<HTMLElement>(".nw-math")];
    if (formulas.length === 0) {
      return undefined;
    }
    let undone = false;
    const typeset = new Map<HTMLElement, string>();
    const measure = document.createElement("div");
    measure.className = "nw-math-measure";
    void (async () => {
      let katex: Typesetter;
      try {
        katex = await load();
      } catch (error) {
        console.error("KaTeX could not be loaded: the formulas show their TeX", error);
        return;
      }
      if (undone) {
        return;
      }
      container.append(measure);
      let laidOut = 0;
      let ready: { formula: HTMLElement; tex: string; made: HTMLElement }[] = [];
      const place = () => {
        if (ready.length === 0) {
          return;
        }
        for (const { formula, tex, made } of ready) {
          formula.replaceChildren(...made.childNodes);
          typeset.set(formula, tex);
        }
        ready = [];
        const before = now();
        container.getBoundingClientRect();
        laidOut += now() - before;
      };
      // Each task, the first too, lays the view out first.
      let started = -Infinity;
      for (const formula of formulas) {
        if (now() - started >= taskTime) {
          place();
          // oxlint-disable-next-line no-await-in-loop -- the page's thread is given back between the tasks
          await new Promise((resolve) => setTimeout(resolve, 0));
          if (undone) {
            return;
          }
          started = now();
          container.getBoundingClientRect();
        }
        if (laidOut >= layoutBudget) {
          break;
        }
        const tex = formula.textContent;
        const made =
          encoder.encode(tex).length <= formulaLimit && !definesMacros(tex)
            ? render(katex, tex, formula.classList.contains("nw-math-block"))
            : undefined;
        if (made !== undefined) {
          measure.replaceChildren(made);
          made.getBoundingClientRect();
          ready.push({ formula, tex, made });
        }
      }
      place();
      measure.remove();
    })();
    return () => {
      undone = true;
      measure.remove();
      for (const [formula, tex] of typeset) {
        formula.textContent = tex;
      }
    };
  };
}

/**
 * render is tex typeset, displayed if a block's, out of the page, where
 * nothing is laid out; undefined if KaTeX cannot, or its typesetting nests
 * deeper than formulaDepth.
 */
function render(katex: Typesetter, tex: string, displayed: boolean): HTMLElement | undefined {
  const typeset = document.createElement("span");
  try {
    katex.render(tex, typeset, { ...options, displayMode: displayed });
  } catch {
    return undefined;
  }
  return deeperThan(typeset, formulaDepth) ? undefined : typeset;
}

/** deeperThan tells whether node's descendants nest more than levels deep. */
function deeperThan(node: ParentNode, levels: number): boolean {
  let level = Array.from(node.children);
  for (let depth = 1; level.length > 0; depth += 1) {
    if (depth > levels) {
      return true;
    }
    level = level.flatMap((each) => Array.from(each.children));
  }
  return false;
}
