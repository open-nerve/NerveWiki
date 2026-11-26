import { File } from "lucide-react";

import { cn } from "../lib/cn";

/**
 * PageGlyph is a page's icon, the left column's tree's, and stands for nothing else (M13 closeout, decision 969): the
 * quick switch's options, a page's subpages and the notebook's home, a search's hits and the trash's rows and preview
 * show it for a page. A source page keeps its own in the tree (FileText), an attachment its kind's (AssetIcon).
 */
export const PageGlyph = File;

/** PageIcon is a page's icon (PageGlyph), for the eye alone, 16px; className places and colours it. */
export function PageIcon({ className }: { className?: string }) {
  return <PageGlyph aria-hidden className={cn("size-4 shrink-0", className)} />;
}
