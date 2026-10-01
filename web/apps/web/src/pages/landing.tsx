import { observer } from "mobx-react-lite";
import { Navigate } from "react-router";
import useSWR from "swr";

import { landingPath } from "../app/landing";
import { NotLoaded } from "../app/not-loaded";
import { useStore, useWorkspaces } from "../stores/context";

/**
 * LandingPage is /: it goes on to the workspace this device showed last,
 * the first of the account's, or the page that creates one (M2/P5 design
 * 3.3). A list this generation has loaded already sends it on at once.
 */
export const LandingPage = observer(function LandingPage() {
  const workspaces = useWorkspaces();
  const { preferences } = useStore();
  const { error, mutate } = useSWR("workspaces", () => workspaces.load());
  if (workspaces.list === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  return <Navigate replace to={landingPath(workspaces.list, preferences.lastWorkspace())} />;
});
