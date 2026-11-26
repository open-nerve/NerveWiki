import { ChevronDown, type LucideIcon } from "lucide-react";
import type { ReactNode, Ref } from "react";

/**
 * PanelSection is a section of the page's right column (M6/P7 design 7): a
 * disclosure, open, whose summary is its title, a heading as a page's
 * subpages' is: one moving by headings finds it. Its summary as the
 * mockup's .ph (M13/P4/S1, decision 863): 12px and faint, its icon before
 * the title, an arrow at its end, turned as it is closed; no count.
 */
export function PanelSection({
  title,
  icon: Icon,
  summaryRef,
  inert = false,
  children,
}: {
  title: string;
  icon: LucideIcon;
  summaryRef?: Ref<HTMLElement>;
  /** Whether the section, its summary with it, is inert: neither clicked nor tabbed to (M13/P4/S3). */
  inert?: boolean;
  children: ReactNode;
}) {
  return (
    <details open inert={inert} className="group space-y-2">
      <summary
        ref={summaryRef}
        className="flex cursor-pointer list-none items-center gap-1.5 rounded-row text-xs text-faint [&::-webkit-details-marker]:hidden"
      >
        <Icon aria-hidden className="size-3.25 shrink-0" />
        <h2 className="font-normal">{title}</h2>
        <ChevronDown
          aria-hidden
          className="ms-auto size-3.5 shrink-0 transition-transform duration-150 group-not-open:-rotate-90"
        />
      </summary>
      {children}
    </details>
  );
}
