import { Navigate, useLocation, type RouteObject } from "react-router";

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
// Every page but sign-in, sign-up and an invitation's needs a signed-in
// session, the 404 too: an unknown path behaves as the pages M2 and M3 add
// (M1/P5 design 3.5). The guards alone decide where the tab goes as its
// session changes; the invitation page shows to any session (M2/P6 design
// 3.4).
//
// A workspace's pages are under its slug, beside the app's own top-level
// pages: the slugs that name these are reserved (M2/P1 design 3.6; the
// test of reserved-slugs.test.ts). / lands on a workspace (M2/P5 design
// 3.3). A notebook's pages are under its workspace's, by the notebook's id
// (M3/P4 design 3.4), and a wiki page's under its notebook's, by the
// page's id (M4/P5 design 3.5), as a tag's pages are by the tag (M6/P6
// design 8); a page's version under the page's, by its revision (M8
// design 4.3); the notebook's trash beside them, an item's preview under
// it by its root's id (M8 design 4.5), and its history, a changeset's page
// under it by its id (M8/P3 design 4.2, 4.3). A workspace's search is under
// its workspace's, beside its notebooks, its query in the address (M8/P5
// design 4.3).
//
// The account's, a workspace's and a notebook's settings are the settings'
// mode (handle: { settings: true }): the left column their directory
// (M13/P3/S1, 00-M13 4.4.1); the account's, outside any workspace's shell,
// draw that column themselves.

/**
 * IndexTo is a settings index's redirect to its first page, the state it was given passed on, as LandingPage passes
 * its own: arrived at, the first page's h1 takes the focus (M13/P3/S1, decision 816).
 */
function IndexTo({ to }: { to: string }) {
  const { state } = useLocation();
  return <Navigate replace to={to} state={state} />;
}

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
            // Outside the guards: the token is in the fragment, which a next would put in the address.
            path: "invitations/:id",
            lazy: async () => {
              const { InvitationPage } = await import("../pages/invitation");
              return { Component: InvitationPage };
            },
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
                    // It goes on to a workspace's, which has a left column (M13/P2/S1; review A, S1A-M1).
                    handle: { sidebar: true },
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
                    // The settings' mode, its column the directory with the account in its foot (M13/P3/S1, decision 811).
                    handle: { sidebar: true, settings: true },
                    lazy: async () => {
                      const { SettingsLayout } = await import("../pages/settings/settings-layout");
                      return { Component: SettingsLayout };
                    },
                    children: [
                      { index: true, Component: () => <IndexTo to="/settings/profile" /> },
                      {
                        path: "profile",
                        lazy: async () => {
                          const { ProfilePage } = await import("../pages/settings/profile-page");
                          return { Component: ProfilePage };
                        },
                      },
                      {
                        path: "appearance",
                        lazy: async () => {
                          const { AppearancePage } = await import("../pages/settings/appearance-page");
                          return { Component: AppearancePage };
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
                    // A left column, with the account in its foot: no top right (layout.tsx; M13/P2/S1, decision 769).
                    handle: { sidebar: true },
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
                        path: "notebooks/:id",
                        lazy: async () => {
                          const { NotebookLayout } = await import("../pages/notebook/notebook-layout");
                          return { Component: NotebookLayout };
                        },
                        children: [
                          {
                            index: true,
                            lazy: async () => {
                              const { NotebookHomePage } = await import("../pages/notebook/notebook-home");
                              return { Component: NotebookHomePage };
                            },
                          },
                          {
                            path: "pages/:pageId",
                            lazy: async () => {
                              const { PageLayout } = await import("../pages/page/page-layout");
                              return { Component: PageLayout };
                            },
                          },
                          {
                            // A version of a page, by a revision it holds (M8 design 4.3).
                            path: "pages/:pageId/versions/:revision",
                            lazy: async () => {
                              const { VersionPage } = await import("../pages/page/version-page");
                              return { Component: VersionPage };
                            },
                          },
                          {
                            // Outside a page's shell, which has the pages of the tree alone (M8 design 4.5).
                            path: "trash",
                            lazy: async () => {
                              const { TrashPage } = await import("../pages/notebook/trash-page");
                              return { Component: TrashPage };
                            },
                          },
                          {
                            path: "trash/:nodeId",
                            lazy: async () => {
                              const { TrashPreview } = await import("../pages/notebook/trash-preview");
                              return { Component: TrashPreview };
                            },
                          },
                          {
                            path: "index",
                            lazy: async () => {
                              const { IndexPage } = await import("../pages/notebook/index-page");
                              return { Component: IndexPage };
                            },
                          },
                          {
                            path: "health",
                            lazy: async () => {
                              const { HealthPage } = await import("../pages/notebook/health-page");
                              return { Component: HealthPage };
                            },
                          },
                          {
                            // The notebook's upgrade under way, its admins' (M11/P1 design 4.4).
                            path: "upgrade",
                            lazy: async () => {
                              const { UpgradePage } = await import("../pages/notebook/upgrade-page");
                              return { Component: UpgradePage };
                            },
                          },
                          {
                            // The notebook's history, its filters in the query (M8/P3 design 4.2).
                            path: "history",
                            lazy: async () => {
                              const { HistoryPage } = await import("../pages/notebook/history-page");
                              return { Component: HistoryPage };
                            },
                          },
                          {
                            path: "history/:changesetId",
                            lazy: async () => {
                              const { ChangesetPage } = await import("../pages/notebook/changeset-page");
                              return { Component: ChangesetPage };
                            },
                          },
                          {
                            // The tag is one segment, its '/' written %2F: a tag the page writes ending with '/'
                            // keeps it (M6/P6 design 8).
                            path: "tags/:tag",
                            lazy: async () => {
                              const { TagPage } = await import("../pages/notebook/tag-page");
                              return { Component: TagPage };
                            },
                          },
                          {
                            path: "settings",
                            // The settings' mode: the workspace's column their directory (M13/P3/S1, decision 811).
                            handle: { settings: true },
                            lazy: async () => {
                              const { NotebookSettingsLayout } = await import("../pages/notebook/settings-layout");
                              return { Component: NotebookSettingsLayout };
                            },
                            children: [
                              { index: true, Component: () => <IndexTo to="general" /> },
                              {
                                path: "general",
                                lazy: async () => {
                                  const { NotebookGeneralPage } = await import("../pages/notebook/general-page");
                                  return { Component: NotebookGeneralPage };
                                },
                              },
                              {
                                path: "members",
                                lazy: async () => {
                                  const { NotebookMembersPage } = await import("../pages/notebook/members-page");
                                  return { Component: NotebookMembersPage };
                                },
                              },
                              {
                                path: "transfer",
                                lazy: async () => {
                                  const { NotebookTransferPage } = await import("../pages/notebook/transfer-page");
                                  return { Component: NotebookTransferPage };
                                },
                              },
                            ],
                          },
                        ],
                      },
                      {
                        // Outside a notebook's shell: it searches them all, or one it names (M8/P5 design 4.3).
                        path: "search",
                        lazy: async () => {
                          const { SearchPage } = await import("../pages/search/search-page");
                          return { Component: SearchPage };
                        },
                      },
                      {
                        path: "settings",
                        // The settings' mode: the workspace's column their directory (M13/P3/S1, decision 811).
                        handle: { settings: true },
                        lazy: async () => {
                          const { WorkspaceSettingsLayout } = await import("../pages/workspace/settings-layout");
                          return { Component: WorkspaceSettingsLayout };
                        },
                        children: [
                          { index: true, Component: () => <IndexTo to="general" /> },
                          {
                            path: "general",
                            lazy: async () => {
                              const { GeneralPage } = await import("../pages/workspace/general-page");
                              return { Component: GeneralPage };
                            },
                          },
                          {
                            path: "members",
                            lazy: async () => {
                              const { MembersPage } = await import("../pages/workspace/members-page");
                              return { Component: MembersPage };
                            },
                          },
                          {
                            path: "ownerless",
                            lazy: async () => {
                              const { OwnerlessPage } = await import("../pages/workspace/ownerless-page");
                              return { Component: OwnerlessPage };
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
