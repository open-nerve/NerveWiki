import type { RouteObject } from "react-router";

import { Layout } from "./layout";

// The app's pages, inside the layout. Each page is loaded on first visit, as
// its own chunk; the import names what it takes, so knip still sees which
// exports are used.
export const routes: RouteObject[] = [
  {
    Component: Layout,
    children: [
      {
        index: true,
        lazy: async () => {
          const { HomePage } = await import("../pages/home");
          return { Component: HomePage };
        },
      },
    ],
  },
];
