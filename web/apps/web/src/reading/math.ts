import type { KatexOptions } from "katex";

import type { Enhancement } from "./enhancement";

/** The longest formula typeset, in bytes of UTF-8: a longer one shows its TeX (M6/P6 design 10). */
export const formulaLimit = 4000;

/** How many formulas are typeset in one task, between which the page may do other work. */
export const formulasAtOnce = 100;

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

/**
 * What defines a macro in the TeX KaTeX reads (\def and its kin, \let,
 * \global, \newcommand and its kin), and a control word with "@",
 * KaTeX's own (what \tag defines among them). A macro used again and
 * again makes a short formula expand without end, past what maxExpand
 * bounds: it counts the expansions, not what they expand to (a formula of
 * 290 bytes took 37 s, M6/P6 B review). Without one, KaTeX's work follows
 * the formula's length. KaTeX has no other way to name a control word
 * (no \csname): the text says it all.
 */
const macroDefinition =
  /\\(?:[gex]?def|let|futurelet|global|long|(?:re)?newcommand|providecommand)(?![a-zA-Z@])|\\[a-zA-Z]*@/;

/** definesMacros tells whether tex defines a macro, or names one of KaTeX's own: it is not typeset. */
export function definesMacros(tex: string): boolean {
  return macroDefinition.test(tex);
}

/**
 * math typesets the reading view's formulas with KaTeX, which load loads
 * (M6/P6 design 10): each .nw-math, the TeX the server wrote, a block's
 * (nw-math-block) displayed. One KaTeX cannot read, longer than
 * formulaLimit, or that defines a macro (definesMacros), shows its TeX.
 * They are typeset formulasAtOnce in a task, KaTeX working on the page's
 * thread. Undone, the formulas typeset show their TeX again, and those not
 * reached yet stay as they are.
 */
export function math(load: () => Promise<Typesetter>): Enhancement {
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
      for (const [i, formula] of formulas.entries()) {
        if (i > 0 && i % formulasAtOnce === 0) {
          // oxlint-disable-next-line no-await-in-loop -- the page's thread is given back between the tasks
          await new Promise((resolve) => setTimeout(resolve, 0));
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

/** render puts tex typeset in formula, displayed for a block's, and tells whether KaTeX could; otherwise formula is as it was. */
function render(katex: Typesetter, tex: string, formula: HTMLElement): boolean {
  const typeset = document.createElement("span");
  try {
    katex.render(tex, typeset, { ...options, displayMode: formula.classList.contains("nw-math-block") });
  } catch {
    return false;
  }
  formula.replaceChildren(...typeset.childNodes);
  return true;
}
