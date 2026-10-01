import { Navigate, type RouteObject } from "react-router";

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
//
// A workspace's pages are under its slug, beside the app's own top-level
// pages: the slugs that name these are reserved (M2/P1 design 3.6; the
// test of reserved-slugs.test.ts). / lands on a workspace (M2/P5 design
// 3.3).
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
                      const { LandingPage } = await import("../pages/landing");
                      return { Component: LandingPage };
                    },
                  },
                  {
                    path: "create-workspace",
                    lazy: async () => {
                      const { CreateWorkspacePage } = await import("../pages/create-workspace");
                      return { Component: CreateWorkspacePage };
                    },
                  },
                  {
                    path: "settings",
                    lazy: async () => {
                      const { SettingsLayout } = await import("../pages/settings/settings-layout");
                      return { Component: SettingsLayout };
                    },
                    children: [
                      { index: true, Component: () => <Navigate replace to="/settings/profile" /> },
                      {
                        path: "profile",
                        lazy: async () => {
                          const { ProfilePage } = await import("../pages/settings/profile-page");
                          return { Component: ProfilePage };
                        },
                      },
                      {
                        path: "security",
                        lazy: async () => {
                          const { SecurityPage } = await import("../pages/settings/security-page");
                          return { Component: SecurityPage };
                        },
                      },
                      {
                        path: "tokens",
                        lazy: async () => {
                          const { TokensPage } = await import("../pages/settings/tokens-page");
                          return { Component: TokensPage };
                        },
                      },
                    ],
                  },
                  {
                    path: ":slug",
                    lazy: async () => {
                      const { WorkspaceLayout } = await import("../pages/workspace/workspace-layout");
                      return { Component: WorkspaceLayout };
                    },
                    children: [
                      {
                        index: true,
                        lazy: async () => {
                          const { WorkspaceHomePage } = await import("../pages/workspace/workspace-home");
                          return { Component: WorkspaceHomePage };
                        },
                      },
                      {
                        path: "settings",
                        lazy: async () => {
                          const { WorkspaceSettingsLayout } = await import("../pages/workspace/settings-layout");
                          return { Component: WorkspaceSettingsLayout };
                        },
                        children: [
                          { index: true, Component: () => <Navigate replace to="general" /> },
                          {
                            path: "general",
                            lazy: async () => {
                              const { GeneralPage } = await import("../pages/workspace/general-page");
                              return { Component: GeneralPage };
                            },
                          },
                        ],
                      },
                    ],
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
