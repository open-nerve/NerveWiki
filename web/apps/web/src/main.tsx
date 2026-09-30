import "./styles.css";

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { createBrowserRouter, RouterProvider } from "react-router";

import { routes } from "./app/routes";
import { StoreProvider } from "./stores/context";
import { PreferencesStore, type PreferenceStorage } from "./stores/preferences.store";
import { RootStore } from "./stores/root.store";

const root = document.getElementById("root");
if (!root) {
  throw new Error("index.html has no #root element");
}
const preferences = new PreferencesStore(browserStorage(), window.matchMedia("(prefers-color-scheme: dark)"));
createRoot(root).render(
  <StrictMode>
    <StoreProvider store={new RootStore(preferences)}>
      <RouterProvider router={createBrowserRouter(routes)} />
    </StoreProvider>
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
