import "./styles.css";

import { createClient } from "@nervewiki/api-client";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { createBrowserRouter, RouterProvider } from "react-router";

import { AppProviders } from "./app/providers";
import { routes } from "./app/routes";
import { PreferencesStore, type PreferenceStorage } from "./stores/preferences.store";
import { RootStore } from "./stores/root.store";

const root = document.getElementById("root");
if (!root) {
  throw new Error("index.html has no #root element");
}
const preferences = new PreferencesStore({
  storage: browserStorage(),
  darkScheme: window.matchMedia("(prefers-color-scheme: dark)"),
  languages: navigator.languages,
});
// The API is on the page's own origin.
const store = new RootStore(preferences, createClient());
createRoot(root).render(
  <StrictMode>
    <AppProviders store={store}>
      <RouterProvider router={createBrowserRouter(routes)} />
    </AppProviders>
  </StrictMode>
);

// Reading window.localStorage throws where the browser blocks site data; the
// preferences then last for this page only.
function browserStorage(): PreferenceStorage {
  try {
    return window.localStorage;
  } catch {
    const values = new Map<string, string>();
    return { getItem: (key) => values.get(key) ?? null, setItem: (key, value) => void values.set(key, value) };
  }
}
