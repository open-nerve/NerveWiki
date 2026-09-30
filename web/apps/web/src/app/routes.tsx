import type { RouteObject } from "react-router";

import { GuestOnly, Onboarded, SignedIn } from "./guards";
import { Layout } from "./layout";
import { RouteError } from "./route-error";

// The app's pages. Each is loaded on first visit, as its own chunk; the
// import names what it takes, so knip still sees which exports are used.
//
// A page's error shows inside the layout, by the error boundary of the
// pathless route around the pages; an error of the layout itself replaces
// it. While the first page's chunk loads, the layout shows with nothing
// below it: without a HydrateFallback, React Router also warns on the
// console on every first load. A path that is no page gets the app's 404:
// the server answers index.html for every page path.
//
// Every page but sign-in and sign-up needs a signed-in session, the 404
// too: an unknown path behaves as the pages M2 and M3 add (M1/P5 design
// 3.5). The guards alone decide where the tab goes as its session changes.
export const routes: RouteObject[] = [
  {
    Component: Layout,
    ErrorBoundary: RouteError,
    children: [
      {
        ErrorBoundary: RouteError,
        HydrateFallback: () => null,
        children: [
          {
            Component: GuestOnly,
            children: [
              {
                path: "sign-in",
                lazy: async () => {
                  const { SignInPage } = await import("../pages/sign-in");
                  return { Component: SignInPage };
                },
              },
              {
                path: "sign-up",
                lazy: async () => {
                  const { SignUpPage } = await import("../pages/sign-up");
                  return { Component: SignUpPage };
                },
              },
            ],
          },
          {
            Component: SignedIn,
            children: [
              {
                path: "onboarding",
                lazy: async () => {
                  const { OnboardingPage } = await import("../pages/onboarding");
                  return { Component: OnboardingPage };
                },
              },
              {
                Component: Onboarded,
                children: [
                  {
                    index: true,
                    lazy: async () => {
                      const { HomePage } = await import("../pages/home");
                      return { Component: HomePage };
                    },
                  },
                  {
                    path: "*",
                    lazy: async () => {
                      const { NotFoundPage } = await import("../pages/not-found");
                      return { Component: NotFoundPage };
                    },
                  },
                ],
              },
            ],
          },
        ],
      },
    ],
  },
];
