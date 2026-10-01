import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import type { Notebook } from "../../services/notebook.service";
import type { Workspace } from "../../services/workspace.service";
import { json, notebookJSON, signedInApp, workspaceJSON } from "../../test/fakes";
import { renderApp } from "../../test/render";

// A workspace's home: its notebooks, as cards (M3/P4 design 3.3).

const atlas: Notebook = {
  ...notebookJSON,
  id: "0199a2b4-0000-7000-8000-0000000000c2",
  name: "Atlas",
  workspace_access: "editor",
  role: "editor",
};

function labApp(list: Notebook[], role: Workspace["role"] = "admin") {
  return signedInApp({
    "GET /api/v0/workspaces": () => json({ data: [{ ...workspaceJSON, role }] }),
    "GET /api/v0/workspaces/lab/notebooks": () => json({ data: list }),
  });
}

const main = async () =>
  (await screen.findByRole("heading", { level: 1, name: "Lab" })).closest("section") as HTMLElement;

test("the home shows each group's notebooks as cards leading to them", async () => {
  const user = userEvent.setup();
  const { router } = renderApp("/lab", labApp([atlas, notebookJSON]));
  const home = await main();

  const mine = await within(home).findByRole("list", { name: "My notebooks" });
  const team = within(home).getByRole("list", { name: "Team notebooks" });
  expect(
    within(mine)
      .getAllByRole("link")
      .map((link) => link.textContent)
  ).toEqual(["PlansPrivate"]);
  expect(
    within(team)
      .getAllByRole("link")
      .map((link) => link.textContent)
  ).toEqual(["AtlasWorkspace can edit"]);
  await user.click(within(team).getByRole("link"));

  expect(await screen.findByRole("heading", { level: 1, name: "Atlas" })).toBeTruthy();
  expect(router.state.location.pathname).toBe(`/lab/notebooks/${atlas.id}`);
});

test.each([
  ["admin", ["New notebook"]],
  ["member", ["New notebook"]],
  ["guest", []],
] as const)("with no notebook, the home says so, offering a workspace's %s %j", async (role, offered) => {
  renderApp("/lab", labApp([], role));
  const home = await main();

  expect(await within(home).findByText("No notebooks yet.")).toBeTruthy();
  expect(
    within(home)
      .queryAllByRole("button")
      .map((button) => button.textContent)
  ).toEqual(offered);
});

test("with notebooks, the home offers no New notebook: the left column does", async () => {
  renderApp("/lab", labApp([notebookJSON]));
  const home = await main();

  await within(home).findByRole("list", { name: "My notebooks" });
  expect(within(home).queryAllByRole("button")).toEqual([]);
});
