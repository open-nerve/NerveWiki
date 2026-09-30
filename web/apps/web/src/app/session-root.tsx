import { useMemo, useSyncExternalStore } from "react";
import { RouterProvider, type createBrowserRouter } from "react-router";

import { RootStore, type AppStores } from "../stores/root.store";
import { AppProviders } from "./providers";

type Router = ReturnType<typeof createBrowserRouter>;

/**
 * SessionRoot renders the app for the tab's session (M1/P5 design 3.3):
 * each login gets its generation of RootStore, and AppProviders mounts anew
 * with it, so that nothing of the session before (a store, a cached
 * answer) reaches the next. The router outlives the generations: a change
 * of session keeps the address, and the route guards decide where to go.
 */
export function SessionRoot({ app, router }: { app: AppStores; router: Router }) {
  const { tokens } = app.session;
  const { loginId } = useSyncExternalStore(tokens.subscribe, () => tokens.state);
  const store = useMemo(() => new RootStore(app, loginId), [app, loginId]);
  return (
    <AppProviders key={loginId ?? "signed-out"} store={store}>
      <RouterProvider router={router} />
    </AppProviders>
  );
}
