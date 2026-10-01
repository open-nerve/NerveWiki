import { Link, Outlet } from "react-router";

import { LanguageMenu } from "./language-menu";
import { ThemeMenu } from "./theme-menu";
import { UserMenu } from "./user-menu";

/**
 * Layout is the shell around every page: the top bar (with the account once
 * signed in) and the page below it. A page is padded, unless it holds a
 * shell of its own (data-shell, a workspace's), which reaches the edges.
 */
export function Layout() {
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
      <main className="flex flex-1 flex-col p-6 has-[[data-shell]]:p-0">
        <Outlet />
      </main>
    </div>
  );
}
