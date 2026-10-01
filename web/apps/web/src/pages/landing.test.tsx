import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { json, signedInApp, workspaceJSON } from "../test/fakes";
import { renderApp } from "../test/render";

// Where / lands (M2/P5 design 3.3).

const acme = { ...workspaceJSON, id: "0199a2b4-0000-7000-8000-0000000000a1", slug: "acme", name: "Acme" };
const withWorkspaces = (...list: (typeof workspaceJSON)[]) =>
  signedInApp({ "GET /api/v0/workspaces": () => json({ data: list }) });

test("/ lands on the workspace this device showed last", async () => {
  const app = withWorkspaces(acme, workspaceJSON);
  app.preferences.setLastWorkspace("lab");
  const { router } = renderApp("/", app);

  expect(await screen.findByRole("heading", { name: "Lab" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/lab");
});

test("/ lands on the first workspace by name when the last one is not the account's", async () => {
  const app = withWorkspaces(acme, workspaceJSON);
  app.preferences.setLastWorkspace("gone");
  const { router } = renderApp("/", app);

  expect(await screen.findByRole("heading", { name: "Acme" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/acme");
});

test("/ lands on the creation page without a workspace", async () => {
  const { router } = renderApp("/", withWorkspaces());

  expect(await screen.findByRole("heading", { name: "Create a workspace" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/create-workspace");
});

test("/ says so when the workspaces cannot be loaded; Try again loads them", async () => {
  const user = userEvent.setup();
  let down = true;
  const { router } = renderApp(
    "/",
    signedInApp({
      "GET /api/v0/workspaces": () =>
        down ? Promise.reject(new TypeError("offline")) : json({ data: [workspaceJSON] }),
    })
  );
  expect((await screen.findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );

  down = false;
  await user.click(screen.getByRole("button", { name: "Try again" }));

  expect(await screen.findByRole("heading", { name: "Lab" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/lab");
});
