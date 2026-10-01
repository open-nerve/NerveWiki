import { configure, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, beforeAll, expect, test } from "vitest";

import type { User } from "../services/account.service";
import type { Workspace, WorkspaceCreate } from "../services/workspace.service";
import { instanceJSON, json, problem, signedInApp, userJSON, workspaceJSON } from "../test/fakes";
import { renderApp } from "../test/render";

// Onboarding's workspace step (M2/P5 design 3.7), in StrictMode as the
// app runs: React runs a new component's effects twice there.
beforeAll(() => configure({ reactStrictMode: true }));
afterAll(() => configure({ reactStrictMode: false }));

/**
 * The server of an account done with the profile step, whose workspaces
 * are list (creations add to it); what changed the account or its
 * workspaces is in sent. While stepsDown, recording a step answers 503;
 * while workspacesDown, the workspaces cannot be read.
 */
function stepServer(list: Workspace[], creation = true) {
  const server = { sent: [] as string[], stepsDown: false, workspacesDown: false };
  let me: User = { ...userJSON, onboarding_steps: ["profile"] };
  const app = signedInApp({
    "GET /api/v0/me": () => json(me),
    "GET /api/v0/instance": () => json({ ...instanceJSON, workspace_creation_enabled: creation }),
    "GET /api/v0/workspaces": () =>
      server.workspacesDown ? Promise.reject(new TypeError("offline")) : json({ data: list }),
    "GET /api/v0/workspace-slugs/*": () => json({ available: true }),
    "POST /api/v0/workspaces": async (request) => {
      const body = (await request.json()) as WorkspaceCreate;
      server.sent.push(`create ${body.slug}`);
      const created = { ...workspaceJSON, id: "0199a2b4-0000-7000-8000-0000000000c1", ...body };
      list.push(created);
      return json(created, 201);
    },
    "POST /api/v0/me/onboarding-steps": async (request) => {
      if (server.stepsDown) {
        return problem(503, "server_busy");
      }
      const { step } = (await request.json()) as { step: string };
      server.sent.push(`step ${step}`);
      me = { ...me, onboarding_steps: [...me.onboarding_steps, step] };
      return json(me);
    },
  });
  return { app, server };
}

test.each([true, false])(
  "an account in a workspace already goes on at once, the step recorded once: creation %s",
  async (creation) => {
    const { app, server } = stepServer([workspaceJSON], creation);
    const { router } = renderApp("/onboarding", app);

    expect(await screen.findByRole("heading", { name: "Lab" })).toBeTruthy();
    expect(router.state.location.pathname).toBe("/lab");
    expect(server.sent).toEqual(["step workspace"]);
  }
);

test("an account without a workspace creates one, and goes on into it", async () => {
  const user = userEvent.setup();
  const { app, server } = stepServer([]);
  const { router } = renderApp("/onboarding", app);

  expect(await screen.findByText("Step 2 of 2")).toBeTruthy();
  expect(screen.getByRole("heading", { name: "Your workspace" })).toBeTruthy();
  await user.type(await screen.findByLabelText("Name"), "Acme");
  await user.click(screen.getByRole("button", { name: "Create and continue" }));

  expect(await screen.findByRole("heading", { name: "Acme" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/acme");
  expect(server.sent).toEqual(["create acme", "step workspace"]);
});

test("while creation is off, an account without a workspace reads how to get into one, and goes on", async () => {
  const user = userEvent.setup();
  const { app, server } = stepServer([], false);
  const { router } = renderApp("/onboarding", app);

  expect(await screen.findByText(/workspaces are created by its administrator/)).toBeTruthy();
  expect(screen.queryByLabelText("Name")).toBeNull();
  await user.click(screen.getByRole("button", { name: "Continue" }));

  expect(await screen.findByRole("heading", { name: "Create a workspace" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/create-workspace");
  expect(server.sent).toEqual(["step workspace"]);
});

test("a step that cannot be recorded says why; Try again records it", async () => {
  const user = userEvent.setup();
  const { app, server } = stepServer([workspaceJSON]);
  server.stepsDown = true;
  renderApp("/onboarding", app);

  expect((await screen.findByRole("alert")).textContent).toContain("busy");
  server.stepsDown = false;
  await user.click(screen.getByRole("button", { name: "Try again" }));

  expect(await screen.findByRole("heading", { name: "Lab" })).toBeTruthy();
  expect(server.sent).toEqual(["step workspace"]);
});

test("a step whose workspaces cannot be loaded says why; Try again loads them", async () => {
  const user = userEvent.setup();
  const { app, server } = stepServer([workspaceJSON]);
  server.workspacesDown = true;
  renderApp("/onboarding", app);

  expect((await screen.findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );
  server.workspacesDown = false;
  await user.click(screen.getByRole("button", { name: "Try again" }));

  expect(await screen.findByRole("heading", { name: "Lab" })).toBeTruthy();
  expect(server.sent).toEqual(["step workspace"]);
});
