import type { ReactNode } from "react";
import { NavLink } from "react-router";

import { cn } from "../lib/cn";

/**
 * NavItem is a link of a navigation that marks the page shown (aria-current
 * and the active background): the settings' and a workspace's. With end, only
 * its own path counts, not the paths below it. A row of the left column
 * (M13/P2/S1): 30px, its icon then its text; the current one has no hover of
 * its own (M13/P1 4.1.3).
 */
export function NavItem({ to, end, children }: { to: string; end?: boolean; children: ReactNode }) {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) =>
        cn(
          "flex h-7.5 items-center gap-2.25 rounded-row px-2 text-row transition-colors duration-150 [&_svg]:size-4 [&_svg]:shrink-0",
          isActive
            ? "bg-active font-medium text-foreground"
            : "text-muted-foreground hover:bg-hover hover:text-foreground"
        )
      }
    >
      {children}
    </NavLink>
  );
}
