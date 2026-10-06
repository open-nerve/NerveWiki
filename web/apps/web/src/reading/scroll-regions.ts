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
 * diagram's wrapper, is followed as it comes. A change is looked at where
 * it is: what scrolls around it, and what it adds. The view itself, which
 * scrolls what is wide and has no region of its own (a table of the
 * writer's own HTML, a long formula in a line), takes the focus as well
 * while it does; it is the page's article, named by it.
 */
export const scrollRegions: Enhancement = (container, { t }) => {
  const followed = new Set<HTMLElement>();
  const follow = (scroller: HTMLElement) => {
    const wide = scroller.scrollWidth > scroller.clientWidth;
    if (scroller === container) {
      if (wide) {
        container.setAttribute("tabindex", "0");
      } else {
        container.removeAttribute("tabindex");
      }
    } else if (wide) {
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
  const take = (scroller: HTMLElement) => {
    if (!followed.has(scroller)) {
      followed.add(scroller);
      resized.observe(scroller);
    }
    follow(scroller);
  };
  take(container);
  for (const scroller of container.querySelectorAll<HTMLElement>(scrolling)) {
    if (ours(scroller)) {
      take(scroller);
    }
  }
  // What an enhancement renders changes the content's width, not the box's, which the resize observer watches.
  const changed = new MutationObserver((records) => {
    const touched = new Set<HTMLElement>([container]);
    for (const record of records) {
      const at = record.target instanceof Element ? record.target : record.target.parentElement;
      let around = at?.closest<HTMLElement>(scrolling);
      while (around && container.contains(around)) {
        touched.add(around);
        around = around.parentElement?.closest<HTMLElement>(scrolling);
      }
      for (const added of record.addedNodes) {
        if (added instanceof HTMLElement) {
          for (const scroller of [added, ...added.querySelectorAll<HTMLElement>(scrolling)]) {
            if (scroller.matches(scrolling)) {
              touched.add(scroller);
            }
          }
        }
      }
    }
    for (const scroller of touched) {
      if (ours(scroller)) {
        take(scroller);
      }
    }
  });
  changed.observe(container, { childList: true, subtree: true });
  return () => {
    changed.disconnect();
    resized.disconnect();
    container.removeAttribute("tabindex");
    for (const scroller of followed) {
      if (scroller !== container) {
        release(scroller);
      }
    }
  };
};

/** ours tells whether scroller is the view's: in a diagram, a label's markup is the writer's, not the server's. */
function ours(scroller: HTMLElement): boolean {
  return scroller.parentElement?.closest(".nw-diagram") === null;
}

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
