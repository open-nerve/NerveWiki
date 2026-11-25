import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

/** A page's list, centred, room above and below it; a settings card's, in the card's stead of a line of text. */
type Size = "page" | "card";

const frame: Record<Size, string> = {
  page: "flex flex-col items-center gap-3 px-6 py-14 text-center",
  card: "flex flex-col items-center gap-2.5 px-4.5 py-7 text-center",
};

/** The tile an empty list's or a status page's icon is in (StatusPage shares it). */
export const iconTileClass: Record<Size, string> = {
  page: "grid size-11 shrink-0 place-items-center rounded-card border border-border bg-surface-2 text-muted-foreground [&_svg]:size-5",
  card: "grid size-9 shrink-0 place-items-center rounded-control border border-border bg-surface-2 text-muted-foreground [&_svg]:size-4.5",
};

/**
 * EmptyState says a list has nothing in it (00-M13 4.3; M13/P5/S1, decision 913): an icon in its tile, read by no one,
 * the line that says it, a paragraph (the line the list said before, its key unchanged), then what to do (action, a
 * button its caller gives, a small pill as a status page's actions: decision 949), if anything. No title, and no role:
 * it is no status, what a page says as it changes its own output. Two sizes: a page's list (page) and a settings
 * card's (card).
 */
export function EmptyState({
  icon: Icon,
  size = "page",
  action,
  children,
}: {
  icon: LucideIcon;
  size?: Size;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className={frame[size]}>
      <div aria-hidden className={iconTileClass[size]}>
        <Icon />
      </div>
      <p className="max-w-90 text-sm text-balance text-muted-foreground">{children}</p>
      {action !== undefined && <div className="mt-1 flex flex-wrap justify-center gap-2">{action}</div>}
    </div>
  );
}
