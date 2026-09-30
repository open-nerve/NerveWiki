import type { RouteObject } from "react-router";

// The app's pages. Each one is loaded on first visit, as its own chunk; the
// import names what it takes, so knip still sees which exports are used.
export const routes: RouteObject[] = [
  {
    index: true,
    lazy: async () => {
      const { HomePage } = await import("../pages/home");
      return { Component: HomePage };
    },
  },
];
