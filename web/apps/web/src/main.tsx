import "./styles.css";

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { createBrowserRouter, RouterProvider } from "react-router";

import { AppProviders } from "./app/providers";
import { routes } from "./app/routes";
import { browserSessionDeps, Session, type SessionDeps } from "./session/session";
import { PreferencesStore } from "./stores/preferences.store";
import { RootStore } from "./stores/root.store";

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
const session = new Session(browserSessionDeps(storage));
void session.start();
const store = new RootStore(preferences, session.public);
createRoot(root).render(
  <StrictMode>
    <AppProviders store={store}>
      <RouterProvider router={createBrowserRouter(routes)} />
    </AppProviders>
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
