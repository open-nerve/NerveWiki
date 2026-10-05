import type { Translate } from "../i18n/i18n";
import type { Enhancement } from "./enhancement";

/**
 * What scrolls sideways in the view, the content wider than the page in
 * it: the server's wrapper of a table, the properties' table and a block
 * formula (.nw-scroll), a diagram's that an enhancement adds (.nw-diagram); a code
 * block; and a formula displayed in a paragraph, which has no wrapper.
 */
const scrolling = ".nw-scroll, pre, span.nw-math-block";

/**
 * scrollRegions lets the keyboard scroll what scrolls sideways in the
 * reading view (WCAG 2.1.1, M6/P6 design 6): while one is wider than it
 * shows, and only then, it takes the focus (tabindex 0) as a region named
 * for what it holds, which a screen reader announces as the focus comes. A
 * width follows the window's, and the content's as an enhancement renders
 * it (a formula, a diagram, the code's colours); one added later, a
 * diagram's wrapper, is followed as it comes.
 */
export const scrollRegions: Enhancement = (container, { t }) => {
  const followed = new Set<HTMLElement>();
  const follow = (scroller: HTMLElement) => {
    if (scroller.scrollWidth > scroller.clientWidth) {
      scroller.setAttribute("tabindex", "0");
      scroller.setAttribute("role", "region");
      scroller.setAttribute("aria-label", nameOf(scroller, t));
    } else {
      release(scroller);
    }
  };
  const resized = new ResizeObserver((entries) => {
    for (const entry of entries) {
      follow(entry.target as HTMLElement);
    }
  });
  const take = () => {
    for (const scroller of container.querySelectorAll<HTMLElement>(scrolling)) {
      if (!followed.has(scroller)) {
        followed.add(scroller);
        resized.observe(scroller);
      }
    }
    for (const scroller of followed) {
      follow(scroller);
    }
  };
  take();
  // What an enhancement renders changes the content's width, not the box's, which the resize observer watches.
  const changed = new MutationObserver(take);
  changed.observe(container, { childList: true, subtree: true });
  return () => {
    changed.disconnect();
    resized.disconnect();
    for (const scroller of followed) {
      release(scroller);
    }
  };
};

/** release has scroller no longer take the focus, nor be a region. */
function release(scroller: HTMLElement) {
  scroller.removeAttribute("tabindex");
  scroller.removeAttribute("role");
  scroller.removeAttribute("aria-label");
}

/** nameOf is the region's name: what it holds. */
function nameOf(scroller: HTMLElement, t: Translate): string {
  if (scroller.matches("pre")) {
    return t("reading.code");
  }
  if (scroller.matches("span.nw-math-block") || scroller.querySelector(":scope > .nw-math-block") !== null) {
    return t("reading.formula");
  }
  if (scroller.querySelector(":scope > table.nw-props") !== null) {
    return t("reading.properties");
  }
  if (scroller.querySelector(":scope > table") !== null) {
    return t("reading.table");
  }
  return scroller.classList.contains("nw-diagram") ? t("reading.diagram") : t("reading.wide");
}
