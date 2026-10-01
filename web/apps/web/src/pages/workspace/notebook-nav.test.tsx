import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import type { Notebook } from "../../services/notebook.service";
import type { Workspace } from "../../services/workspace.service";
import { json, notebookJSON, problem, signedInApp, workspaceJSON, type Answer } from "../../test/fakes";
import { renderApp } from "../../test/render";

// The notebooks in a workspace's left column, and the creation of one
// (M3/P4 design 3.3).

/** notebook is a notebook of Lab's named name, with id ending in n. */
function notebook(n: number, name: string, more: Partial<Notebook> = {}): Notebook {
  return { ...notebookJSON, id: `0199a2b4-0000-7000-8000-0000000000${(0xc0 + n).toString(16)}`, name, ...more };
}

// In the server's order, by name: Beta and Plans are the account's own;
// Atlas is open to the workspace, Zeta has another member.
const atlas = notebook(1, "Atlas", { workspace_access: "viewer", role: "reader" });
const beta = notebook(2, "Beta");
const plans = notebook(3, "Plans");
const zeta = notebook(4, "Zeta", { member_count: 2 });

/** The app of an account that is role in Lab, which sees list there; routes adds to the answers. */
function labApp(list: Notebook[], role: Workspace["role"] = "admin", routes: Record<string, Answer> = {}) {
  return signedInApp({
    "GET /api/v0/workspaces": () => json({ data: [{ ...workspaceJSON, role }] }),
    "GET /api/v0/workspaces/lab/notebooks": () => json({ data: list }),
    ...routes,
  });
}

const nav = () => screen.findByRole("navigation", { name: "Lab" });

/** The links of each group shown in column, by the group's name. */
function groups(column: HTMLElement): Record<string, (string | null)[][]> {
  return Object.fromEntries(
    ["My notebooks", "Team notebooks"].flatMap((name) => {
      const list = within(column).queryByRole("list", { name });
      const links = list === null ? [] : within(list).getAllByRole("link");
      return list === null ? [] : [[name, links.map((link) => [link.textContent, link.getAttribute("href")])]];
    })
  );
}

test("the left column lists the account's own notebooks, then the team's, each by name", async () => {
  renderApp("/lab", labApp([atlas, beta, plans, zeta]));

  const column = await nav();
  await within(column).findByRole("list", { name: "My notebooks" });
  expect(groups(column)).toEqual({
    "My notebooks": [
      ["Beta", `/lab/notebooks/${beta.id}`],
      ["Plans", `/lab/notebooks/${plans.id}`],
    ],
    "Team notebooks": [
      ["Atlas", `/lab/notebooks/${atlas.id}`],
      ["Zeta", `/lab/notebooks/${zeta.id}`],
    ],
  });
  expect(
    within(column)
      .getAllByRole("heading", { level: 2 })
      .map((h) => h.textContent)
  ).toEqual(["My notebooks", "Team notebooks"]);
});

test.each([
  ["only the account's own", [plans], ["My notebooks"]],
  ["only the team's", [atlas], ["Team notebooks"]],
])("with %s, the other group is not shown", async (_name, list, shown) => {
  renderApp("/lab", labApp(list));

  const column = await nav();
  await within(column).findAllByRole("list");
  expect(Object.keys(groups(column))).toEqual(shown);
});

test("with no notebook, the left column says so", async () => {
  renderApp("/lab", labApp([]));

  const column = await nav();
  expect(await within(column).findByText("No notebooks yet.")).toBeTruthy();
  expect(within(column).queryAllByRole("list")).toEqual([]);
});

test.each([
  ["admin", true],
  ["member", true],
  ["guest", false],
] as const)("a workspace's %s is offered New notebook: %s", async (role, offered) => {
  renderApp("/lab", labApp([plans], role));

  const column = await nav();
  await within(column).findByRole("list", { name: "My notebooks" });
  expect(within(column).queryByRole("button", { name: "New notebook" }) !== null).toBe(offered);
});

test("the left column says so when the notebooks cannot be loaded; Try again loads them", async () => {
  const user = userEvent.setup();
  let down = true;
  renderApp(
    "/lab",
    labApp([], "admin", {
      "GET /api/v0/workspaces/lab/notebooks": () =>
        down ? Promise.reject(new TypeError("offline")) : json({ data: [plans] }),
    })
  );
  const column = await nav();
  expect((await within(column).findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );

  down = false;
  await user.click(within(column).getByRole("button", { name: "Try again" }));

  expect(await within(column).findByRole("link", { name: "Plans" })).toBeTruthy();
});

afterEach(() => vi.useRealTimers());

test("reading the notebooks again shows what changed elsewhere", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  let list = [plans];
  const { router } = renderApp(
    "/lab",
    labApp([], "admin", { "GET /api/v0/workspaces/lab/notebooks": () => json({ data: list }) })
  );
  expect(await within(await nav()).findByRole("link", { name: "Plans" })).toBeTruthy();

  // Shared elsewhere; coming back to the workspace reads the list again, once the read before is no longer recent.
  list = [atlas, plans];
  await act(() => router.navigate("/create-workspace"));
  await act(() => vi.advanceTimersByTimeAsync(2_000));
  await act(() => router.navigate("/lab"));

  expect(await within(await nav()).findByRole("link", { name: "Atlas" })).toBeTruthy();
});

/** The app of Lab's admin, who sees list; what POST notebooks gets is in sent, and answer answers it. */
function creationServer(answer?: Answer, list: Notebook[] = []) {
  const sent: unknown[] = [];
  const app = labApp(list, "admin", {
    "POST /api/v0/workspaces/lab/notebooks": async (request) => {
      const body = (await request.json()) as { name: string; workspace_access: Notebook["workspace_access"] };
      sent.push(body);
      return (
        answer?.(request) ??
        json({ ...notebookJSON, id: zeta.id, name: body.name, workspace_access: body.workspace_access }, 201)
      );
    },
  });
  return { app, sent };
}

async function openCreation(user: ReturnType<typeof userEvent.setup>) {
  await user.click(within(await nav()).getByRole("button", { name: "New notebook" }));
  return screen.findByRole("dialog", { name: "New notebook" });
}

// The focus goes from the dialog to the new home's heading, never back to
// New notebook on the way: a screen reader would read that first.
test("a notebook created opens on its home, arrived at, and the left column lists it", async () => {
  const user = userEvent.setup();
  const { app, sent } = creationServer(undefined, [plans]);
  const { router } = renderApp(`/lab/notebooks/${plans.id}`, app);
  await screen.findByRole("heading", { level: 1, name: "Plans" });
  const dialog = await openCreation(user);

  const name = within(dialog).getByLabelText("Name");
  expect(within(dialog).getByRole("radio", { name: "Private" })).toHaveProperty("checked", true);
  expect(name.getAttribute("aria-describedby")).not.toBeNull();
  await user.type(name, "  Field notes ");
  await user.click(within(dialog).getByRole("radio", { name: "Workspace can read" }));
  const focused: Element[] = [];
  const record = (event: FocusEvent) => focused.push(event.target as Element);
  document.addEventListener("focusin", record);
  await user.click(within(dialog).getByRole("button", { name: "Create" }));

  const heading = await screen.findByRole("heading", { level: 1, name: "Field notes" });
  expect(sent).toEqual([{ name: "Field notes", workspace_access: "viewer" }]);
  expect(router.state.location.pathname).toBe(`/lab/notebooks/${zeta.id}`);
  expect(screen.queryByRole("dialog")).toBeNull();
  // The dialog gives the focus back on a timer: the heading keeps it.
  await act(() => new Promise((resolve) => setTimeout(resolve, 10)));
  document.removeEventListener("focusin", record);
  expect(document.activeElement).toBe(heading);
  expect(focused.map((element) => element.textContent)).not.toContain("New notebook");
  const link = within(await within(await nav()).findByRole("list", { name: "Team notebooks" })).getByRole("link");
  expect([link.textContent, link.getAttribute("aria-current")]).toEqual(["Field notes", "page"]);
});

test.each([
  ["no name", " ", "Required."],
  ["a name too long", "研".repeat(86), "At most 255 bytes: 255 Latin letters, or about 85 Chinese characters."],
])("%s is found before sending, under the field", async (_name, typed, message) => {
  const user = userEvent.setup();
  const { app, sent } = creationServer();
  renderApp("/lab", app);
  const dialog = await openCreation(user);
  const name = within(dialog).getByLabelText("Name");

  await user.type(name, typed);
  await user.click(within(dialog).getByRole("button", { name: "Create" }));

  await waitFor(() => expect(document.activeElement).toBe(name));
  expect(name.getAttribute("aria-invalid")).toBe("true");
  expect(within(dialog).getByText(message)).toBeTruthy();
  expect(sent).toEqual([]);
});

test.each([
  ["invalid_format", 'Cannot contain / \\ : * ? " < > | # ^ [ ] or control characters, nor start or end with a dot.'],
  ["not_allowed", "Windows reserves this name (such as CON, NUL or COM1); choose another."],
])("the server's %s shows under the name, by a title's rules", async (code, message) => {
  const user = userEvent.setup();
  const { app } = creationServer(() =>
    problem(422, "validation_failed", { errors: [{ field: "name", code, message: "…" }] })
  );
  renderApp("/lab", app);
  const dialog = await openCreation(user);
  const name = within(dialog).getByLabelText("Name");

  await user.type(name, "CON");
  await user.click(within(dialog).getByRole("button", { name: "Create" }));

  expect(await within(dialog).findByText(message)).toBeTruthy();
  expect(within(dialog).queryByRole("alert")).toBeNull();
  await waitFor(() => expect(document.activeElement).toBe(name));
});

test("a refusal shows above the form, which stays as typed; Cancel forgets it", async () => {
  const user = userEvent.setup();
  const { app } = creationServer(() => problem(403, "forbidden"));
  renderApp("/lab", app);
  let dialog = await openCreation(user);
  await user.type(within(dialog).getByLabelText("Name"), "Plans");
  await user.click(within(dialog).getByRole("button", { name: "Create" }));

  expect((await within(dialog).findByRole("alert")).textContent).toBe("You do not have permission to do this.");
  expect(within(dialog).getByLabelText<HTMLInputElement>("Name").value).toBe("Plans");
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  dialog = await openCreation(user);

  expect(within(dialog).getByLabelText<HTMLInputElement>("Name").value).toBe("");
  expect(within(dialog).queryByRole("alert")).toBeNull();
});

// Cancelled, the dialog no longer leads anywhere: the creation, already
// out, still lands in the left column, and the next dialog gives the focus
// back to its trigger as usual.
test("a creation answered after Cancel goes nowhere; the left column lists the notebook", async () => {
  const user = userEvent.setup();
  let release: (() => void) | undefined;
  const { app } = creationServer(async () => {
    await new Promise<void>((resolve) => (release = resolve));
    return json({ ...notebookJSON, id: zeta.id, name: "Late" }, 201);
  });
  const { router } = renderApp("/lab", app);
  let dialog = await openCreation(user);
  await user.type(within(dialog).getByLabelText("Name"), "Late");
  await user.click(within(dialog).getByRole("button", { name: "Create" }));
  await waitFor(() => expect(release).toBeDefined());

  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  release?.();

  expect(await within(await nav()).findByRole("link", { name: "Late" })).toBeTruthy();
  await act(() => new Promise((resolve) => setTimeout(resolve, 10)));
  expect(router.state.location.pathname).toBe("/lab");
  dialog = await openCreation(user);
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  await waitFor(() =>
    expect(document.activeElement).toBe(
      within(screen.getByRole("navigation", { name: "Lab" })).getByRole("button", { name: "New notebook" })
    )
  );
});
