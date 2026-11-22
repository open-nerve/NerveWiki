import type { ReactNode } from "react";

import { UserMenu } from "./user-menu";

/**
 * ShellSidebar is a shell's left column (M13/P2/S1, decision 774), in
 * three parts: its top, which stays (the workspace's switcher and search);
 * its middle, which scrolls itself ([data-sidebar-scroll]), the end of its
 * scroll carrying the window on, as before the column stuck: contained,
 * Chromium let no wheel over a middle that does not overflow scroll the
 * window (decision 802; review A, S1A-M2); its foot, the
 * account's row, above a line, the middle scrolling past it. On the
 * sidebar's colour, a shade off the page. From md: up it sticks to the
 * window's left, as tall as the window, which still scrolls the whole
 * page (00-M13 decision 717); below md, until the drawer (M13/P2/S3), it
 * comes before the page. It is no scroll anchor of the window's
 * (overflow-anchor: none), as when it was as tall as the page beside it,
 * which what changes size above the reader would move (M6/P6 B fix check
 * 4): stuck, a probe of the three engines saw no difference either way,
 * harmless kept (M13/P2/S1).
 */
/**
 * intoColumn brings element into view in the left column's middle ([data-sidebar-scroll]), 8px to spare above or
 * below it, by that area's scrollTop alone: never scrollIntoView, which scrolls the window too (v0.1 design 13.2,
 * item 26; M13/P2/S2, 4.2.5). It answers false for an element in no such area.
 */
export function intoColumn(element: Element): boolean {
  const area = element.closest<HTMLElement>("[data-sidebar-scroll]");
  if (area === null) {
    return false;
  }
  const row = element.getBoundingClientRect();
  const view = area.getBoundingClientRect();
  if (row.top < view.top) {
    area.scrollTop -= view.top - row.top + 8;
  } else if (row.bottom > view.bottom) {
    area.scrollTop += row.bottom - view.bottom + 8;
  }
  return true;
}

export function ShellSidebar({ top, children }: { top: ReactNode; children: ReactNode }) {
  return (
    <div
      data-shell
      className="flex flex-col bg-sidebar [overflow-anchor:none] max-md:border-b max-md:border-border md:sticky md:top-0 md:h-svh md:w-63 md:shrink-0 md:self-start md:border-r md:border-border"
    >
      <div className="space-y-2 px-2 pt-2.5 pb-1.5">{top}</div>
      {/* pt-1: the area clips what overflows it, the first link's focus ring 2px out of it included. */}
      <div data-sidebar-scroll className="min-h-0 flex-1 overflow-y-auto px-2 pt-1 pb-3">
        {children}
      </div>
      <div className="shrink-0 border-t border-border p-2">
        <UserMenu place="row" />
      </div>
    </div>
  );
}
