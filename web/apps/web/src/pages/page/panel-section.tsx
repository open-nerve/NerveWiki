import type { ReactNode } from "react";

/**
 * PanelSection is a section of the page's right column (M6/P7 design 7): a
 * disclosure, open, whose summary is its title, as a page's subpages are
 * titled.
 */
export function PanelSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <details open className="space-y-2">
      <summary className="cursor-pointer text-sm font-medium text-muted-foreground">{title}</summary>
      {children}
    </details>
  );
}
