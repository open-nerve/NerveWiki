import type { MouseEvent } from "react";

/**
 * linkFollowed is the address of the plain link a click on a list of them
 * landed on, when the app is to follow it as a router's link would
 * (react-router's useLinkClickHandler): the main button, no modifier key,
 * the link opening where it is, the click not prevented, here or by a list
 * inside this one that followed it already. Any other (a new tab or
 * window, the middle button) is the browser's: undefined. One handler on
 * a list instead of a router's link on each of its rows (M12/P4/S4 design
 * 3.3, 3.7).
 */
export function linkFollowed(event: MouseEvent<HTMLElement>): string | undefined {
  if (
    event.defaultPrevented ||
    event.button !== 0 ||
    event.metaKey ||
    event.altKey ||
    event.ctrlKey ||
    event.shiftKey ||
    !(event.target instanceof Element)
  ) {
    return undefined;
  }
  const link = event.target.closest("a");
  const target = link?.getAttribute("target") ?? "";
  if (link === null || !event.currentTarget.contains(link) || (target !== "" && target !== "_self")) {
    return undefined;
  }
  return link.getAttribute("href") ?? undefined;
}
