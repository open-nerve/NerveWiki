import type { ReactNode } from "react";
import { NavLink } from "react-router";

import { cn } from "../lib/cn";

/**
 * NavItem is a link of a navigation that marks the page shown (aria-current
 * and the accent): the settings' and a workspace's. With end, only its own
 * path counts, not the paths below it.
 */
export function NavItem({ to, end, children }: { to: string; end?: boolean; children: ReactNode }) {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) =>
        cn("rounded-md px-3 py-2 text-sm hover:bg-accent", isActive ? "bg-accent font-medium" : "text-muted-foreground")
      }
    >
      {children}
    </NavLink>
  );
}
