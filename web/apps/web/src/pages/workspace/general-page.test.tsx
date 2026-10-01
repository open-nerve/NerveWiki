import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import type { Workspace } from "../../services/workspace.service";
import { json, problem, signedInApp, workspaceJSON, type Answer } from "../../test/fakes";
import { renderApp } from "../../test/render";
import { watchFor } from "../../test/watch";

// A workspace's general settings (M2/P5 design 3.6).

const acme: Workspace = { ...workspaceJSON, id: "0199a2b4-0000-7000-8000-0000000000a1", slug: "acme", name: "Acme" };

/**
 * The server of the page: the account's workspaces are Acme (whose role is
 * role) and Lab; rename and remove answer the changes, and what went out is
 * in sent.
 */
function settingsServer({
  role = "admin",
  rename,
  remove,
}: { role?: Workspace["role"]; rename?: Answer; remove?: Answer } = {}) {
  const sent: string[] = [];
  let list: Workspace[] = [{ ...acme, role }, workspaceJSON];
  const app = signedInApp({
    "GET /api/v0/workspaces": () => json({ data: list }),
    "PATCH /api/v0/workspaces/acme": async (request) => {
      const { name } = (await request.json()) as { name: string };
      sent.push(`PATCH ${name}`);
      return rename?.(request) ?? json({ ...acme, name, updated_at: "2026-10-01T09:00:00Z" });
    },
    "DELETE /api/v0/workspaces/acme": async (request) => {
      sent.push("DELETE");
      const answer = remove === undefined ? new Response(null, { status: 204 }) : await remove(request);
      if (answer.ok) {
        list = list.filter((w) => w.slug !== "acme");
      }
      return answer;
    },
  });
  return { app, sent };
}

const nameField = () => screen.findByLabelText<HTMLInputElement>("Name");

test("the settings of a workspace open on its general page", async () => {
  const { router } = renderApp("/acme/settings", settingsServer().app);

  expect(await screen.findByRole("heading", { level: 2, name: "General" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/acme/settings/general");
  const nav = screen.getByRole("navigation", { name: "Acme" });
  expect(within(nav).getByRole("link", { name: "Settings" }).getAttribute("aria-current")).toBe("page");
  expect(within(nav).getByRole("link", { name: "Home" }).getAttribute("aria-current")).toBeNull();
});

test("an admin renames the workspace; the switcher shows the new name", async () => {
  const user = userEvent.setup();
  const { app, sent } = settingsServer();
  renderApp("/acme/settings/general", app);

  await user.clear(await nameField());
  await user.type(await nameField(), "  Acme Labs ");
  await user.click(screen.getByRole("button", { name: "Save" }));

  expect(await screen.findByText("Saved.")).toBeTruthy();
  expect(sent).toEqual(["PATCH Acme Labs"]);
  expect(screen.getByRole("button", { name: "Acme Labs" })).toBeTruthy();
  expect((await nameField()).value).toBe("Acme Labs");

  // Saving the name it has sends nothing; an edit takes the saved state away.
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(sent).toEqual(["PATCH Acme Labs"]);
  await user.type(await nameField(), "!");
  expect(screen.queryByText("Saved.")).toBeNull();
});

// A rename answered after the field was edited again saved the name sent, not the one shown: the field keeps the
// edit, not marked saved, and the next save sends it (R3 of the M2 Codex review).
test("a name edited while its rename is out keeps the edit, not marked saved", async () => {
  const user = userEvent.setup();
  let release: (() => void) | undefined;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  let first = true;
  const { app, sent } = settingsServer({
    rename: async () => {
      const name = first ? "Acme Labs" : "Acme Works";
      if (first) {
        first = false;
        await held;
      }
      return json({ ...acme, name, updated_at: "2026-10-01T09:00:00Z" });
    },
  });
  renderApp("/acme/settings/general", app);
  await user.clear(await nameField());
  const save = screen.getByRole("button", { name: "Save" });
  await user.type(await nameField(), " Acme Labs");
  await user.click(save);
  await waitFor(() => expect(sent).toHaveLength(1));
  await user.clear(await nameField());
  await user.type(await nameField(), "Acme Works ");
  release?.();
  await waitFor(() => expect(save).toHaveProperty("disabled", false));

  expect((await nameField()).value).toBe("Acme Works ");
  expect(screen.queryByText("Saved.")).toBeNull();
  await user.click(save);
  expect(await screen.findByText("Saved.")).toBeTruthy();
  expect(sent).toEqual(["PATCH Acme Labs", "PATCH Acme Works"]);
});

test("a name the local check or the server refuses shows under the field", async () => {
  const user = userEvent.setup();
  const { app, sent } = settingsServer({
    rename: () =>
      problem(422, "validation_failed", { errors: [{ field: "name", code: "invalid_format", message: "control" }] }),
  });
  renderApp("/acme/settings/general", app);

  await user.clear(await nameField());
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(await screen.findByText("Required.")).toBeTruthy();
  await user.type(await nameField(), "a".repeat(81));
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(await screen.findByText("At most 80 characters.")).toBeTruthy();
  expect(sent).toEqual([]);

  await user.clear(await nameField());
  await user.type(await nameField(), "Acme Two");
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(await screen.findByText("No control characters.")).toBeTruthy();
  expect(screen.queryByText("Saved.")).toBeNull();
});

test.each([
  ["deleted", undefined],
  ["already gone", () => problem(404, "workspace.not_found")],
])("deleting asks for the slug, then lands on another workspace: %s", async (_, remove) => {
  const user = userEvent.setup();
  const { app, sent } = settingsServer({ remove });
  const { router } = renderApp("/acme/settings/general", app);

  await user.click(await screen.findByRole("button", { name: "Delete workspace" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Delete Acme?" });
  const confirm = within(dialog).getByRole("button", { name: "Delete" });
  expect(confirm.hasAttribute("disabled")).toBe(true);
  await user.type(within(dialog).getByLabelText("Type acme to confirm"), "acm");
  expect(confirm.hasAttribute("disabled")).toBe(true);
  await user.type(within(dialog).getByLabelText("Type acme to confirm"), "e");
  const notFound = watchFor("Page not found");
  await user.click(confirm);

  expect(await screen.findByRole("heading", { name: "Lab" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/lab");
  expect(notFound()).toBe(false);
  expect(sent).toEqual(["DELETE"]);
  await user.click(screen.getByRole("button", { name: "Lab" }));
  expect((await screen.findAllByRole("menuitemradio")).map((item) => item.textContent)).toEqual(["Lab"]);
});

test("the dialog opens on the slug's field, where Enter deletes once it is typed", async () => {
  const user = userEvent.setup();
  const { app, sent } = settingsServer();
  const { router } = renderApp("/acme/settings/general", app);

  await user.click(await screen.findByRole("button", { name: "Delete workspace" }));
  const field = within(await screen.findByRole("alertdialog")).getByLabelText("Type acme to confirm");
  await waitFor(() => expect(document.activeElement).toBe(field));
  await user.keyboard("acm{Enter}");
  expect(sent).toEqual([]);
  await user.keyboard("e{Enter}");

  expect(await screen.findByRole("heading", { name: "Lab" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/lab");
  expect(sent).toEqual(["DELETE"]);
});

test("a refused deletion stays in the dialog with why; closing it clears the slug typed", async () => {
  const user = userEvent.setup();
  const { app } = settingsServer({ remove: () => problem(403, "forbidden") });
  renderApp("/acme/settings/general", app);

  await user.click(await screen.findByRole("button", { name: "Delete workspace" }));
  let dialog = await screen.findByRole("alertdialog");
  await user.type(within(dialog).getByLabelText("Type acme to confirm"), "acme");
  await user.click(within(dialog).getByRole("button", { name: "Delete" }));

  expect((await within(dialog).findByRole("alert")).textContent).toBe("You do not have permission to do this.");
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());

  await user.click(screen.getByRole("button", { name: "Delete workspace" }));
  dialog = await screen.findByRole("alertdialog");
  expect(within(dialog).getByLabelText<HTMLInputElement>("Type acme to confirm").value).toBe("");
  expect(within(dialog).queryByRole("alert")).toBeNull();
});

test.each(["member", "guest"] as const)("a %s sees the name and the address, and cannot change them", async (role) => {
  renderApp("/acme/settings/general", settingsServer({ role }).app);

  expect(await screen.findByText("Only the workspace's admins can rename it.")).toBeTruthy();
  expect(screen.queryByRole("textbox")).toBeNull();
  expect(screen.queryByRole("button", { name: "Save" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Delete workspace" })).toBeNull();
  expect(screen.getByText("The address of a workspace cannot change.")).toBeTruthy();
});
