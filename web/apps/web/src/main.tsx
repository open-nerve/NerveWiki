import "./styles.css";

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { createBrowserRouter } from "react-router";

import { routes } from "./app/routes";
import { SessionRoot } from "./app/session-root";
import { EditorExtensions, editorExtensions } from "./editor/registry";
import { browserEventDeps } from "./events/deps";
import { EventHandlers, eventHandlers } from "./events/handlers";
import { Enhancements, readingEnhancements } from "./reading/enhancement";
import { browserSessionDeps, Session, type SessionDeps } from "./session/session";
import { PreferencesStore } from "./stores/preferences.store";
import { AppStores } from "./stores/root.store";

const root = document.getElementById("root");
if (!root) {
  throw new Error("index.html has no #root element");
}
const storage = browserStorage();
const preferences = new PreferencesStore({
  storage,
  darkScheme: window.matchMedia("(prefers-color-scheme: dark)"),
  languages: navigator.languages,
});
// The page's one session: the clients of the API, on the page's own origin, and the tokens.
const sessionDeps = browserSessionDeps(storage);
const session = new Session(sessionDeps);
void session.start();
// The event stream's tabs share the session's storage, for the lease where the page has no Web Locks.
const events = browserEventDeps(storage, sessionDeps.onStorage, sessionDeps.randomHex(16));
createRoot(root).render(
  <StrictMode>
    <Enhancements value={readingEnhancements}>
      <EditorExtensions value={editorExtensions}>
        <EventHandlers value={eventHandlers}>
          <SessionRoot app={new AppStores(preferences, session, events)} router={createBrowserRouter(routes)} />
        </EventHandlers>
      </EditorExtensions>
    </Enhancements>
  </StrictMode>
);

// Reading window.localStorage throws where the browser blocks site data; the
// preferences and the session then last for this page only.
function browserStorage(): SessionDeps["storage"] {
  try {
    return window.localStorage;
  } catch {
    const values = new Map<string, string>();
    return {
      getItem: (key) => values.get(key) ?? null,
      setItem: (key, value) => void values.set(key, value),
      removeItem: (key) => void values.delete(key),
    };
  }
}
