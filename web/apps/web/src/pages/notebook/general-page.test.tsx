import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { json, notebookJSON, problem } from "../../test/fakes";
import { ada, bob, notebookMember, notebookServer } from "../../test/notebook-server";
import { renderApp } from "../../test/render";

// A notebook's general settings (M3/P4 design 3.4).

const general = `/lab/notebooks/${notebookJSON.id}/settings/general`;
const plans = `/api/v0/notebooks/${notebookJSON.id}`;
const nameField = () => screen.findByLabelText<HTMLInputElement>("Name");
const nav = () => screen.findByRole("navigation", { name: "Lab" });

test("the settings of a notebook open on its general page, from its home", async () => {
  const user = userEvent.setup();
  const { router } = renderApp(`/lab/notebooks/${notebookJSON.id}`, notebookServer().app);

  await user.click(await screen.findByRole("link", { name: "Notebook settings" }));

  expect(await screen.findByRole("heading", { level: 2, name: "General" })).toBeTruthy();
  expect(router.state.location.pathname).toBe(general);
  expect(screen.getByRole("heading", { level: 1, name: "Plans settings" })).toBeTruthy();
  const settings = screen.getByRole("navigation", { name: "Notebook settings" });
  expect(within(settings).getByRole("link", { name: "General" }).getAttribute("aria-current")).toBe("page");
  expect(
    within(await nav())
      .getByRole("link", { name: "Plans" })
      .getAttribute("aria-current")
  ).toBe("page");
});

test("an admin renames the notebook; the left column shows the new name, in its place", async () => {
  const user = userEvent.setup();
  const server = notebookServer();
  renderApp(general, server.app);

  await user.clear(await nameField());
  await user.type(await nameField(), "  Roadmap ");
  await user.click(screen.getAllByRole("button", { name: "Save" })[0] as HTMLElement);

  expect(await screen.findByText("Saved.")).toBeTruthy();
  expect(server.sent).toEqual(['PATCH {"name":"Roadmap"}']);
  expect((await nameField()).value).toBe("Roadmap");
  expect(within(await nav()).getByRole("link", { name: "Roadmap" })).toBeTruthy();

  // The name as saved goes out no more.
  await user.click(screen.getAllByRole("button", { name: "Save" })[0] as HTMLElement);
  expect(await screen.findByText("Saved.")).toBeTruthy();
  expect(server.sent).toEqual(['PATCH {"name":"Roadmap"}']);
});

test("a name the local check or the server refuses shows under the field, by a title's rules", async () => {
  const user = userEvent.setup();
  const server = notebookServer({
    answers: {
      [`PATCH /api/v0/notebooks/${notebookJSON.id}`]: () =>
        problem(422, "validation_failed", { errors: [{ field: "name", code: "not_allowed", message: "…" }] }),
    },
  });
  renderApp(general, server.app);
  const save = async () => user.click(screen.getAllByRole("button", { name: "Save" })[0] as HTMLElement);

  await user.clear(await nameField());
  await save();
  expect(await screen.findByText("Required.")).toBeTruthy();
  await user.type(await nameField(), "nul.txt");
  await save();

  expect(
    await screen.findByText("Windows reserves this name (such as CON, NUL or COM1); choose another.")
  ).toBeTruthy();
  await waitFor(async () => expect(document.activeElement).toBe(await nameField()));
  expect(server.sent).toEqual(['PATCH {"name":"nul.txt"}']);
});

test("an admin opens the notebook to the workspace on Save; the left column moves it to the team's", async () => {
  const user = userEvent.setup();
  const server = notebookServer({ members: [notebookMember(ada, "admin")] });
  renderApp(general, server.app);
  await within(await nav()).findByRole("list", { name: "My notebooks" });

  // Moving through the options chooses them; nothing goes out until Save.
  await user.click(await screen.findByRole("radio", { name: "Workspace can read" }));
  await user.keyboard("{ArrowDown}");
  expect(screen.getByRole("radio", { name: "Workspace can edit" })).toHaveProperty("checked", true);
  expect(server.sent).toEqual([]);
  await user.click(screen.getAllByRole("button", { name: "Save" })[1] as HTMLElement);

  expect(await within(await nav()).findByRole("list", { name: "Team notebooks" })).toBeTruthy();
  expect(server.sent).toEqual(['PATCH {"workspace_access":"editor"}']);
  expect(within(await nav()).queryByRole("list", { name: "My notebooks" })).toBeNull();
});

test.each([
  ["deleted", undefined],
  ["already gone", () => problem(404, "notebook.not_found")],
])(
  "the admin deletes the notebook once its name is typed, and lands on the workspace's home, its heading focused: %s",
  async (_, gone) => {
    const user = userEvent.setup();
    const server = notebookServer(gone === undefined ? {} : { answers: { [`DELETE ${plans}`]: gone } });
    const { router } = renderApp(general, server.app);

    await user.click(await screen.findByRole("button", { name: "Delete notebook" }));
    const dialog = await screen.findByRole("alertdialog", { name: "Delete Plans?" });
    const confirm = within(dialog).getByRole("button", { name: "Delete" });
    expect(confirm).toHaveProperty("disabled", true);
    await user.type(within(dialog).getByLabelText("Type Plans to confirm"), "Plans{Enter}");

    const home = await screen.findByRole("heading", { level: 1, name: "Lab" });
    expect(router.state.location.pathname).toBe("/lab");
    expect(server.sent).toEqual(["DELETE"]);
    await waitFor(() => expect(document.activeElement).toBe(home));
    expect(screen.queryByRole("heading", { name: "Page not found" })).toBeNull();
    expect(within(await nav()).queryByRole("link", { name: "Plans" })).toBeNull();
  }
);

test("a deletion refused stays in the dialog with why, and the notebook stays", async () => {
  const user = userEvent.setup();
  const server = notebookServer({
    answers: { [`DELETE /api/v0/notebooks/${notebookJSON.id}`]: () => problem(403, "forbidden") },
  });
  renderApp(general, server.app);

  await user.click(await screen.findByRole("button", { name: "Delete notebook" }));
  const dialog = await screen.findByRole("alertdialog");
  await user.type(within(dialog).getByLabelText("Type Plans to confirm"), "Plans{Enter}");

  expect((await within(dialog).findByRole("alert")).textContent).toBe("You do not have permission to do this.");
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  expect(within(await nav()).getByRole("link", { name: "Plans" })).toBeTruthy();
  expect(screen.getByRole("heading", { level: 2, name: "General" })).toBeTruthy();
});

test.each(["editor", "reader"] as const)(
  "its %s sees the name and the access, and cannot change them",
  async (role) => {
    const members = [notebookMember(ada, role), notebookMember(bob, "admin")];
    renderApp(general, notebookServer({ access: "viewer", members }).app);

    expect(await screen.findByText("Only the notebook's admins can change its name.")).toBeTruthy();
    expect(screen.getByText("Workspace can read")).toBeTruthy();
    expect(screen.getByText("Only the notebook's admins can change its workspace access.")).toBeTruthy();
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.queryByRole("radio")).toBeNull();
    expect(screen.queryByRole("button", { name: "Save" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Delete notebook" })).toBeNull();
  }
);

/** The general page's two Save buttons: the name's, then the access's. */
const saves = () => screen.getAllByRole("button", { name: "Save" }) as [HTMLElement, HTMLElement];

test("a name or an access chosen while its save is out keeps the choice, not marked saved", async () => {
  const user = userEvent.setup();
  let release: (() => void) | undefined;
  const server = notebookServer({
    members: [notebookMember(ada, "admin")],
    answers: {
      [`PATCH ${plans}`]: async (request) => {
        const body = (await request.clone().json()) as object;
        await new Promise<void>((resolve) => (release = resolve));
        return json({ ...notebookJSON, ...body });
      },
    },
  });
  renderApp(general, server.app);

  await user.clear(await nameField());
  await user.type(await nameField(), "Roadmap");
  await user.click(saves()[0]);
  await waitFor(() => expect(server.sent).toHaveLength(1));
  await user.type(await nameField(), " 2027");
  release?.();
  await waitFor(() => expect(saves()[0]).toHaveProperty("disabled", false));
  expect((await nameField()).value).toBe("Roadmap 2027");
  expect(screen.queryByText("Saved.")).toBeNull();

  await user.click(screen.getByRole("radio", { name: "Workspace can read" }));
  await user.click(saves()[1]);
  await waitFor(() => expect(server.sent).toHaveLength(2));
  await user.click(screen.getByRole("radio", { name: "Workspace can edit" }));
  release?.();
  await waitFor(() => expect(saves()[1]).toHaveProperty("disabled", false));
  expect(screen.getByRole("radio", { name: "Workspace can edit" })).toHaveProperty("checked", true);
  expect(screen.queryByText("Saved.")).toBeNull();
});

afterEach(() => vi.useRealTimers());

// SWR reads the notebooks again as the tab regains the focus: untouched
// forms follow a change made elsewhere, and Save sends nothing back over it.
test("untouched forms follow another admin's changes; Save sends nothing back over them", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const server = notebookServer({ members: [notebookMember(ada, "admin"), notebookMember(bob, "admin")] });
  renderApp(general, server.app);
  expect((await nameField()).value).toBe("Plans");

  server.plans = { ...server.plans, name: "Roadmap", workspace_access: "editor" };
  await act(() => vi.advanceTimersByTimeAsync(6_000));
  act(() => void window.dispatchEvent(new Event("focus")));

  await waitFor(async () => expect((await nameField()).value).toBe("Roadmap"));
  expect(screen.getByRole("radio", { name: "Workspace can edit" })).toHaveProperty("checked", true);
  await user.click(saves()[0]);
  await user.click(saves()[1]);
  expect(await screen.findAllByText("Saved.")).toHaveLength(2);
  expect(server.sent).toEqual([]);
});
