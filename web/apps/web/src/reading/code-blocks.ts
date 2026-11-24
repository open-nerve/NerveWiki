import type { Translate } from "../i18n/i18n";
import type { Enhancement } from "./enhancement";

/** How long a copy's word shows: Copied, or Couldn't copy. */
const shownFor = 1500;

/** languageOf is the language a code block's code names (its language- class), or none. */
function languageOf(code: HTMLElement): string | undefined {
  return [...code.classList].find((name) => name.startsWith("language-"))?.slice("language-".length);
}

/**
 * copyButton is a code block's Copy, and what it said beside it: a copy's word for a moment, Copied for a screen reader
 * alone (the button's check is what shows), Couldn't copy shown (decision 890). Its name is Copy whatever it said.
 * Only the last press tells: an earlier one's answer, come later, neither says nor clears (review B's doubt); nor does
 * one come after the cleanup (review A's N1). Its timers are kept in timers, for the cleanup.
 */
function copyButton(
  code: HTMLElement,
  t: Translate,
  timers: Set<ReturnType<typeof setTimeout>>,
  undone: { now: boolean }
): HTMLElement[] {
  const said = document.createElement("span");
  said.className = "nw-code-said";
  // No output: a status would be another of the page's (the edit's is its one), and Copy keeps its name.
  said.setAttribute("aria-live", "polite");
  const copy = document.createElement("button");
  copy.type = "button";
  copy.className = "nw-code-copy";
  copy.textContent = t("reading.copy");
  let presses = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const clear = () => {
    said.textContent = "";
    delete said.dataset.state;
    delete copy.dataset.state;
  };
  const told = (press: number, copied: boolean) => {
    if (press !== presses || undone.now) {
      return;
    }
    if (copied) {
      copy.dataset.state = "copied";
    }
    said.dataset.state = copied ? "copied" : "failed";
    said.textContent = t(copied ? "reading.copied" : "reading.copyFailed");
    const shown = setTimeout(() => {
      clear();
      timers.delete(shown);
    }, shownFor);
    timer = shown;
    timers.add(shown);
  };
  copy.addEventListener("click", () => {
    presses += 1;
    const press = presses;
    clear();
    if (timer !== undefined) {
      clearTimeout(timer);
      timers.delete(timer);
      timer = undefined;
    }
    // The code's text alone, not its head's: what the reader would select of it.
    navigator.clipboard.writeText(code.textContent).then(
      () => told(press, true),
      () => told(press, false)
    );
  });
  return [said, copy];
}

/**
 * codeBlocks gives each code block (a pre whose one child is a code not mermaid's: a diagram's is still one until
 * drawn; an author's pre of more than one code has none, review A's N2) its head (00-M13 4.5.3; M13/P4/S2, decisions
 * 869, 890): the language its code names, and Copy where the page may use the clipboard, which copies the code's text
 * alone. A block of neither has none. The head is the pre's first child: the view's top-level node is the same, the
 * block's region its name still (scrollRegions, after it); reading.css keeps it at the block's left as the code
 * scrolls sideways, unselectable, and does not print it. Cleaned up, the heads go and their words' timers with them, a
 * copy answered later saying nothing: the view compares its blocks by the server's HTML, and the Copy that had the
 * focus is found again by its place in its block (patch.ts).
 */
export const codeBlocks: Enhancement = (container, { t }) => {
  const heads: HTMLElement[] = [];
  const timers = new Set<ReturnType<typeof setTimeout>>();
  const undone = { now: false };
  // As create-token-dialog.tsx asks: a page served insecurely, or a browser without it, has none.
  const clipboard = typeof navigator.clipboard?.writeText === "function";
  for (const code of container.querySelectorAll<HTMLElement>("pre > code:only-child:not(.language-mermaid)")) {
    const language = languageOf(code);
    if (language === undefined && !clipboard) {
      continue;
    }
    const head = document.createElement("div");
    head.className = "nw-code-head";
    if (language !== undefined) {
      head.append(language);
    }
    if (clipboard) {
      head.append(...copyButton(code, t, timers, undone));
    }
    code.parentElement?.prepend(head);
    heads.push(head);
  }
  if (heads.length === 0) {
    return undefined;
  }
  return () => {
    undone.now = true;
    for (const timer of timers) {
      clearTimeout(timer);
    }
    timers.clear();
    for (const head of heads) {
      head.remove();
    }
  };
};
