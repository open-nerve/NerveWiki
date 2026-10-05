import { render } from "@testing-library/react";
import { createMemoryRouter, type InitialEntry, type RouteObject } from "react-router";

import { routes } from "../app/routes";
import { SessionRoot } from "../app/session-root";
import { EditorExtensions, type EditorExtension } from "../editor/registry";
import { EventHandlers, type EventHandler } from "../events/handlers";
import { Enhancements, type Enhancement } from "../reading/enhancement";
import { testApp } from "./fakes";

type RenderOptions = {
  /** The app's routes unless a test gives its own. */
  routes?: RouteObject[];
  /** The reading views' enhancements: none unless given, where main.tsx gives the app's. */
  enhancements?: readonly Enhancement[];
  /** The editor's extensions: none unless given, where main.tsx gives the registry. */
  editorExtensions?: readonly EditorExtension[];
  /** The event stream's handlers by type: none unless given, where main.tsx gives the app's. */
  eventHandlers?: ReadonlyMap<string, EventHandler>;
};

/**
 * renderApp renders the app's routes at path, or at the last of a history's
 * entries, for app's session, started
 * as the page starts it: by default signed out, over an API that answers
 * GET /api/v0/instance with instanceJSON. The reading views get
 * enhancements, the editor its extensions and the event stream its
 * handlers as the composition root gives them, by default none.
 */
export function renderApp(
  path: string | InitialEntry[],
  app = testApp(),
  {
    routes: appRoutes = routes,
    enhancements = [],
    editorExtensions = [],
    eventHandlers = new Map(),
  }: RenderOptions = {}
) {
  const router = createMemoryRouter(appRoutes, { initialEntries: typeof path === "string" ? [path] : path });
  void app.session.start();
  return {
    app,
    router,
    ...render(
      <Enhancements value={enhancements}>
        <EditorExtensions value={editorExtensions}>
          <EventHandlers value={eventHandlers}>
            <SessionRoot app={app} router={router} />
          </EventHandlers>
        </EditorExtensions>
      </Enhancements>
    ),
  };
}
