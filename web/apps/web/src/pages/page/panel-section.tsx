import type { ReactNode, Ref } from "react";

/**
 * PanelSection is a section of the page's right column (M6/P7 design 7): a
 * disclosure, open, whose summary is its title, a heading as a page's
 * subpages' is: one moving by headings finds it.
 */
export function PanelSection({
  title,
  summaryRef,
  children,
}: {
  title: string;
  summaryRef?: Ref<HTMLElement>;
  children: ReactNode;
}) {
  return (
    <details open className="space-y-2">
      <summary ref={summaryRef} className="cursor-pointer text-sm text-muted-foreground">
        <h2 className="inline font-medium">{title}</h2>
      </summary>
      {children}
    </details>
  );
}
