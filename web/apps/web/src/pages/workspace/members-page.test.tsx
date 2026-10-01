import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import type { WorkspaceMember } from "../../services/member.service";
import type { Workspace } from "../../services/workspace.service";
import { json, problem, signedInApp, userJSON, workspaceJSON, type Answer } from "../../test/fakes";
import { renderApp } from "../../test/render";
import { watchFor } from "../../test/watch";

// A workspace's members page: the members and leaving (M2/P6 design 3.3).

const acme: Workspace = { ...workspaceJSON, id: "0199a2b4-0000-7000-8000-0000000000a1", slug: "acme", name: "Acme" };
const ada: WorkspaceMember = {
  id: "0199a2b4-0000-7000-8000-0000000000d1",
  user_id: userJSON.id,
  role: "admin",
  display_name: "Ada",
  email: "ada@example.com",
  created_at: "2026-10-01T08:00:00Z",
};
const bob: WorkspaceMember = {
  ...ada,
  id: "0199a2b4-0000-7000-8000-0000000000d2",
  user_id: "0199a2b4-0000-7000-8000-000000000002",
  role: "member",
  display_name: "Bob",
  email: "bob@example.com",
  created_at: "2026-10-02T08:00:00Z",
};
const cy: WorkspaceMember = {
  ...bob,
  id: "0199a2b4-0000-7000-8000-0000000000d3",
  user_id: "0199a2b4-0000-7000-8000-000000000003",
  role: "guest",
  display_name: "Cy",
  email: "cy@example.com",
};

/**
 * The server of Lab's members page, as the account sees it with role:
 * Ada (the account), Bob and Cy; the account's workspaces are Lab and
 * Acme. Changes answer as update, remove and leave say, or succeed; what
 * went out is in sent. demote makes the account a member of Lab.
 */
function membersServer({
  role = "admin",
  update,
  remove,
  leave,
}: { role?: Workspace["role"]; update?: Answer; remove?: Answer; leave?: Answer } = {}) {
  const sent: string[] = [];
  let mine: Workspace["role"] = role;
  let left = false;
  let members = [{ ...ada, role }, bob, cy];
  const memberOf = (request: Request) => members.find((m) => request.url.endsWith(m.id)) ?? bob;
  const app = signedInApp({
    "GET /api/v0/workspaces": () => json({ data: left ? [acme] : [acme, { ...workspaceJSON, role: mine }] }),
    "GET /api/v0/workspaces/lab/members": () => {
      sent.push("GET members");
      return json({ data: mine === "guest" ? members.map((m) => ({ ...m, email: null })) : members });
    },
    "PATCH /api/v0/workspace-members/*": async (request) => {
      const { role: changed } = (await request.json()) as { role: Workspace["role"] };
      const member = memberOf(request);
      sent.push(`PATCH ${member.display_name} ${changed}`);
      const answer = (await update?.(request)) ?? json({ ...member, role: changed });
      if (answer.ok) {
        members = members.map((m) => (m.id === member.id ? { ...m, role: changed } : m));
      }
      return answer;
    },
    "DELETE /api/v0/workspace-members/*": async (request) => {
      const member = memberOf(request);
      sent.push(`DELETE ${member.display_name}`);
      members = members.filter((m) => m.id !== member.id);
      return (await remove?.(request)) ?? new Response(null, { status: 204 });
    },
    "POST /api/v0/workspaces/lab/leave": async (request) => {
      sent.push("leave");
      const answer = (await leave?.(request)) ?? new Response(null, { status: 204 });
      left = answer.ok;
      return answer;
    },
  });
  return {
    app,
    sent,
    demote: () => {
      mine = "member";
    },
  };
}

/** The rows of the members list, each as its text. */
async function rows(): Promise<string[]> {
  const list = await screen.findByRole("list");
  return within(list)
    .getAllByRole("listitem")
    .map((item) => item.textContent ?? "");
}

/** The names of the members listed. */
async function names(): Promise<string[]> {
  const list = await screen.findByRole("list");
  return within(list)
    .getAllByRole("listitem")
    .map((item) => item.querySelector("p")?.firstChild?.textContent ?? "");
}

test("the members are listed by when they joined, with the account's own row marked", async () => {
  renderApp("/lab/settings/members", membersServer({ role: "member" }).app);

  expect(await rows()).toEqual([
    "AdaYouada@example.com · Joined Oct 1, 2026Member",
    "Bobbob@example.com · Joined Oct 2, 2026Member",
    "Cycy@example.com · Joined Oct 2, 2026Guest",
  ]);
  const nav = screen.getByRole("navigation", { name: "Workspace settings" });
  expect(within(nav).getByRole("link", { name: "Members" }).getAttribute("aria-current")).toBe("page");
});

test("a guest sees the members without their addresses", async () => {
  renderApp("/lab/settings/members", membersServer({ role: "guest" }).app);

  expect(await rows()).toEqual([
    "AdaYouJoined Oct 1, 2026Guest",
    "BobJoined Oct 2, 2026Member",
    "CyJoined Oct 2, 2026Guest",
  ]);
});

test("an admin changes another member's role as it is chosen; their own row has no controls", async () => {
  const user = userEvent.setup();
  const { app, sent } = membersServer();
  renderApp("/lab/settings/members", app);

  const bobsRole = await screen.findByRole("combobox", { name: "Role of Bob" });
  await user.selectOptions(bobsRole, "guest");

  await waitFor(() => expect(sent).toEqual(["GET members", "PATCH Bob guest"]));
  await waitFor(() => expect(bobsRole).toHaveProperty("disabled", false));
  expect((bobsRole as HTMLSelectElement).value).toBe("guest");
  expect(screen.queryByRole("combobox", { name: "Role of Ada" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Remove Ada" })).toBeNull();
  expect(screen.getByRole("button", { name: "Remove Cy" })).toBeTruthy();
});

test.each(["member", "guest"] as const)("a %s sees the roles, and can change none", async (role) => {
  renderApp("/lab/settings/members", membersServer({ role }).app);

  await rows();
  expect(screen.queryByRole("combobox")).toBeNull();
  expect(screen.queryByRole("button", { name: /^Remove/ })).toBeNull();
});

test("a change refused says why; the list and the workspaces are read again, and the controls follow the account's role", async () => {
  const user = userEvent.setup();
  const server = membersServer({
    update: () => {
      server.demote();
      return problem(403, "forbidden");
    },
  });
  renderApp("/lab/settings/members", server.app);

  await user.selectOptions(await screen.findByRole("combobox", { name: "Role of Bob" }), "admin");

  expect((await screen.findByRole("alert")).textContent).toBe("You do not have permission to do this.");
  await waitFor(() => expect(screen.queryByRole("combobox")).toBeNull());
  expect(server.sent).toEqual(["GET members", "PATCH Bob admin", "GET members"]);
});

test.each([
  ["removed", undefined],
  ["gone already", () => problem(404, "workspace.member_not_found")],
])("an admin removes a member once confirmed: %s", async (_, remove) => {
  const user = userEvent.setup();
  const { app, sent } = membersServer({ remove });
  renderApp("/lab/settings/members", app);

  await user.click(await screen.findByRole("button", { name: "Remove Bob" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Remove Bob from Lab?" });
  await user.click(within(dialog).getByRole("button", { name: "Remove" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(await names()).toEqual(["Ada", "Cy"]);
  expect(document.activeElement).toBe(screen.getByRole("heading", { name: "Members" }));
  expect(sent).toEqual(["GET members", "DELETE Bob"]);
});

test("leaving lands on another workspace, with no 404 between", async () => {
  const user = userEvent.setup();
  const { app, sent } = membersServer({ role: "member" });
  const { router } = renderApp("/lab/settings/members", app);

  await user.click(await screen.findByRole("button", { name: "Leave workspace" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Leave Lab?" });
  const notFound = watchFor("Page not found");
  await user.click(within(dialog).getByRole("button", { name: "Leave" }));

  expect(await screen.findByRole("heading", { name: "Acme" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/acme");
  expect(notFound()).toBe(false);
  expect(sent).toEqual(["GET members", "leave"]);
});

test("the only admin cannot leave: the dialog says why, and the workspace stays", async () => {
  const user = userEvent.setup();
  const { app } = membersServer({ leave: () => problem(409, "workspace.sole_admin") });
  const { router } = renderApp("/lab/settings/members", app);

  await user.click(await screen.findByRole("button", { name: "Leave workspace" }));
  const dialog = await screen.findByRole("alertdialog");
  await user.click(within(dialog).getByRole("button", { name: "Leave" }));

  expect((await within(dialog).findByRole("alert")).textContent).toBe(
    "You are the workspace's only admin. Make another member an admin first, or, if no one else is in it, delete the workspace."
  );
  expect(router.state.location.pathname).toBe("/lab/settings/members");
});
