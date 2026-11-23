import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import {
  byRoute,
  instanceJSON,
  json,
  notebookJSON,
  problem,
  signedInApp,
  testApp,
  tokensJSON,
  userJSON,
  workspaceJSON,
} from "../test/fakes";
import { notebookServer } from "../test/notebook-server";
import { guide, pagePath, pageServer } from "../test/page-server";
import { renderApp } from "../test/render";
import { arrived } from "./arrival";

// Where the settings were entered from (M13/P3/S1, decisions 812, 813, 833, 834): Layout records it as the address
// enters the settings, and the directory's Back goes there.

/** The directory's Back: a link of the left column, "Back" or "Back to …". */
function back(): HTMLAnchorElement {
  const column = document.querySelector<HTMLElement>("[data-shell]");
  if (column === null) {
    throw new Error("no left column");
  }
  return within(column).getByRole("link", { name: /^Back(?: to .+)?$/ });
}

/** Waits for Back to say text and to lead to href. */
async function expectBack(text: string, href: string): Promise<void> {
  await waitFor(() => expect([back().textContent, back().getAttribute("href")]).toEqual([text, href]));
}

const home = `/lab/notebooks/${notebookJSON.id}`;

test("entered from a workspace's home, its settings go Back there, the origin kept through the account's and a notebook's", async () => {
  const { router } = renderApp("/lab", notebookServer().app);
  await screen.findByRole("heading", { level: 1, name: "Lab" });

  await act(() => router.navigate("/lab/settings/general", { state: arrived }));
  await screen.findByRole("heading", { level: 1, name: "Workspace settings" });
  await expectBack("Back to Lab", "/lab");

  // Across to the account's settings, then to a notebook's: the directory mounts anew, the origin stays.
  await act(() => router.navigate("/settings/profile", { state: arrived }));
  await screen.findByRole("heading", { level: 1, name: "Profile" });
  await expectBack("Back to Lab", "/lab");
  await act(() => router.navigate(`${home}/settings/members`, { state: arrived }));
  await screen.findByRole("heading", { level: 1, name: "Plans settings" });
  await expectBack("Back to Lab", "/lab");
});

test("left and entered again from a search, the settings go Back to the search, its query kept", async () => {
  const { router } = renderApp("/lab", notebookServer().app);
  await screen.findByRole("heading", { level: 1, name: "Lab" });
  await act(() => router.navigate("/lab/settings/general", { state: arrived }));
  await expectBack("Back to Lab", "/lab");
  await act(() => router.navigate("/lab"));
  await screen.findByRole("heading", { level: 1, name: "Lab" });

  await act(() => router.navigate("/lab/search?q=a"));
  await act(() => router.navigate("/lab/settings/members", { state: arrived }));

  await expectBack("Back to Lab", "/lab/search?q=a");
});

test("opened at their address, the tab's first, the settings have no origin: Back goes to the workspace's home; the index's redirect keeps an origin", async () => {
  const { router } = renderApp("/lab/settings", notebookServer().app);
  await screen.findByRole("heading", { level: 1, name: "Workspace settings" });
  expect(router.state.location.pathname).toBe("/lab/settings/general");
  await expectBack("Back to Lab", "/lab");

  await act(() => router.navigate("/lab/search?q=a"));
  // The index replaces itself with the first page: a replace inside the settings, the origin taken before it.
  await act(() => router.navigate("/lab/settings", { state: arrived }));
  await waitFor(() => expect(router.state.location.pathname).toBe("/lab/settings/general"));
  await expectBack("Back to Lab", "/lab/search?q=a");
});

// Entered by a replace, the settings have no origin (decision 833): the page replaced would send them back at once.

test("a settings address opened signed out, then signed in through the guard's replace: Back to the workspace, not the sign-in page", async () => {
  const user = userEvent.setup();
  const app = testApp(
    byRoute({
      "GET /api/v0/instance": () => json(instanceJSON),
      "POST /api/v0/auth/login": () => json(tokensJSON),
      "GET /api/v0/me": () => json(userJSON),
      "GET /api/v0/workspaces": () => json({ data: [workspaceJSON] }),
    })
  );
  app.preferences.setLastWorkspace("lab");
  const { router } = renderApp("/settings/security", app);
  await user.type(await screen.findByLabelText("E-mail address"), "ada@example.com");
  await user.type(screen.getByLabelText("Password"), "correct horse");
  await user.click(screen.getByRole("button", { name: "Sign in" }));

  await waitFor(() => expect(router.state.location.pathname).toBe("/settings/security"));
  await screen.findByRole("heading", { level: 1, name: "Security" });
  await expectBack("Back to Lab", "/lab");
});

test("the onboarding's next a settings page, gone to by its replace: Back does not go to the onboarding", async () => {
  const { router } = renderApp("/onboarding?next=%2Fsettings%2Fprofile", signedInApp());

  await waitFor(() => expect(router.state.location.pathname).toBe("/settings/profile"));
  await screen.findByRole("heading", { level: 1, name: "Profile" });
  // No workspace this device showed last either: Back alone, to /.
  await expectBack("Back", "/");
});

// The origin's fragment (decision 834): an invitation's token is in it; in a workspace, its page's h1 takes the arrival
// focus, the anchor's element would take it before or after, so it goes.

test("entered from the invitation's page, Back goes there with its fragment: the invitation shows, its title focused", async () => {
  const user = userEvent.setup();
  const { router } = renderApp(
    "/invitations/inv-1#tok",
    signedInApp({
      "POST /api/v0/workspace-invitations/inv-1/preview": () =>
        json({ workspace: { name: "Acme", slug: "acme" }, role: "member" }),
    })
  );
  await screen.findByRole("heading", { level: 1, name: "Invitation" });
  await screen.findByText(/Acme/);

  await act(() => router.navigate("/settings/profile", { state: arrived }));
  await screen.findByRole("heading", { level: 1, name: "Profile" });
  await expectBack("Back", "/invitations/inv-1#tok");
  await user.click(back());

  await waitFor(() => expect(router.state.location.hash).toBe("#tok"));
  const title = await screen.findByRole("heading", { level: 1, name: "Invitation" });
  expect(await screen.findByText(/Acme/)).toBeTruthy();
  expect(screen.queryByText(/This invitation no longer works/)).toBeNull();
  await waitFor(() => expect(document.activeElement).toBe(title));
});

test("entered from a page at one of its anchors, Back goes to the page without it", async () => {
  const { router } = renderApp(`${pagePath(guide.id)}#h`, pageServer().app);
  await screen.findByRole("heading", { level: 1, name: "Guide" });

  await act(() => router.navigate("/settings/profile", { state: arrived }));

  await screen.findByRole("heading", { level: 1, name: "Profile" });
  await expectBack("Back to Plans", pagePath(guide.id));
});

test("entered from a workspace the list read has not (its 404), Back goes to /", async () => {
  const { router } = renderApp("/lab", signedInApp({ "GET /api/v0/workspaces": () => json({ data: [] }) }));
  await screen.findByRole("heading", { level: 1, name: "Page not found" });

  await act(() => router.navigate("/settings/profile", { state: arrived }));

  await screen.findByRole("heading", { level: 1, name: "Profile" });
  await expectBack("Back", "/");
});

test("entered through the top right's account menu from a workspace whose list cannot be read, Back goes to the page it was on, unnamed (review A's N2)", async () => {
  const user = userEvent.setup();
  const { router } = renderApp("/lab", signedInApp({ "GET /api/v0/workspaces": () => problem(503, "server_busy") }));
  await screen.findByRole("button", { name: "Try again" });

  await user.click(await screen.findByRole("button", { name: "Ada" }));
  await user.click(await screen.findByRole("menuitem", { name: "Settings" }));

  await screen.findByRole("heading", { level: 1, name: "Profile" });
  expect(router.state.location.pathname).toBe("/settings/profile");
  // Not known gone, the workspace not known either: Back to where the account was, not to /.
  await expectBack("Back", "/lab");
});

test("left for another page and come back to by Back two entries, the settings go Back to the page just left", async () => {
  const { router } = renderApp("/lab", notebookServer().app);
  await screen.findByRole("heading", { level: 1, name: "Lab" });
  await act(() => router.navigate("/lab/settings/general", { state: arrived }));
  await expectBack("Back to Lab", "/lab");

  await act(() => router.navigate("/lab/search?q=a"));
  await act(() => router.navigate(home));
  await screen.findByRole("heading", { level: 1, name: "Plans" });
  await act(() => router.navigate(-2));

  await screen.findByRole("heading", { level: 1, name: "Workspace settings" });
  await expectBack("Back to Plans", home);
});
