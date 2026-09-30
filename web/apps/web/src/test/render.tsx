import { render } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";

import { AppProviders } from "../app/providers";
import { routes } from "../app/routes";
import { RootStore } from "../stores/root.store";
import { fakeApi, instanceJSON, json, preferences } from "./fakes";

/**
 * renderApp renders the app's routes at path, over a RootStore whose API
 * answers GET /api/v0/instance with instanceJSON unless the test passes
 * another store.
 */
export function renderApp(
  path: string,
  store = new RootStore(
    preferences(),
    fakeApi(() => json(instanceJSON))
  )
) {
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  return {
    store,
    router,
    ...render(
      <AppProviders store={store}>
        <RouterProvider router={router} />
      </AppProviders>
    ),
  };
}
