import { configure, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, beforeAll, expect, test } from "vitest";

import type { User } from "../services/account.service";
import type { Notebook, NotebookCreate } from "../services/notebook.service";
import type { Workspace } from "../services/workspace.service";
import { json, notebookJSON, problem, signedInApp, userJSON, workspaceJSON } from "../test/fakes";
import { renderApp } from "../test/render";

// Onboarding's notebook step (M3/P4 design 3.6), in StrictMode as the app
// runs: React runs a new component's effects twice there.
beforeAll(() => configure({ reactStrictMode: true }));
afterAll(() => configure({ reactStrictMode: false }));

const acme: Workspace = { ...workspaceJSON, id: "0199a2b4-0000-7000-8000-0000000000a1", slug: "acme", name: "Acme" };

/**
 * The server of an account done with the profile and workspace steps,
 * whose workspaces are list and whose notebooks in each are notebooks'
 * (creations add to them); what changed the account or its notebooks is
 * in sent. While stepsDown, recording a step answers 503; while
 * notebooksDown, the notebooks cannot be read.
 */
function stepServer(list: Workspace[], notebooks: Record<string, Notebook[]> = {}) {
  const server = { sent: [] as string[], stepsDown: false, notebooksDown: false };
  let me: User = { ...userJSON, onboarding_steps: ["profile", "workspace"] };
  const app = signedInApp({
    "GET /api/v0/me": () => json(me),
    "GET /api/v0/workspaces": () => json({ data: list }),
    "GET /api/v0/workspaces/*/notebooks": (request) => {
      const slug = new URL(request.url).pathname.split("/")[4] ?? "";
      return server.notebooksDown ? Promise.reject(new TypeError("offline")) : json({ data: notebooks[slug] ?? [] });
    },
    "POST /api/v0/workspaces/*/notebooks": async (request) => {
      const slug = new URL(request.url).pathname.split("/")[4] ?? "";
      const body = (await request.json()) as NotebookCreate;
      server.sent.push(`create ${slug} ${body.name} ${body.workspace_access}`);
      const workspace = list.find((each) => each.slug === slug) ?? workspaceJSON;
      const created = { ...notebookJSON, workspace_id: workspace.id, name: body.name };
      notebooks[slug] = [...(notebooks[slug] ?? []), created];
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

test("an account that sees a notebook in its workspace goes on at once, the step recorded once", async () => {
  const { app, server } = stepServer([workspaceJSON], { lab: [notebookJSON] });
  const { router } = renderApp("/onboarding", app);

  expect(await screen.findByRole("heading", { level: 1, name: "Lab" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/lab");
  expect(server.sent).toEqual(["step notebook"]);
});

test("an account that sees none creates a private one, My notes unless named, and goes on with it", async () => {
  const user = userEvent.setup();
  const { app, server } = stepServer([workspaceJSON]);
  renderApp("/onboarding", app);

  expect(await screen.findByText("Step 3 of 3")).toBeTruthy();
  expect(screen.getByRole("heading", { name: "Your first notebook" })).toBeTruthy();
  expect(screen.getByText(/^A private notebook in Lab/)).toBeTruthy();
  expect((await screen.findByLabelText<HTMLInputElement>("Name")).value).toBe("My notes");
  await user.click(screen.getByRole("button", { name: "Create and continue" }));

  const nav = await screen.findByRole("navigation", { name: "Lab" });
  const mine = await within(nav).findByRole("list", { name: "My notebooks" });
  expect(within(mine).getByRole("link").textContent).toBe("My notes");
  expect(server.sent).toEqual(["create lab My notes none", "step notebook"]);
});

test("a name the step finds wrong shows under the field; one typed goes out trimmed", async () => {
  const user = userEvent.setup();
  const { app, server } = stepServer([workspaceJSON]);
  renderApp("/onboarding", app);
  const name = await screen.findByLabelText("Name");

  await user.clear(name);
  await user.click(screen.getByRole("button", { name: "Create and continue" }));
  expect(await screen.findByText("Required.")).toBeTruthy();
  await user.type(name, " Field notes ");
  await user.click(screen.getByRole("button", { name: "Create and continue" }));

  expect(await screen.findByRole("navigation", { name: "Lab" })).toBeTruthy();
  expect(server.sent).toEqual(["create lab Field notes none", "step notebook"]);
});

test("the notebook goes where the account lands, past a workspace it is a guest of", async () => {
  const user = userEvent.setup();
  const { app, server } = stepServer([
    { ...acme, role: "guest" },
    { ...workspaceJSON, role: "member" },
  ]);
  app.preferences.setLastWorkspace("acme");
  renderApp("/onboarding", app);

  expect(await screen.findByText(/^A private notebook in Lab/)).toBeTruthy();
  await user.click(screen.getByRole("button", { name: "Create and continue" }));

  expect(await screen.findByRole("navigation", { name: "Acme" })).toBeTruthy();
  expect(server.sent).toEqual(["create lab My notes none", "step notebook"]);
});

test.each([
  ["a guest everywhere", [{ ...acme, role: "guest" as const }], "/acme"],
  ["without a workspace", [], "/create-workspace"],
])("an account %s reads where notebooks are created, and goes on", async (_name, list, landed) => {
  const user = userEvent.setup();
  const { app, server } = stepServer(list);
  const { router } = renderApp("/onboarding", app);

  expect(await screen.findByText(/^Notebooks are created in a workspace where you are a member/)).toBeTruthy();
  expect(screen.queryByLabelText("Name")).toBeNull();
  await user.click(screen.getByRole("button", { name: "Continue" }));

  await screen.findByRole("heading", { level: 1 });
  expect(router.state.location.pathname).toBe(landed);
  expect(server.sent).toEqual(["step notebook"]);
});

test("a step not recorded after the creation says why; Try again records it, creating no second", async () => {
  const user = userEvent.setup();
  const { app, server } = stepServer([workspaceJSON]);
  server.stepsDown = true;
  renderApp("/onboarding", app);
  await user.click(await screen.findByRole("button", { name: "Create and continue" }));

  expect((await screen.findByRole("alert")).textContent).toContain("busy");
  server.stepsDown = false;
  await user.click(screen.getByRole("button", { name: "Try again" }));

  expect(await screen.findByRole("navigation", { name: "Lab" })).toBeTruthy();
  expect(server.sent).toEqual(["create lab My notes none", "step notebook"]);
});

test("a step whose notebooks cannot be loaded says why; Try again loads them", async () => {
  const user = userEvent.setup();
  const { app, server } = stepServer([workspaceJSON], { lab: [notebookJSON] });
  server.notebooksDown = true;
  renderApp("/onboarding", app);

  expect((await screen.findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );
  server.notebooksDown = false;
  await user.click(screen.getByRole("button", { name: "Try again" }));

  expect(await screen.findByRole("heading", { level: 1, name: "Lab" })).toBeTruthy();
  expect(server.sent).toEqual(["step notebook"]);
});
