import type { ReactNode } from "react";

import { UserMenu } from "./user-menu";

/**
 * ShellSidebar is a shell's left column (M13/P2/S1, decision 774), in
 * three parts: its top, which stays (the workspace's switcher and search);
 * its middle, which scrolls itself ([data-sidebar-scroll]), the end of its
 * scroll not carrying the window on (overscroll-contain); its foot, the
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
export function ShellSidebar({ top, children }: { top: ReactNode; children: ReactNode }) {
  return (
    <div
      data-shell
      className="flex flex-col bg-sidebar [overflow-anchor:none] max-md:border-b max-md:border-border md:sticky md:top-0 md:h-svh md:w-63 md:shrink-0 md:self-start md:border-r md:border-border"
    >
      <div className="space-y-2 px-2 pt-2.5 pb-1.5">{top}</div>
      {/* pt-1: the area clips what overflows it, the first link's focus ring 2px out of it included. */}
      <div data-sidebar-scroll className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-2 pt-1 pb-3">
        {children}
      </div>
      <div className="shrink-0 border-t border-border p-2">
        <UserMenu place="row" />
      </div>
    </div>
  );
}
