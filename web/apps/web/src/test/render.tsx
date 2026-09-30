import { render } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";

import { routes } from "../app/routes";
import { StoreProvider } from "../stores/context";
import { PreferencesStore } from "../stores/preferences.store";
import { RootStore } from "../stores/root.store";
import { darkScheme, memoryStorage } from "./fakes";

/** renderApp renders the app's routes at path, over stores made of fakes. */
export function renderApp(
  path: string,
  store = new RootStore(new PreferencesStore(memoryStorage(), darkScheme(false)))
) {
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  return {
    store,
    router,
    ...render(
      <StoreProvider store={store}>
        <RouterProvider router={router} />
      </StoreProvider>
    ),
  };
}
