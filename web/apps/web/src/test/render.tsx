import { render } from "@testing-library/react";
import { createMemoryRouter } from "react-router";

import { routes } from "../app/routes";
import { SessionRoot } from "../app/session-root";
import { testApp } from "./fakes";

/**
 * renderApp renders the app's routes at path for app's session, started
 * as the page starts it: by default signed out, over an API that answers
 * GET /api/v0/instance with instanceJSON.
 */
export function renderApp(path: string, app = testApp(), appRoutes = routes) {
  const router = createMemoryRouter(appRoutes, { initialEntries: [path] });
  void app.session.start();
  return { app, router, ...render(<SessionRoot app={app} router={router} />) };
}
