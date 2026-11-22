import { createContext, useCallback, useContext, useState } from "react";
import { Outlet, useMatches } from "react-router";

import { useArrivalWatch } from "./arrival";
import { Corner, CornerBack } from "./corner";
import { FileDropGuard } from "./file-drop";

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

/** What a route says of the shell: sidebar, that its pages have a left column, the account in its foot. */
type Shell = { sidebar?: boolean };

/**
 * Layout is the shell around every page: no top bar (M13/P2/S1), the left
 * column's place and, beside it, the page, the one main, which holds
 * whatever the routes render: a page, its error, its 404. A shell of a
 * page's own (a workspace's) puts its left column beside the main, by
 * useShellColumn; the column's place renders no box of its own. Where no
 * left column is, the top right holds the account (Corner). A route that
 * has one (handle: { sidebar: true }) has the account in the column's
 * foot instead, unless what shows in its place gives the top right back
 * (useCornerBack): the workspaces not loaded, at / or in a workspace, or
 * the workspace not found, the session unavailable, an error (decision
 * 769). The route says so as it renders:
 * a mark set in an effect would show the top right for a paint first. A
 * file dropped where no part of the page takes it stays out of the tab
 * (FileDropGuard). It watches the reader from each arrival on
 * (useArrivalWatch).
 */
export function Layout() {
  useArrivalWatch();
  const [column, setColumn] = useState<HTMLElement | null>(null);
  const [given, setGiven] = useState(0);
  const giveBack = useCallback(() => {
    setGiven((n) => n + 1);
    return () => setGiven((n) => n - 1);
  }, []);
  const sidebar = useMatches().some((match) => (match.handle as Shell | undefined)?.sidebar === true);
  return (
    <div className="flex min-h-svh flex-col md:flex-row">
      <FileDropGuard />
      <div ref={setColumn} className="contents" />
      <div className="flex min-w-0 flex-1 flex-col">
        {(!sidebar || given > 0) && <Corner />}
        <main className="flex min-w-0 flex-1 flex-col p-6">
          <CornerBack value={giveBack}>
            <ShellColumn value={column}>
              <Outlet />
            </ShellColumn>
          </CornerBack>
        </main>
      </div>
    </div>
  );
}
