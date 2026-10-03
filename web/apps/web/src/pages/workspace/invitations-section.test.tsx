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
  const state = { down: false };
  let list = pending;
  const app = signedInApp({
    "GET /api/v0/workspaces": () => json({ data: [{ ...workspaceJSON, role }] }),
    "GET /api/v0/workspaces/lab/members": () => json({ data: [{ ...ada, role }] }),
    "GET /api/v0/workspaces/lab/invitations": () => {
      sent.push("GET invitations");
      return state.down ? Promise.reject(new TypeError("offline")) : json({ data: list });
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
  return { app, sent, state };
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

// An invitation answered after the field was edited again: the new draft stays, and the invitation made is said
// (R3 of the M2 Codex review).
test("an address typed while an invitation is out stays once it is made", async () => {
  const user = userEvent.setup();
  let release: (() => void) | undefined;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  const { app, sent } = invitationsServer({
    create: async () => {
      await held;
      return json({ ...bob, id: "0199a2b4-0000-7000-8000-0000000000e3", email: "dee@example.com" }, 201);
    },
  });
  renderApp("/lab/settings/members", app);
  const field = await screen.findByLabelText<HTMLInputElement>("E-mail address");

  await user.type(field, "dee@example.com");
  await user.click(screen.getByRole("button", { name: "Invite" }));
  await waitFor(() => expect(sent).toContain("POST dee@example.com member"));
  await user.clear(field);
  await user.type(field, "eve@example.com");
  release?.();

  expect(await screen.findByText("Invited dee@example.com. Copy the link and send it to them.")).toBeTruthy();
  expect(field.value).toBe("eve@example.com");
});

test.each([
  ["", undefined, "Required."],
  ["\u3000", undefined, "Required."],
  ["bob@example.com", "duplicate", "Already invited: copy the link of that invitation below."],
  ["ada@example.com", "not_allowed", "Already a member of this workspace."],
])("an address refused shows why under the field, and nothing above: %j", async (email, code, message) => {
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
  expect(screen.queryByRole("alert")).toBeNull();
  // A member, unless another role is chosen.
  expect(sent).toEqual(code === undefined ? ["GET invitations"] : ["GET invitations", `POST ${email} member`]);
});

test("a role refused shows why above the form: its select has no place for it", async () => {
  const user = userEvent.setup();
  const { app } = invitationsServer({
    create: () =>
      problem(422, "validation_failed", { errors: [{ field: "role", code: "not_allowed", message: "refused" }] }),
  });
  renderApp("/lab/settings/members", app);

  await user.type(await screen.findByLabelText("E-mail address"), "eve@example.com");
  await user.click(screen.getByRole("button", { name: "Invite" }));

  expect(await screen.findByRole("alert")).toBeTruthy();
});

test("the spaces an input method types around an address go before it is sent", async () => {
  const user = userEvent.setup();
  const { app, sent } = invitationsServer();
  renderApp("/lab/settings/members", app);

  await user.type(await screen.findByLabelText("E-mail address"), "\u3000dee@example.com\u3000");
  await user.click(screen.getByRole("button", { name: "Invite" }));

  expect(await screen.findByText("Invited dee@example.com. Copy the link and send it to them.")).toBeTruthy();
  expect(sent).toEqual(["GET invitations", "POST dee@example.com member"]);
});

test("an invitation's link goes to the clipboard, its token in the fragment", async () => {
  const user = userEvent.setup();
  renderApp("/lab/settings/members", invitationsServer().app);

  await user.click(await screen.findByRole("button", { name: "Copy link: bob@example.com" }));

  expect(await screen.findByText("Copied the link of the invitation to bob@example.com.")).toBeTruthy();
  expect(await navigator.clipboard.readText()).toBe(`${window.location.origin}/invitations/${bob.id}#nwk_inv_bob`);
  expect(screen.queryByRole("textbox", { name: /^Link of/ })).toBeNull();
});

test("where the browser refuses to copy, the link shows in a field, selected; copied after all, it goes", async () => {
  const user = userEvent.setup();
  renderApp("/lab/settings/members", invitationsServer().app);
  await user.click(await screen.findByRole("button", { name: "Copy link: bob@example.com" }));
  expect(await screen.findByText("Copied the link of the invitation to bob@example.com.")).toBeTruthy();
  const refused = vi
    .spyOn(navigator.clipboard, "writeText")
    .mockRejectedValue(new DOMException("denied", "NotAllowedError"));

  await user.click(screen.getByRole("button", { name: "Copy link: cy@example.com" }));

  const field = await screen.findByRole<HTMLInputElement>("textbox", {
    name: "Link of the invitation to cy@example.com",
  });
  expect(field.value).toBe(`${window.location.origin}/invitations/${cy.id}#nwk_inv_cy`);
  expect(field.readOnly).toBe(true);
  expect(document.activeElement).toBe(field);
  expect([field.selectionStart, field.selectionEnd]).toEqual([0, field.value.length]);
  expect(screen.queryByText(/^Copied/)).toBeNull();

  refused.mockRestore();
  await user.click(screen.getByRole("button", { name: "Copy link: cy@example.com" }));
  expect(await screen.findByText("Copied the link of the invitation to cy@example.com.")).toBeTruthy();
  expect(screen.queryByRole("textbox", { name: /^Link of/ })).toBeNull();
});

test("where the page has no clipboard (an address served over HTTP), the link shows in a field", async () => {
  const user = userEvent.setup();
  vi.spyOn(navigator, "clipboard", "get").mockReturnValue(undefined as unknown as Clipboard);
  renderApp("/lab/settings/members", invitationsServer().app);

  await user.click(await screen.findByRole("button", { name: "Copy link: cy@example.com" }));

  const field = await screen.findByRole<HTMLInputElement>("textbox", {
    name: "Link of the invitation to cy@example.com",
  });
  expect(field.value).toBe(`${window.location.origin}/invitations/${cy.id}#nwk_inv_cy`);
  expect(screen.queryByText(/^Copied/)).toBeNull();
});

test("the invitations say so when they cannot be read; Try again reads them", async () => {
  const user = userEvent.setup();
  const { app, state } = invitationsServer();
  state.down = true;
  renderApp("/lab/settings/members", app);
  expect((await screen.findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );

  state.down = false;
  await user.click(screen.getByRole("button", { name: "Try again" }));

  expect((await pendingList()).map(([email]) => email)).toEqual(["cy@example.com", "bob@example.com"]);
});

test.each([
  ["withdrawn", undefined],
  ["gone already", () => problem(404, "workspace.invitation_not_found")],
])("an admin withdraws an invitation once confirmed: %s", async (_, remove) => {
  const user = userEvent.setup();
  const { app, sent } = invitationsServer({ remove });
  renderApp("/lab/settings/members", app);
  await user.click(await screen.findByRole("button", { name: "Copy link: bob@example.com" }));
  expect(await screen.findByText("Copied the link of the invitation to bob@example.com.")).toBeTruthy();

  await user.click(await screen.findByRole("button", { name: "Withdraw the invitation to bob@example.com" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Withdraw the invitation to bob@example.com?" });
  await user.click(within(dialog).getByRole("button", { name: "Withdraw" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect((await pendingList()).map(([email]) => email)).toEqual(["cy@example.com"]);
  expect(document.activeElement).toBe(screen.getByRole("heading", { name: "Invitations" }));
  expect(screen.queryByText(/^Copied/)).toBeNull();
  expect(sent).toEqual(["GET invitations", "DELETE bob@example.com"]);
});
