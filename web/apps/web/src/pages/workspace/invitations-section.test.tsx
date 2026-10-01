import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";

import type { WorkspaceInvitation, WorkspaceInvitationCreate } from "../../services/invitation.service";
import type { WorkspaceMember } from "../../services/member.service";
import type { Workspace } from "../../services/workspace.service";
import { json, problem, signedInApp, userJSON, workspaceJSON, type Answer } from "../../test/fakes";
import { renderApp } from "../../test/render";

// The invitations of the members page (M2/P6 design 3.3).

const ada: WorkspaceMember = {
  id: "0199a2b4-0000-7000-8000-0000000000d1",
  user_id: userJSON.id,
  role: "admin",
  display_name: "Ada",
  email: "ada@example.com",
  created_at: "2026-10-01T08:00:00Z",
};
const bob: WorkspaceInvitation = {
  id: "0199a2b4-0000-7000-8000-0000000000e1",
  email: "bob@example.com",
  role: "member",
  token: "nwk_inv_bob",
  created_at: "2026-10-01T09:00:00Z",
};
const cy: WorkspaceInvitation = {
  id: "0199a2b4-0000-7000-8000-0000000000e2",
  email: "cy@example.com",
  role: "guest",
  token: "nwk_inv_cy",
  created_at: "2026-10-02T09:00:00Z",
};

/**
 * The server of Lab's members page, as the account sees it with role, Lab
 * having the invitations pending; create and remove answer as they say, or
 * succeed; what went out is in sent.
 */
function invitationsServer({
  role = "admin",
  pending = [cy, bob],
  create,
  remove,
}: { role?: Workspace["role"]; pending?: WorkspaceInvitation[]; create?: Answer; remove?: Answer } = {}) {
  const sent: string[] = [];
  let list = pending;
  const app = signedInApp({
    "GET /api/v0/workspaces": () => json({ data: [{ ...workspaceJSON, role }] }),
    "GET /api/v0/workspaces/lab/members": () => json({ data: [{ ...ada, role }] }),
    "GET /api/v0/workspaces/lab/invitations": () => {
      sent.push("GET invitations");
      return json({ data: list });
    },
    "POST /api/v0/workspaces/lab/invitations": async (request) => {
      const body = (await request.clone().json()) as WorkspaceInvitationCreate;
      sent.push(`POST ${body.email} ${body.role}`);
      const created = {
        ...bob,
        id: "0199a2b4-0000-7000-8000-0000000000e3",
        token: "nwk_inv_dee",
        ...body,
        email: body.email.toLowerCase(),
      };
      const answer = (await create?.(request)) ?? json(created, 201);
      if (answer.ok) {
        list = [created, ...list];
      }
      return answer;
    },
    "DELETE /api/v0/workspace-invitations/*": async (request) => {
      const gone = list.find((i) => request.url.endsWith(i.id));
      sent.push(`DELETE ${gone?.email}`);
      list = list.filter((i) => i !== gone);
      return (await remove?.(request)) ?? new Response(null, { status: 204 });
    },
  });
  return { app, sent };
}

/** The invitations listed, each as its address and its line under it. */
async function pendingList(): Promise<string[][]> {
  const list = await screen.findByRole("list", { name: "Invitations" });
  return within(list)
    .getAllByRole("listitem")
    .map((item) => [...item.querySelectorAll("p")].map((p) => p.textContent ?? ""));
}

test("an admin sees the invitations pending, newest first", async () => {
  renderApp("/lab/settings/members", invitationsServer().app);

  expect(await pendingList()).toEqual([
    ["cy@example.com", "Guest · invited Oct 2, 2026"],
    ["bob@example.com", "Member · invited Oct 1, 2026"],
  ]);
});

test("without invitations pending, the section says so", async () => {
  renderApp("/lab/settings/members", invitationsServer({ pending: [] }).app);

  expect(await screen.findByText("No invitations pending.")).toBeTruthy();
});

test.each(["member", "guest"] as const)("a %s sees no invitations, which are not even read", async (role) => {
  const { app, sent } = invitationsServer({ role });
  renderApp("/lab/settings/members", app);

  expect(await screen.findByRole("heading", { name: "Members" })).toBeTruthy();
  await screen.findByRole("list", { name: "Members" });
  expect(screen.queryByRole("heading", { name: "Invitations" })).toBeNull();
  expect(sent).toEqual([]);
});

test("an admin invites an address as a role; the invitation comes first, and the field empties", async () => {
  const user = userEvent.setup();
  const { app, sent } = invitationsServer();
  renderApp("/lab/settings/members", app);

  await user.type(await screen.findByLabelText("E-mail address"), "Dee@Example.com");
  await user.selectOptions(screen.getByLabelText("Role"), "admin");
  await user.click(screen.getByRole("button", { name: "Invite" }));

  expect(await screen.findByText("Invited dee@example.com. Copy the link and send it to them.")).toBeTruthy();
  expect(sent).toEqual(["GET invitations", "POST Dee@Example.com admin"]);
  expect((await pendingList()).map(([email]) => email)).toEqual([
    "dee@example.com",
    "cy@example.com",
    "bob@example.com",
  ]);
  expect(screen.getByLabelText<HTMLInputElement>("E-mail address").value).toBe("");
});

test.each([
  ["", undefined, "Required."],
  ["bob@example.com", "duplicate", "Already invited: copy the link of that invitation below."],
  ["ada@example.com", "not_allowed", "Already a member of this workspace."],
])("an address refused shows why under the field: %j", async (email, code, message) => {
  const user = userEvent.setup();
  const { app, sent } = invitationsServer({
    create: () => problem(422, "validation_failed", { errors: [{ field: "email", code, message: "refused" }] }),
  });
  renderApp("/lab/settings/members", app);

  const field = await screen.findByLabelText<HTMLInputElement>("E-mail address");
  if (email !== "") {
    await user.type(field, email);
  }
  await user.click(screen.getByRole("button", { name: "Invite" }));

  expect(await screen.findByText(message)).toBeTruthy();
  await waitFor(() => expect(document.activeElement).toBe(field));
  expect(field.value).toBe(email);
  expect(sent.length).toBe(email === "" ? 1 : 2);
});

test("an invitation's link goes to the clipboard, its token in the fragment", async () => {
  const user = userEvent.setup();
  renderApp("/lab/settings/members", invitationsServer().app);

  await user.click(await screen.findByRole("button", { name: "Copy the link of the invitation to bob@example.com" }));

  expect(await screen.findByText("Copied the link of the invitation to bob@example.com.")).toBeTruthy();
  expect(await navigator.clipboard.readText()).toBe(`${window.location.origin}/invitations/${bob.id}#nwk_inv_bob`);
  expect(screen.queryByRole("textbox", { name: /^Link of/ })).toBeNull();
});

test("where the browser cannot copy, the link shows in a field, selected", async () => {
  const user = userEvent.setup();
  vi.spyOn(navigator.clipboard, "writeText").mockRejectedValue(new DOMException("denied", "NotAllowedError"));
  renderApp("/lab/settings/members", invitationsServer().app);

  await user.click(await screen.findByRole("button", { name: "Copy the link of the invitation to cy@example.com" }));

  const field = await screen.findByRole<HTMLInputElement>("textbox", {
    name: "Link of the invitation to cy@example.com",
  });
  expect(field.value).toBe(`${window.location.origin}/invitations/${cy.id}#nwk_inv_cy`);
  expect(field.readOnly).toBe(true);
  expect(document.activeElement).toBe(field);
  expect([field.selectionStart, field.selectionEnd]).toEqual([0, field.value.length]);
  expect(screen.queryByText(/^Copied/)).toBeNull();
});

test.each([
  ["withdrawn", undefined],
  ["gone already", () => problem(404, "workspace.invitation_not_found")],
])("an admin withdraws an invitation once confirmed: %s", async (_, remove) => {
  const user = userEvent.setup();
  const { app, sent } = invitationsServer({ remove });
  renderApp("/lab/settings/members", app);

  await user.click(await screen.findByRole("button", { name: "Withdraw the invitation to bob@example.com" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Withdraw the invitation to bob@example.com?" });
  await user.click(within(dialog).getByRole("button", { name: "Withdraw" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect((await pendingList()).map(([email]) => email)).toEqual(["cy@example.com"]);
  expect(document.activeElement).toBe(screen.getByRole("heading", { name: "Invitations" }));
  expect(sent).toEqual(["GET invitations", "DELETE bob@example.com"]);
});
