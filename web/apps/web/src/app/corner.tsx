import { ArrowLeft } from "lucide-react";
import { observer } from "mobx-react-lite";
import { createContext, use, useLayoutEffect } from "react";
import { Link, useMatch } from "react-router";
import useSWR from "swr";

import { useT } from "../i18n/i18n";
import { SessionChangedError } from "../session/token-manager";
import { useStore } from "../stores/context";
import { AppearanceMenu } from "./appearance-menu";
import { arrived } from "./arrival";
import { useSession } from "./session-state";
import { UserMenu } from "./user-menu";

/**
 * CornerBack gives the top right back where a route with a left column shows none (M13/P2/S1, decision 769): it
 * counts one more giver, undone by what it answers. Outside the layout, the root route's error boundary, it does
 * nothing: there is no top right there.
 */
export const CornerBack = createContext<() => () => void>(() => () => {});

/**
 * useCornerBack has the layout show the top right while the component is mounted, though the route has a left column:
 * the workspace's shell failing to load it or not finding it, the session unavailable, a route's error. In a layout
 * effect: the layout renders again before the paint, the corner never flashing (decision 793).
 */
export function useCornerBack(): void {
  const giveBack = use(CornerBack);
  useLayoutEffect(() => giveBack(), [giveBack]);
}

/**
 * GiveCornerBack has the top right shown in the column's stead (useCornerBack), rendered only where what a route with
 * a left column shows has none: the workspaces' load failed, at / or in a workspace, or the workspace not found; never
 * while they load (M13/P2/S1, decision 769; review A, S1A-M1): the account's button would mount there, then again in
 * the column.
 */
export function GiveCornerBack(): null {
  useCornerBack();
  return null;
}

/**
 * Corner is the top right where no left column is (M13/P2/S1, decisions 769, 792): signed in, the account's button,
 * after Back to the workspace on the page that creates a workspace once the account has one to go back to (decision
 * 803); signed out, the session unavailable or the account's load failed, Appearance; while the session starts or the
 * account loads, nothing, its height kept. It loads the account itself, by the guard's key: an error boundary above
 * the guards may be the tab's first page, the account never loaded (review C-M1); one load serves both, and none once
 * it is in. The workspaces it reads on the page that creates one by the landing's and the shell's key, mostly read
 * already.
 */
export const Corner = observer(function Corner() {
  const { status } = useSession();
  const { account, workspaces } = useStore();
  const t = useT();
  const creating = useMatch("/create-workspace") !== null;
  const loading = status === "signed-in" && account !== undefined && account.me === undefined;
  const { error } = useSWR(loading ? "me" : null, () => account?.load());
  useSWR(creating && status === "signed-in" && workspaces !== undefined ? "workspaces" : null, () =>
    workspaces?.load()
  );
  const failed = error !== undefined && !(error instanceof SessionChangedError);
  // Without a workspace, / lands on the page that creates one: back would be there again.
  const back = creating && (workspaces?.list?.length ?? 0) > 0;
  let shown = null;
  if (status === "signed-in" && account?.me !== undefined) {
    shown = (
      <>
        {back && (
          <Link
            to="/"
            state={arrived}
            className="inline-flex h-7.5 shrink-0 items-center gap-1.5 rounded-full px-3 text-compact font-medium whitespace-nowrap text-muted-foreground transition-colors duration-150 hover:bg-hover hover:text-foreground [&_svg]:size-4"
          >
            <ArrowLeft aria-hidden />
            {t("shell.backToWorkspace")}
          </Link>
        )}
        <UserMenu place="corner" />
      </>
    );
  } else if (status === "signed-out" || status === "unavailable" || failed) {
    shown = <AppearanceMenu />;
  }
  return <div className="flex h-12 shrink-0 items-center justify-end gap-1 px-3 md:px-4">{shown}</div>;
});
