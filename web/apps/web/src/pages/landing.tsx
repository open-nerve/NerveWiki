import { observer } from "mobx-react-lite";
import { Navigate, useLocation } from "react-router";
import useSWR from "swr";

import { GiveCornerBack } from "../app/corner";
import { landingPath } from "../app/landing";
import { NotLoaded } from "../app/not-loaded";
import { SessionChangedError } from "../session/token-manager";
import { useStore, useWorkspaces } from "../stores/context";

/**
 * LandingPage is /: it goes on to the workspace this device showed last,
 * the first of the account's, or the page that creates one (M2/P5 design
 * 3.3). A list this generation has loaded already sends it on at once. An
 * arrival goes on with it: the page landed on takes the focus (M3/P4
 * design 3.5). Where it goes has a left column, or is the page that
 * creates a workspace: its route says it has one, the account's button
 * waiting; the list failing to load, it gives the top right back
 * (M13/P2/S1; review A, S1A-M1).
 */
export const LandingPage = observer(function LandingPage() {
  const workspaces = useWorkspaces();
  const { preferences } = useStore();
  const { state } = useLocation();
  const { error, mutate } = useSWR("workspaces", () => workspaces.load());
  if (workspaces.list === undefined) {
    // A load cut by a change of session is loading, as NotLoaded shows it.
    const failed = error !== undefined && !(error instanceof SessionChangedError);
    return (
      <>
        {failed && <GiveCornerBack />}
        <NotLoaded error={error} retry={() => void mutate()} />
      </>
    );
  }
  return <Navigate replace to={landingPath(workspaces.list, preferences.lastWorkspace())} state={state} />;
});
