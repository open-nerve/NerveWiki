import { createContext, useContext, useState } from "react";
import { Link, Outlet } from "react-router";

import { LanguageMenu } from "./language-menu";
import { ThemeMenu } from "./theme-menu";
import { UserMenu } from "./user-menu";

/** ShellColumn is the element a page's shell puts its left column in: before the main, outside it. */
const ShellColumn = createContext<HTMLElement | null>(null);

/**
 * useShellColumn is the element a shell portals its left column into
 * (createPortal), or null before the layout has it: the column is
 * navigation beside the page, so it stays out of the main (M3 handoff 1;
 * M4/P5 design 3.2).
 */
export function useShellColumn(): HTMLElement | null {
  return useContext(ShellColumn);
}

/**
 * Layout is the shell around every page: the top bar (with the account once
 * signed in) and the page below it, the one main, which holds whatever the
 * routes render: a page, its error, its 404. A shell of a page's own (a
 * workspace's) puts its left column beside the main, by useShellColumn;
 * the column's place renders no box of its own.
 */
export function Layout() {
  const [column, setColumn] = useState<HTMLElement | null>(null);
  return (
    <div className="flex min-h-svh flex-col">
      <header className="flex h-14 items-center justify-between border-b px-4">
        <Link to="/" className="font-semibold">
          Nerve Wiki
        </Link>
        <div className="flex items-center gap-1">
          <LanguageMenu />
          <ThemeMenu />
          <UserMenu />
        </div>
      </header>
      <div className="flex flex-1 flex-col md:flex-row">
        <div ref={setColumn} className="contents" />
        <main className="flex min-w-0 flex-1 flex-col p-6">
          <ShellColumn value={column}>
            <Outlet />
          </ShellColumn>
        </main>
      </div>
    </div>
  );
}
