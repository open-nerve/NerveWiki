import { act, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { instanceJSON, json, signedInApp, workspaceJSON } from "../../test/fakes";
import { renderApp } from "../../test/render";

// The shell of a workspace's pages (M2/P5 design 3.2).

const acme = { ...workspaceJSON, id: "0199a2b4-0000-7000-8000-0000000000a1", slug: "acme", name: "Acme" };
const nameField = () => screen.findByLabelText<HTMLInputElement>("Name");
const withWorkspaces = (routes = {}) =>
  signedInApp({ "GET /api/v0/workspaces": () => json({ data: [acme, workspaceJSON] }), ...routes });

test("the switcher lists the account's workspaces, checks the one shown, and goes to another", async () => {
  const user = userEvent.setup();
  const { router } = renderApp("/lab", withWorkspaces());

  await user.click(await screen.findByRole("button", { name: "Lab", description: "Switch workspace" }));
  const items = await screen.findAllByRole("menuitemradio");
  expect(items.map((item) => [item.textContent, item.getAttribute("aria-checked")])).toEqual([
    ["Acme", "false"],
    ["Lab", "true"],
  ]);
  await user.click(screen.getByRole("menuitemradio", { name: "Acme" }));

  expect(await screen.findByRole("heading", { name: "Acme" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/acme");
});

test.each([
  [true, ["Create workspace"]],
  [false, []],
])("with creation %s, the switcher offers to create a workspace: %j", async (enabled, offered) => {
  const user = userEvent.setup();
  renderApp(
    "/lab",
    withWorkspaces({ "GET /api/v0/instance": () => json({ ...instanceJSON, workspace_creation_enabled: enabled }) })
  );

  await user.click(await screen.findByRole("button", { name: "Lab" }));
  await screen.findAllByRole("menuitemradio");

  const items = screen.queryAllByRole("menuitem");
  expect(items.map((item) => item.textContent)).toEqual(offered);
  expect(items.map((item) => item.getAttribute("href"))).toEqual(offered.map(() => "/create-workspace"));
});

test("a workspace shown is the one the device lands on next", async () => {
  const app = withWorkspaces();
  renderApp("/acme", app);

  expect(await screen.findByRole("heading", { name: "Acme" })).toBeTruthy();
  expect(app.preferences.lastWorkspace()).toBe("acme");
});

// Only an arrival moves the focus (M3/P4 design 3.5): a page opened, or
// gone to from the switcher, leaves it where the browser puts it.
test("a workspace's home opened, or chosen in the switcher, does not take the focus", async () => {
  const user = userEvent.setup();
  renderApp("/lab", withWorkspaces());

  const lab = await screen.findByRole("heading", { name: "Lab" });
  expect(document.activeElement).not.toBe(lab);
  await user.click(screen.getByRole("button", { name: "Lab", description: "Switch workspace" }));
  await user.click(await screen.findByRole("menuitemradio", { name: "Acme" }));

  const chosen = await screen.findByRole("heading", { name: "Acme" });
  expect(document.activeElement).not.toBe(chosen);
});

test("a slug the account has no workspace of is not found, nor remembered", async () => {
  const app = withWorkspaces();
  renderApp("/zeta", app);

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
  expect(screen.queryByRole("navigation", { name: "Zeta" })).toBeNull();
  expect(screen.queryByRole("button", { description: "Switch workspace" })).toBeNull();
  expect(app.preferences.lastWorkspace()).toBeUndefined();
});

// The left column is no landmark of its own: the workspace's navigation is
// the one it holds (M3/P4 design 3.3).
test("the navigation leads to the workspace's pages, marking the one shown", async () => {
  renderApp("/acme", withWorkspaces());

  const nav = await screen.findByRole("navigation", { name: "Acme" });
  expect(screen.queryByRole("complementary")).toBeNull();
  // The switcher too is in the navigation, a landmark's.
  expect(within(nav).getByRole("button", { name: "Acme", description: "Switch workspace" })).toBeTruthy();
  const links = within(nav).getAllByRole("link");
  expect(links.map((link) => [link.textContent, link.getAttribute("href"), link.getAttribute("aria-current")])).toEqual(
    [
      ["Home", "/acme", "page"],
      ["Settings", "/acme/settings", null],
    ]
  );
});

test("a page of one workspace starts anew in another: a name typed for one is not offered to another", async () => {
  const user = userEvent.setup();
  const { router } = renderApp("/acme/settings/general", withWorkspaces());
  await user.clear(await nameField());
  await user.type(await nameField(), "Acme Two");

  await act(() => router.navigate("/lab/settings/general"));

  expect((await nameField()).value).toBe("Lab");
});

afterEach(() => vi.useRealTimers());

test("a workspace a later read no longer has is not found", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  let list = [acme, workspaceJSON];
  const { router } = renderApp("/acme", signedInApp({ "GET /api/v0/workspaces": () => json({ data: list }) }));
  expect(await screen.findByRole("heading", { name: "Acme" })).toBeTruthy();

  // Removed elsewhere; coming back to it reads the list again, once the read before is no longer recent.
  list = [workspaceJSON];
  await act(() => router.navigate("/create-workspace"));
  await act(() => vi.advanceTimersByTimeAsync(2_000));
  await act(() => router.navigate("/acme"));

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
});

test("the shell says so when the workspaces cannot be loaded; Try again loads them", async () => {
  const user = userEvent.setup();
  let down = true;
  renderApp(
    "/acme",
    signedInApp({
      "GET /api/v0/workspaces": () => (down ? Promise.reject(new TypeError("offline")) : json({ data: [acme] })),
    })
  );
  expect((await screen.findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );

  down = false;
  await user.click(screen.getByRole("button", { name: "Try again" }));

  expect(await screen.findByRole("heading", { name: "Acme" })).toBeTruthy();
});
