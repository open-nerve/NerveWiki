import { Link, Outlet } from "react-router";

import { LanguageMenu } from "./language-menu";
import { ThemeMenu } from "./theme-menu";
import { ThemeSync } from "./theme-sync";

/** Layout is the shell around every page: the top bar and the page below it. */
export function Layout() {
  return (
    <div className="flex min-h-svh flex-col">
      <ThemeSync />
      <header className="flex h-14 items-center justify-between border-b px-4">
        <Link to="/" className="font-semibold">
          Nerve Wiki
        </Link>
        <div className="flex items-center gap-1">
          <LanguageMenu />
          <ThemeMenu />
        </div>
      </header>
      <main className="flex-1 p-6">
        <Outlet />
      </main>
    </div>
  );
}
