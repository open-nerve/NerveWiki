import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import type { Notebook } from "../../services/notebook.service";
import { json, notebookJSON, problem, signedInApp } from "../../test/fakes";
import { ada, bob, cy, notebookMember, notebookServer } from "../../test/notebook-server";
import { renderApp } from "../../test/render";

// A notebook's members page: its members, adding them, leaving (M3/P4
// design 3.4).

const members = `/lab/notebooks/${notebookJSON.id}/settings/members`;
const nav = () => screen.findByRole("navigation", { name: "Lab" });
const plans = `/api/v0/notebooks/${notebookJSON.id}`;

/** The rows of the members list, each as its text. */
async function rows(): Promise<string[]> {
  const list = await screen.findByRole("list", { name: "Members" });
  return within(list)
    .getAllByRole("listitem")
    .map((item) => item.textContent ?? "");
}

/** The button of the role of the member who, which reads it as role. */
const roleButton = (role: string, who: string) => screen.findByRole("button", { name: `${role}, role of ${who}` });

async function choose(user: ReturnType<typeof userEvent.setup>, button: HTMLElement, role: string) {
  await user.click(button);
  await user.click(await screen.findByRole("menuitemradio", { name: role }));
}

test("the members are listed by when they joined, the account's own row marked and without controls", async () => {
  renderApp(members, notebookServer().app);

  expect(await rows()).toEqual([
    "AdaYouada@example.com · Joined Oct 1, 2026Admin",
    "Bobbob@example.com · Joined Oct 2, 2026EditorRemove",
  ]);
  const settings = screen.getByRole("navigation", { name: "Notebook settings" });
  expect(within(settings).getByRole("link", { name: "Members" }).getAttribute("aria-current")).toBe("page");
  expect(screen.queryByRole("button", { name: /role of Ada/ })).toBeNull();
  expect(screen.queryByRole("button", { name: /^Remove Ada/ })).toBeNull();
});

test("a guest of the workspace sees the members without their addresses", async () => {
  renderApp(
    members,
    notebookServer({ workspaceRole: "guest", members: [notebookMember(ada, "reader"), notebookMember(bob, "admin")] })
      .app
  );

  expect(await rows()).toEqual(["AdaYouJoined Oct 1, 2026Reader", "BobJoined Oct 2, 2026Admin"]);
});

test("an admin changes another member's role as one is chosen, and keeps the focus", async () => {
  const user = userEvent.setup();
  const server = notebookServer();
  renderApp(members, server.app);

  await choose(user, await roleButton("Editor", "Bob (bob@example.com)"), "Reader");

  const changed = await roleButton("Reader", "Bob (bob@example.com)");
  expect(server.sent).toEqual(["GET members", "GET workspace members", "PATCH Bob reader"]);
  await waitFor(() => expect(document.activeElement).toBe(changed));
});

test.each(["editor", "reader"] as const)("its %s sees the roles, and can change none nor add", async (role) => {
  renderApp(members, notebookServer({ members: [notebookMember(ada, role), notebookMember(bob, "admin")] }).app);

  expect(await rows()).toEqual([
    `AdaYouada@example.com · Joined Oct 1, 2026${role === "editor" ? "Editor" : "Reader"}`,
    "Bobbob@example.com · Joined Oct 2, 2026Admin",
  ]);
  expect(screen.queryByRole("button", { name: /role of/ })).toBeNull();
  expect(screen.queryByRole("button", { name: /^Remove/ })).toBeNull();
  expect(screen.queryByRole("heading", { name: "Add a member" })).toBeNull();
});

test("a change of role refused says why above the list; the lists read again, the controls follow the account's role", async () => {
  const user = userEvent.setup();
  const server = notebookServer({
    answers: { "PATCH /api/v0/notebook-members/*": () => problem(403, "forbidden") },
  });
  renderApp(members, server.app);
  const button = await roleButton("Editor", "Bob (bob@example.com)");
  // Elsewhere: Bob made himself the admin, and Ada an editor.
  server.members = [notebookMember(ada, "editor"), notebookMember(bob, "admin")];

  await choose(user, button, "Admin");

  expect((await screen.findByRole("alert")).textContent).toBe("You do not have permission to do this.");
  await waitFor(() => expect(screen.queryByRole("button", { name: /role of Bob/ })).toBeNull());
  expect(await rows()).toEqual([
    "AdaYouada@example.com · Joined Oct 1, 2026Editor",
    "Bobbob@example.com · Joined Oct 2, 2026Admin",
  ]);
  expect(screen.queryByRole("heading", { name: "Add a member" })).toBeNull();
});

test("a member removed leaves the list, the focus on its heading; the notebook becomes the account's own", async () => {
  const user = userEvent.setup();
  const server = notebookServer();
  renderApp(members, server.app);
  await within(await nav()).findByRole("list", { name: "Team notebooks" });

  await user.click(await screen.findByRole("button", { name: "Remove Bob (bob@example.com)" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Remove Bob from Plans?" });
  await user.click(within(dialog).getByRole("button", { name: "Remove" }));

  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("heading", { name: "Members" })));
  expect(await rows()).toEqual(["AdaYouada@example.com · Joined Oct 1, 2026Admin"]);
  expect(await within(await nav()).findByRole("list", { name: "My notebooks" })).toBeTruthy();
});

/** The member select of the add form, and the names it offers. */
async function candidates(): Promise<string[]> {
  const select = await screen.findByLabelText<HTMLSelectElement>("Member");
  return [...select.options].map((option) => option.textContent ?? "");
}

test("an admin adds a member of the workspace not in the notebook yet, with a role", async () => {
  const user = userEvent.setup();
  const server = notebookServer({ members: [notebookMember(ada, "admin")] });
  renderApp(members, server.app);
  await within(await nav()).findByRole("list", { name: "My notebooks" });

  expect(await candidates()).toEqual(["Choose a member", "Bob (bob@example.com)", "Cy (cy@example.com)"]);
  expect(screen.getByLabelText<HTMLSelectElement>("Role").value).toBe("editor");
  await user.selectOptions(screen.getByLabelText("Member"), "Cy (cy@example.com)");
  await user.selectOptions(screen.getByLabelText("Role"), "Reader");
  await user.click(screen.getByRole("button", { name: "Add" }));

  expect((await screen.findByRole("status")).textContent).toBe("Cy added.");
  expect(server.sent).toContain("POST Cy reader");
  expect(await rows()).toEqual([
    "AdaYouada@example.com · Joined Oct 1, 2026Admin",
    "Cycy@example.com · Joined Oct 3, 2026ReaderRemove",
  ]);
  expect(await candidates()).toEqual(["Choose a member", "Bob (bob@example.com)"]);
  expect(screen.getByLabelText<HTMLSelectElement>("Member").value).toBe("");
  // Two members: the notebook is the team's now.
  expect(await within(await nav()).findByRole("list", { name: "Team notebooks" })).toBeTruthy();
});

test("adding with no member chosen is found before sending", async () => {
  const user = userEvent.setup();
  const server = notebookServer();
  renderApp(members, server.app);
  await candidates();

  await user.click(screen.getByRole("button", { name: "Add" }));

  expect(await screen.findByText("Required.")).toBeTruthy();
  await waitFor(() => expect(document.activeElement).toBe(screen.getByLabelText("Member")));
  expect(server.sent.filter((sent) => sent.startsWith("POST"))).toEqual([]);
});

test.each([
  ["not_allowed", "No longer a member of the workspace."],
  ["duplicate", "A member of this notebook already."],
])("an addition refused as %s says why under the member, and reads both lists again", async (code, message) => {
  const user = userEvent.setup();
  const server = notebookServer({
    answers: {
      [`POST ${plans}/members`]: () =>
        problem(422, "validation_failed", { errors: [{ field: "user_id", code, message: "…" }] }),
    },
  });
  renderApp(members, server.app);
  await candidates();
  // Elsewhere: Cy left the workspace.
  server.workspaceMembers = server.workspaceMembers.filter((m) => m.user_id !== cy.user_id);

  await user.selectOptions(screen.getByLabelText("Member"), "Cy (cy@example.com)");
  await user.click(screen.getByRole("button", { name: "Add" }));

  await waitFor(async () => expect(await candidates()).toEqual(["Choose a member"]));
  expect(screen.getByText(message)).toBeTruthy();
  expect(screen.queryByText("Every member of the workspace is in this notebook already.")).toBeNull();
  await waitFor(() => expect(document.activeElement).toBe(screen.getByLabelText("Member")));
  expect(server.sent.filter((sent) => sent.startsWith("GET"))).toEqual([
    "GET members",
    "GET workspace members",
    "GET workspace members",
    "GET members",
  ]);
});

test("with every member of the workspace in the notebook, the admin reads so", async () => {
  renderApp(
    members,
    notebookServer({
      members: [notebookMember(ada, "admin"), notebookMember(bob, "editor"), notebookMember(cy, "reader")],
    }).app
  );

  expect(await candidates()).toEqual(["Choose a member"]);
  expect(screen.getByLabelText("Member").getAttribute("aria-describedby")).not.toBeNull();
  expect(screen.getByText("Every member of the workspace is in this notebook already.")).toBeTruthy();
});

test("one who sees the notebook by its access only has no membership to leave", async () => {
  renderApp(members, notebookServer({ access: "editor", members: [notebookMember(bob, "admin")] }).app);

  expect(await rows()).toEqual(["Bobbob@example.com · Joined Oct 2, 2026Admin"]);
  expect(screen.queryByRole("button", { name: "Leave notebook" })).toBeNull();
});

async function leave(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole("button", { name: "Leave notebook" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Leave Plans?" });
  await user.click(within(dialog).getByRole("button", { name: "Leave" }));
  return dialog;
}

test("leaving a private notebook lands on the workspace's home, its heading focused", async () => {
  const user = userEvent.setup();
  const server = notebookServer({ members: [notebookMember(ada, "editor"), notebookMember(bob, "admin")] });
  const { router } = renderApp(members, server.app);

  await leave(user);

  const home = await screen.findByRole("heading", { level: 1, name: "Lab" });
  expect(router.state.location.pathname).toBe("/lab");
  await waitFor(() => expect(document.activeElement).toBe(home));
  expect(screen.queryByRole("heading", { name: "Page not found" })).toBeNull();
  expect(server.sent).toContain("leave");
});

test("leaving a notebook open to the workspace stays on it, with its role by the access; the list is read again", async () => {
  const user = userEvent.setup();
  const server = notebookServer({
    access: "viewer",
    members: [notebookMember(ada, "editor"), notebookMember(bob, "admin")],
  });
  const { router } = renderApp(members, server.app);

  await leave(user);

  expect(await rows()).toEqual(["Bobbob@example.com · Joined Oct 2, 2026Admin"]);
  expect(router.state.location.pathname).toBe(members);
  expect(screen.queryByRole("button", { name: "Leave notebook" })).toBeNull();
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("heading", { name: "Members" })));
});

test.each([
  [
    "the only admin",
    problem(409, "notebook.sole_admin"),
    "You are this notebook's only admin: make another member an admin first, or delete the notebook.",
  ],
  [
    "a membership ended already",
    problem(404, "notebook.member_not_found"),
    "You are no longer a member of this notebook.",
  ],
])("%s: the dialog says why, and the notebook stays", async (_name, answer, message) => {
  const user = userEvent.setup();
  const server = notebookServer({ answers: { [`POST ${plans}/leave`]: () => answer.clone() } });
  renderApp(members, server.app);

  const dialog = await leave(user);

  expect((await within(dialog).findByRole("alert")).textContent).toBe(message);
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  expect(within(await nav()).getByRole("link", { name: "Plans" })).toBeTruthy();
});

afterEach(() => vi.useRealTimers());

test("the members read again show what changed elsewhere", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = notebookServer();
  const { router } = renderApp(members, server.app);
  await rows();

  server.members = [...server.members, notebookMember(cy, "reader")];
  await act(() => router.navigate(`/lab/notebooks/${notebookJSON.id}/settings/general`));
  await act(() => vi.advanceTimersByTimeAsync(2_000));
  await act(() => router.navigate(members));

  await waitFor(async () => expect(await rows()).toHaveLength(3));
});

test("the members say so when they cannot be read; Try again reads them", async () => {
  const user = userEvent.setup();
  let down = true;
  const app = signedInApp({
    "GET /api/v0/workspaces/lab/notebooks": () => json({ data: [notebookJSON] }),
    [`GET ${plans}/members`]: () =>
      down ? Promise.reject(new TypeError("offline")) : json({ data: [notebookMember(ada, "admin")] }),
    "GET /api/v0/workspaces/lab/members": () => json({ data: [ada] }),
  });
  renderApp(members, app);

  expect((await screen.findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );
  down = false;
  await user.click(screen.getByRole("button", { name: "Try again" }));

  expect(await rows()).toEqual(["AdaYouada@example.com · Joined Oct 1, 2026Admin"]);
});

// Each notebook's pages read its own members (v0.1 design 13.2, item 15).
test("going from one notebook's members to another's lists the other's", async () => {
  const user = userEvent.setup();
  const atlas: Notebook = {
    ...notebookJSON,
    id: "0199a2b4-0000-7000-8000-0000000000c2",
    name: "Atlas",
    member_count: 2,
  };
  renderApp(
    members,
    signedInApp({
      "GET /api/v0/workspaces/lab/notebooks": () => json({ data: [atlas, notebookJSON] }),
      [`GET ${plans}/members`]: () => json({ data: [notebookMember(ada, "admin")] }),
      [`GET /api/v0/notebooks/${atlas.id}/members`]: () =>
        json({ data: [notebookMember(ada, "admin"), notebookMember(bob, "reader")] }),
      "GET /api/v0/workspaces/lab/members": () => json({ data: [ada, bob] }),
    })
  );
  expect(await rows()).toHaveLength(1);

  await user.click(within(await nav()).getByRole("link", { name: "Atlas" }));
  await user.click(await screen.findByRole("link", { name: "Notebook settings" }));
  await user.click(await screen.findByRole("link", { name: "Members" }));

  await waitFor(async () => expect(await rows()).toHaveLength(2));
});
