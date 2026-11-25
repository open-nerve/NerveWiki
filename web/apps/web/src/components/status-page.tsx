import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { iconTileClass } from "./empty-state";

/** A status page's line that says why: under its title, centred, as wide as an entry card. */
export const statusBodyClass = "max-w-100 text-sm text-balance text-muted-foreground";
/** A status page's actions: a row under its line, centred, broken where it is narrow. */
export const statusActionsClass = "mt-2 flex flex-wrap justify-center gap-2";

/**
 * StatusPage is a page that says why there is nothing (M13/P5/S1, decision 921): a page not found, an error, the
 * session unavailable. Centred, its icon in a tile read by no one, then what the caller writes: its h1 (entryTitleClass),
 * its line (statusBodyClass) and its actions (statusActionsClass). alert, the section is an alert, its whole text read
 * as it shows, the title in it.
 */
export function StatusPage({
  icon: Icon,
  alert = false,
  children,
}: {
  icon: LucideIcon;
  alert?: boolean;
  children: ReactNode;
}) {
  return (
    <section
      role={alert ? "alert" : undefined}
      className="flex flex-col items-center gap-3 px-2 pt-[6svh] text-center md:pt-[10svh]"
    >
      <div aria-hidden className={iconTileClass.page}>
        <Icon />
      </div>
      {children}
    </section>
  );
}
