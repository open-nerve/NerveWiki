import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { instanceJSON, json, signedInApp, workspaceJSON } from "../../test/fakes";
import { renderApp } from "../../test/render";

// The shell of a workspace's pages (M2/P5 design 3.2).

const acme = { ...workspaceJSON, id: "0199a2b4-0000-7000-8000-0000000000a1", slug: "acme", name: "Acme" };
const withWorkspaces = (routes = {}) =>
  signedInApp({ "GET /api/v0/workspaces": () => json({ data: [acme, workspaceJSON] }), ...routes });

test("the switcher lists the account's workspaces, checks the one shown, and goes to another", async () => {
  const user = userEvent.setup();
  const { router } = renderApp("/lab", withWorkspaces());

  await user.click(await screen.findByRole("button", { name: "Lab" }));
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

test("a slug the account has no workspace of is not found, nor remembered", async () => {
  const app = withWorkspaces();
  renderApp("/zeta", app);

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
  expect(screen.queryByRole("complementary", { name: "Workspace" })).toBeNull();
  expect(app.preferences.lastWorkspace()).toBeUndefined();
});

test("the navigation leads to the workspace's pages, marking the one shown", async () => {
  renderApp("/acme", withWorkspaces());

  const nav = await screen.findByRole("navigation", { name: "Acme" });
  const links = within(nav).getAllByRole("link");
  expect(links.map((link) => [link.textContent, link.getAttribute("href"), link.getAttribute("aria-current")])).toEqual(
    [
      ["Home", "/acme", "page"],
      ["Settings", "/acme/settings", null],
    ]
  );
});
