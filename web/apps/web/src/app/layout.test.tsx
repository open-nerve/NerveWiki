import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";

import { json, problem, signedInApp, testApp, tokensJSON, userJSON, workspaceJSON, type Answer } from "../test/fakes";
import { renderApp } from "../test/render";

/** held is an answer kept until released; asked settles once it is asked for. */
function held() {
  const waiting: ((response: Response) => void)[] = [];
  const askers: (() => void)[] = [];
  const asked = new Promise<void>((resolve) => {
    askers.push(resolve);
  });
  const answer: Answer = () =>
    new Promise<Response>((resolve) => {
      waiting.push(resolve);
      for (const ask of askers) ask();
    });
  return { answer, asked, release: (response: Response) => waiting.forEach((resolve) => resolve(response)) };
}

test("choosing a theme in the account's Appearance applies it to the page, the menu staying open", async () => {
  const user = userEvent.setup();
  const { app } = renderApp("/lab", signedInApp());
  await screen.findByRole("heading", { name: "Lab" });
  expect(document.documentElement.classList.contains("dark")).toBe(false);

  await user.click(screen.getByRole("button", { name: "Ada" }));
  await user.click(await screen.findByRole("menuitem", { name: "Appearance" }));
  // Clicked, not moved to: jsdom lays nothing out, and Radix takes the pointer leaving the submenu's item for a
  // pointer going elsewhere, which closes the submenu, where a browser sees it go towards it.
  fireEvent.click(await screen.findByRole("menuitemradio", { name: "Dark" }));

  expect(app.preferences.theme).toBe("dark");
  expect(document.documentElement.classList.contains("dark")).toBe(true);
  // Chosen in place (M13/P2/S1, decision 770): the menu is still open, for the language too.
  expect(screen.getByRole("menuitem", { name: "Appearance" })).toBeTruthy();
});

test("a workspace's left column is beside the one main, which holds its page (M3 handoff 1)", async () => {
  renderApp("/lab", signedInApp());
  const heading = await screen.findByRole("heading", { name: "Lab" });
  // getByRole finds one or fails.
  const main = screen.getByRole("main");
  expect(main.contains(heading)).toBe(true);
  const nav = screen.getByRole("navigation", { name: "Lab" });
  expect(main.contains(nav)).toBe(false);
  expect(nav.compareDocumentPosition(main) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
});

test("a page without a shell is the one main", async () => {
  renderApp("/create-workspace", signedInApp());
  const heading = await screen.findByRole("heading", { level: 1 });
  expect(screen.getByRole("main").contains(heading)).toBe(true);
});

test("a workspace's 404 is in the main, without the left column", async () => {
  renderApp("/nowhere", signedInApp());
  const heading = await screen.findByRole("heading", { name: "Page not found" });
  expect(screen.getByRole("main").contains(heading)).toBe(true);
  expect(screen.queryByRole("navigation", { name: "Lab" })).toBeNull();
});

// The top right (M13/P2/S1, decisions 769, 792): where no left column is, the account's button, or Appearance while
// the tab has no session to show; nothing while the session or the account loads.

test("signed out, the top right has Appearance alone, the theme and the language in its menu", async () => {
  const user = userEvent.setup();
  renderApp("/sign-in", testApp());
  await screen.findByRole("heading", { name: "Sign in" });

  const appearance = screen.getByRole("button", { name: "Appearance" });
  expect(screen.queryByRole("button", { name: "Ada" })).toBeNull();
  await user.click(appearance);
  const menu = await screen.findByRole("menu");
  expect(within(menu).getByRole("group", { name: "Theme" })).toBeTruthy();
  expect(within(menu).getByRole("group", { name: "Language" })).toBeTruthy();
  expect(
    within(menu)
      .getAllByRole("menuitemradio")
      .map((item) => item.textContent)
  ).toEqual(["System", "Light", "Dark", "简体中文", "English"]);
});

test("a workspace's page has one account's button, in the left column after its navigation", async () => {
  renderApp("/lab", signedInApp());
  await screen.findByRole("heading", { name: "Lab" });

  // getByRole finds one or fails.
  const account = screen.getByRole("button", { name: "Ada" });
  const column = account.closest("[data-shell]");
  expect(column).not.toBeNull();
  const nav = screen.getByRole("navigation", { name: "Lab" });
  expect(column?.contains(nav)).toBe(true);
  expect(nav.compareDocumentPosition(account) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Appearance" })).toBeNull();
});

test("while the workspaces load there is no account's button; a slug not found then has it at the top right, outside the column", async () => {
  const workspaces = held();
  renderApp("/nowhere", signedInApp({ "GET /api/v0/workspaces": workspaces.answer }));
  // Asked for once the account has loaded: only the workspaces are missing (review C-M4).
  await workspaces.asked;
  expect(await screen.findByRole("status")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Ada" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Appearance" })).toBeNull();

  workspaces.release(json({ data: [workspaceJSON] }));
  await screen.findByRole("heading", { name: "Page not found" });
  const account = await screen.findByRole("button", { name: "Ada" });
  expect(account.closest("[data-shell]")).toBeNull();
  // Back to the workspace is the settings', the creating page's and the signed-in invitation's (review C, S1C-N3;
  // M13/P5/S1, decision 941).
  expect(screen.queryByRole("link", { name: "Back to the workspace" })).toBeNull();
});

test("at / the account's button waits for the workspaces, then is the column's, one of it", async () => {
  const workspaces = held();
  renderApp("/", signedInApp({ "GET /api/v0/workspaces": workspaces.answer }));
  // Where / goes has a left column, or is the page that creates a workspace: no button at the top right that would
  // go as the column comes (review A, S1A-M1).
  await workspaces.asked;
  expect(await screen.findByRole("status")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Ada" })).toBeNull();

  workspaces.release(json({ data: [workspaceJSON] }));
  await screen.findByRole("heading", { name: "Lab" });
  const accounts = screen.getAllByRole("button", { name: "Ada" });
  expect(accounts).toHaveLength(1);
  expect(accounts[0]?.closest("[data-shell]")).not.toBeNull();
});

test("at / the workspaces' load failing, the account's button is at the top right", async () => {
  renderApp("/", signedInApp({ "GET /api/v0/workspaces": () => problem(500, "internal") }));

  await screen.findByRole("button", { name: "Try again" });
  const account = await screen.findByRole("button", { name: "Ada" });
  expect(account.closest("[data-shell]")).toBeNull();
});

test("the workspaces' load failing, the account's button is at the top right", async () => {
  renderApp("/lab", signedInApp({ "GET /api/v0/workspaces": () => problem(500, "internal") }));

  await screen.findByRole("button", { name: "Try again" });
  const account = await screen.findByRole("button", { name: "Ada" });
  expect(account.closest("[data-shell]")).toBeNull();
});

test("from a slug not found back home, there is one account's button, the column's", async () => {
  const user = userEvent.setup();
  renderApp("/nowhere", signedInApp());
  await screen.findByRole("heading", { name: "Page not found" });
  expect(screen.getByRole("button", { name: "Ada" }).closest("[data-shell]")).toBeNull();

  await user.click(screen.getByRole("link", { name: "Go to the home page" }));

  // The 404 gave the top right back as it went (review C-M5).
  await screen.findByRole("heading", { name: "Lab" });
  const accounts = screen.getAllByRole("button", { name: "Ada" });
  expect(accounts).toHaveLength(1);
  expect(accounts[0]?.closest("[data-shell]")).not.toBeNull();
});

test("the settings have a left column, their directory, the account in its foot, no top right; its Back lands on the workspace, arrived at: its title takes the focus", async () => {
  const user = userEvent.setup();
  const app = signedInApp();
  app.preferences.setLastWorkspace("lab");
  // Any page of theirs, not the profile alone (review A, S1A-N4, r28).
  const { router } = renderApp("/settings/security", app);
  await screen.findByRole("heading", { level: 1, name: "Security" });
  const accounts = screen.getAllByRole("button", { name: "Ada" });
  expect(accounts).toHaveLength(1);
  expect(accounts[0]?.closest("[data-shell]")).not.toBeNull();
  expect(screen.queryByRole("link", { name: "Back to the workspace" })).toBeNull();

  // Opened at its address, Back goes to the workspace this device showed last (M13/P3/S1, decision 813).
  await user.click(await screen.findByRole("link", { name: "Back to Lab" }));

  const lab = await screen.findByRole("heading", { name: "Lab" });
  expect(router.state.location.pathname).toBe("/lab");
  await waitFor(() => expect(document.activeElement).toBe(lab));
});

test("on the page that creates a workspace, Back to the workspace too, the account having one; arrived at", async () => {
  const user = userEvent.setup();
  const { router } = renderApp("/create-workspace", signedInApp());
  await screen.findByRole("heading", { level: 1, name: "Create a workspace" });

  // Gone there from the switcher, and not creating one after all (decision 803; review A, S1A-Q1).
  await user.click(await screen.findByRole("link", { name: "Back to the workspace" }));

  const lab = await screen.findByRole("heading", { name: "Lab" });
  expect(router.state.location.pathname).toBe("/lab");
  await waitFor(() => expect(document.activeElement).toBe(lab));
});

test("an account without a workspace, landed on the page that creates one, has no Back to the workspace", async () => {
  const { router } = renderApp("/", signedInApp({ "GET /api/v0/workspaces": () => json({ data: [] }) }));

  await screen.findByRole("heading", { level: 1, name: "Create a workspace" });
  expect(router.state.location.pathname).toBe("/create-workspace");
  expect(await screen.findByRole("button", { name: "Ada" })).toBeTruthy();
  // / would land it here again.
  expect(screen.queryByRole("link", { name: "Back to the workspace" })).toBeNull();
});

test("while the session starts the top right has nothing, then the account's button", async () => {
  const refresh = held();
  let reads = 0;
  renderApp(
    // A page without a left column, whose top right is there as the session starts (review C, P3C-I5).
    "/create-workspace",
    signedInApp({
      "POST /api/v0/auth/refresh": refresh.answer,
      "GET /api/v0/me": () => {
        reads += 1;
        return json(userJSON);
      },
    })
  );
  // The guard's Loading shows: the page is up, its session starting (review A-M4).
  await refresh.asked;
  expect(await screen.findByRole("status")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Appearance" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Ada" })).toBeNull();

  refresh.release(json(tokensJSON));

  expect(await screen.findByRole("button", { name: "Ada" })).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Appearance" })).toBeNull();
  // The guard and the top right waited for the account by one key: one read (review C, S1C-N3).
  expect(reads).toBe(1);
});

test("the account not loading, the session's unavailable page has Appearance at the top right", async () => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  renderApp("/lab", signedInApp({ "GET /api/v0/me": () => problem(500, "internal") }));

  expect(await screen.findByRole("heading", { name: "Cannot reach the server" })).toBeTruthy();
  expect(await screen.findByRole("button", { name: "Appearance" })).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Ada" })).toBeNull();
});

// A page without a left column, and one with (review C, P3C-I5).
test.each(["/create-workspace", "/lab"])(
  "the session unavailable at %s, the top right has Appearance, no account's button",
  async (path) => {
    // The refresh refused for now: the tab has a session it cannot reach (decision 792; review C, S1C-M1).
    renderApp(path, signedInApp({ "POST /api/v0/auth/refresh": () => problem(503, "server_busy") }));

    expect(await screen.findByRole("heading", { name: "Cannot reach the server" })).toBeTruthy();
    expect(await screen.findByRole("button", { name: "Appearance" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Ada" })).toBeNull();
  }
);

test("in Chinese, the top right's Appearance and Back to the workspace", async () => {
  const signedOut = testApp();
  signedOut.preferences.setLocale("zh-CN");
  const { unmount } = renderApp("/sign-in", signedOut);
  expect(await screen.findByRole("button", { name: "外观" })).toBeTruthy();
  unmount();

  const signedIn = signedInApp();
  signedIn.preferences.setLocale("zh-CN");
  renderApp("/create-workspace", signedIn);
  expect(await screen.findByRole("link", { name: "回到工作区" })).toBeTruthy();
});
