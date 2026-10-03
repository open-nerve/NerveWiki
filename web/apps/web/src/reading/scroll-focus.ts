import type { Enhancement } from "./enhancement";

/**
 * scrollFocus lets the keyboard scroll what scrolls sideways in the
 * reading view (WCAG 2.1.1): the view itself, which a table wider than it
 * makes scroll, and each code block wider than the view. Each takes the
 * focus (tabindex 0) while it is wider than it shows, and only then, as
 * its width follows the window's. A wrapper of each table, scrolling on
 * its own, needs the renderer (M6, M4 closeout).
 */
export const scrollFocus: Enhancement = (container) => {
  const scrollers = [container, ...container.querySelectorAll<HTMLElement>("pre")];
  const follow = () => {
    for (const scroller of scrollers) {
      if (scroller.scrollWidth > scroller.clientWidth) {
        scroller.tabIndex = 0;
      } else {
        scroller.removeAttribute("tabindex");
      }
    }
  };
  follow();
  const observer = new ResizeObserver(follow);
  for (const scroller of scrollers) {
    observer.observe(scroller);
  }
  return () => {
    observer.disconnect();
    for (const scroller of scrollers) {
      scroller.removeAttribute("tabindex");
    }
  };
};
