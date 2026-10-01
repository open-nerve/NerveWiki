import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import type { Workspace, WorkspaceCreate } from "../services/workspace.service";
import { instanceJSON, json, problem, signedInApp, workspaceJSON } from "../test/fakes";
import { renderApp } from "../test/render";

// Creating a workspace (M2/P5 design 3.5).

/** The server's answer to the check of each slug the tests type. */
function availability(slug: string): Response {
  switch (slug) {
    case "settings":
      return json({ available: false, reason: "reserved" });
    case "taken":
      return json({ available: false, reason: "taken" });
    default:
      return json({ available: true });
  }
}

/**
 * The server of the creation page: create answers a creation; what was
 * created and which slugs were checked is in sent and asked. Its list has
 * workspaceJSON and what it created.
 */
function creationServer(
  create: (body: WorkspaceCreate) => Response = (body) => json(created(body), 201),
  enabled = true
) {
  const sent: WorkspaceCreate[] = [];
  const asked: string[] = [];
  const list = [workspaceJSON];
  const app = signedInApp({
    "GET /api/v0/instance": () => json({ ...instanceJSON, workspace_creation_enabled: enabled }),
    "GET /api/v0/workspaces": () => json({ data: list }),
    "POST /api/v0/workspaces": async (request) => {
      const body = (await request.json()) as WorkspaceCreate;
      sent.push(body);
      const answer = create(body);
      if (answer.ok) {
        list.unshift(created(body));
      }
      return answer;
    },
    "GET /api/v0/workspace-slugs/*": (request) => {
      const slug = decodeURIComponent(new URL(request.url).pathname.split("/").pop() ?? "");
      asked.push(slug);
      return availability(slug);
    },
  });
  return { app, sent, asked };
}

function created(body: WorkspaceCreate): Workspace {
  return { ...workspaceJSON, id: "0199a2b4-0000-7000-8000-0000000000c1", name: body.name, slug: body.slug };
}

const nameField = () => screen.findByLabelText("Name");
const slugField = () => screen.getByLabelText<HTMLInputElement>("Address");

test("the slug follows the name until one is typed, and the server says whether it is free", async () => {
  const user = userEvent.setup();
  const { app, asked } = creationServer();
  renderApp("/create-workspace", app);

  await user.type(await nameField(), "Acme Labs");
  expect(slugField().value).toBe("acme-labs");
  expect(await screen.findByText("Available.")).toBeTruthy();

  await user.clear(slugField());
  await user.type(slugField(), "settings");
  expect(await screen.findByText("Reserved by the app. Choose another.")).toBeTruthy();
  await user.clear(slugField());
  await user.type(slugField(), "taken");
  expect(await screen.findByText("Another workspace has it.")).toBeTruthy();
  expect(slugField().getAttribute("aria-invalid")).toBe("true");

  // Typed once, the slug no longer follows the name.
  await user.type(await nameField(), " Two");
  expect(slugField().value).toBe("taken");
  // Only slugs spelled as slugs are asked, each once it settled.
  await user.clear(slugField());
  await user.type(slugField(), "Not A Slug");
  await new Promise((resolve) => setTimeout(resolve, 400));
  expect(asked).toEqual(["acme-labs", "settings", "taken"]);
});

test("a name or a slug the local check refuses is not sent", async () => {
  const user = userEvent.setup();
  const { app, sent } = creationServer();
  renderApp("/create-workspace", app);

  await user.type(await nameField(), "研发部");
  expect(slugField().value).toBe("");
  await user.click(screen.getByRole("button", { name: "Create workspace" }));
  expect(await screen.findByText("Required.")).toBeTruthy();
  expect(document.activeElement).toBe(slugField());

  await user.clear(await nameField());
  await user.type(slugField(), "Bad Slug");
  await user.click(screen.getByRole("button", { name: "Create workspace" }));
  expect(await screen.findByText("1 to 48 lower-case letters, digits, _ or -.")).toBeTruthy();
  expect(screen.getByText("Required.")).toBeTruthy();
  expect(sent).toEqual([]);
});

test("a creation goes into the new workspace, which the switcher lists", async () => {
  const user = userEvent.setup();
  const { app, sent } = creationServer();
  const { router } = renderApp("/create-workspace", app);

  await user.type(await nameField(), "  Acme ");
  await user.click(screen.getByRole("button", { name: "Create workspace" }));

  expect(await screen.findByRole("heading", { name: "Acme" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/acme");
  expect(sent).toEqual([{ name: "Acme", slug: "acme" }]);
  await user.click(screen.getByRole("button", { name: "Acme" }));
  expect((await screen.findAllByRole("menuitemradio")).map((item) => item.textContent)).toEqual(["Acme", "Lab"]);
});

test.each([
  [problem(409, "workspace.slug_taken"), "Address", "Another workspace already has this address."],
  [
    problem(422, "validation_failed", { errors: [{ field: "slug", code: "not_allowed", message: "is reserved" }] }),
    "Address",
    "Reserved by the app. Choose another.",
  ],
  [
    problem(422, "validation_failed", { errors: [{ field: "name", code: "invalid_format", message: "control" }] }),
    "Name",
    "No control characters.",
  ],
])("a refused creation shows why under its field: %#", async (answer, field, message) => {
  const user = userEvent.setup();
  const { app } = creationServer(() => answer);
  const { router } = renderApp("/create-workspace", app);

  await user.type(await nameField(), "Acme");
  await user.click(screen.getByRole("button", { name: "Create workspace" }));

  await waitFor(() => expect(document.activeElement).toBe(screen.getByLabelText(field)));
  expect(screen.getByText(message)).toBeTruthy();
  expect(screen.queryByRole("alert")).toBeNull();
  expect(router.state.location.pathname).toBe("/create-workspace");
});

test("a slug changed after a refusal shows whether the new one is free, not the refusal", async () => {
  const user = userEvent.setup();
  const { app } = creationServer((body) =>
    body.slug === "acme" ? problem(409, "workspace.slug_taken") : json(created(body), 201)
  );
  renderApp("/create-workspace", app);

  await user.type(await nameField(), "Acme");
  await user.click(screen.getByRole("button", { name: "Create workspace" }));
  expect(await screen.findByText("Another workspace already has this address.")).toBeTruthy();

  await user.type(slugField(), "-labs");
  expect(await screen.findByText("Available.")).toBeTruthy();
  expect(screen.queryByText("Another workspace already has this address.")).toBeNull();
});

test("a creation refused as a whole says why above the form", async () => {
  const user = userEvent.setup();
  const { app } = creationServer(() => problem(403, "workspace.creation_disabled"));
  renderApp("/create-workspace", app);

  await user.type(await nameField(), "Acme");
  await user.click(screen.getByRole("button", { name: "Create workspace" }));

  expect((await screen.findByRole("alert")).textContent).toContain("Creating workspaces is disabled on this server.");
});

test("while creation is off, the page says how to get into a workspace, with no form", async () => {
  const { app } = creationServer(undefined, false);
  renderApp("/create-workspace", app);

  expect(await screen.findByText(/workspaces are created by its administrator/)).toBeTruthy();
  expect(screen.queryByRole("textbox")).toBeNull();
  expect(screen.queryByRole("button", { name: "Create workspace" })).toBeNull();
});
