import type { MouseEventHandler, ReactNode } from "react";
import { NavLink } from "react-router";

import { cn } from "../lib/cn";

/**
 * NavItem is a link of a navigation that marks the page shown (aria-current
 * and the active background): the settings' and a workspace's. With end, only
 * its own path counts, not the paths below it. A row of the left column
 * (M13/P2/S1): 30px, its icon then its text; the current one has no hover of
 * its own (M13/P1 4.1.3), and a transparent border, which forced colours
 * draw where they take its background away, its text where it was (M13/P3/S1,
 * decision 845). state is the route state of the page it goes to; onClick,
 * its click (useFollow's, which decides the state itself).
 */
export function NavItem({
  to,
  end,
  state,
  onClick,
  children,
}: {
  to: string;
  end?: boolean;
  state?: unknown;
  onClick?: MouseEventHandler<HTMLAnchorElement>;
  children: ReactNode;
}) {
  return (
    <NavLink
      to={to}
      end={end}
      state={state}
      onClick={onClick}
      className={({ isActive }) =>
        cn(
          "flex h-7.5 items-center gap-2.25 rounded-row px-2 text-row transition-colors duration-150 [&_svg]:size-4 [&_svg]:shrink-0",
          isActive
            ? "border border-transparent bg-active px-1.75 font-medium text-foreground"
            : "text-muted-foreground hover:bg-hover hover:text-foreground"
        )
      }
    >
      {children}
    </NavLink>
  );
}
