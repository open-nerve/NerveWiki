import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import type { WorkspaceInvitation } from "../../services/invitation.service";
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

const toBob: WorkspaceInvitation = {
  id: "0199a2b4-0000-7000-8000-0000000000e1",
  email: "bob@example.com",
  role: "admin",
  token: "nwk_inv_bob",
  created_at: "2026-10-03T08:00:00Z",
};

/**
 * The server of Lab's members page, as the account sees it with role:
 * Ada (the account), Bob and Cy, and an invitation pending to Bob's
 * address (the server's administrator gave it to him); the account's
 * workspaces are Lab and Acme, whose one member is Ada. Changes answer as
 * update, remove and leave say, or succeed; what went out is in sent.
 * The test changes members, invitations, mine (the account's role) and
 * down (Lab's members cannot be read) as the server would.
 */
function membersServer({
  role = "admin",
  update,
  remove,
  leave,
}: { role?: Workspace["role"]; update?: Answer; remove?: Answer; leave?: Answer } = {}) {
  const server = {
    sent: [] as string[],
    mine: role,
    down: false,
    members: [{ ...ada, role }, bob, cy],
    invitations: [toBob],
  };
  let left = false;
  const memberOf = (request: Request) => server.members.find((m) => request.url.endsWith(m.id)) ?? bob;
  const app = signedInApp({
    "GET /api/v0/workspaces": () => json({ data: left ? [acme] : [acme, { ...workspaceJSON, role: server.mine }] }),
    "GET /api/v0/workspaces/lab/members": () => {
      server.sent.push("GET members");
      if (server.down) {
        return Promise.reject(new TypeError("offline"));
      }
      const members = server.members;
      return json({ data: server.mine === "guest" ? members.map((m) => ({ ...m, email: null })) : members });
    },
    "GET /api/v0/workspaces/lab/invitations": () => json({ data: server.invitations }),
    "GET /api/v0/workspaces/acme/members": () => json({ data: [ada] }),
    "GET /api/v0/workspaces/acme/invitations": () => json({ data: [] }),
    "PATCH /api/v0/workspace-members/*": async (request) => {
      const { role: changed } = (await request.clone().json()) as { role: Workspace["role"] };
      const member = memberOf(request);
      server.sent.push(`PATCH ${member.display_name} ${changed}`);
      const answer = (await update?.(request)) ?? json({ ...member, role: changed });
      if (answer.ok) {
        server.members = server.members.map((m) => (m.id === member.id ? { ...m, role: changed } : m));
      }
      return answer;
    },
    "DELETE /api/v0/workspace-members/*": async (request) => {
      const member = memberOf(request);
      server.sent.push(`DELETE ${member.display_name}`);
      server.members = server.members.filter((m) => m.id !== member.id);
      server.invitations = server.invitations.filter((i) => i.email !== member.email);
      return (await remove?.(request)) ?? new Response(null, { status: 204 });
    },
    "POST /api/v0/workspaces/lab/leave": async (request) => {
      server.sent.push("leave");
      const answer = (await leave?.(request)) ?? new Response(null, { status: 204 });
      left = answer.ok;
      return answer;
    },
  });
  return Object.assign(server, { app });
}

/** The rows of the members list, each as its text. */
async function rows(): Promise<string[]> {
  const list = await screen.findByRole("list", { name: "Members" });
  return within(list)
    .getAllByRole("listitem")
    .map((item) => item.textContent ?? "");
}

/** The names of the members listed. */
async function names(): Promise<string[]> {
  const list = await screen.findByRole("list", { name: "Members" });
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

/** The button of the role of the member who, which reads it as role. */
const roleButton = (role: string, who: string) => screen.findByRole("button", { name: `${role}, role of ${who}` });

/** Chooses role in the menu of button. */
async function choose(user: ReturnType<typeof userEvent.setup>, button: HTMLElement, role: string) {
  await user.click(button);
  await user.click(await screen.findByRole("menuitemradio", { name: role }));
}

test("an admin changes another member's role as one is chosen, and keeps the focus; their own row has no controls", async () => {
  const user = userEvent.setup();
  const { app, sent } = membersServer();
  renderApp("/lab/settings/members", app);

  await choose(user, await roleButton("Member", "Bob (bob@example.com)"), "Guest");

  const changed = await roleButton("Guest", "Bob (bob@example.com)");
  expect(sent).toEqual(["GET members", "PATCH Bob guest"]);
  await waitFor(() => expect(document.activeElement).toBe(changed));
  expect(screen.queryByRole("button", { name: /role of Ada/ })).toBeNull();
  expect(screen.queryByRole("button", { name: /^Remove Ada/ })).toBeNull();
  expect(screen.getByRole("button", { name: "Remove Cy (cy@example.com)" })).toBeTruthy();
});

test.each(["member", "guest"] as const)("a %s sees the roles, and can change none", async (role) => {
  renderApp("/lab/settings/members", membersServer({ role }).app);

  await rows();
  expect(screen.queryByRole("button", { name: /role of/ })).toBeNull();
  expect(screen.queryByRole("button", { name: /^Remove/ })).toBeNull();
});

test("one change of role goes out at a time; a refused one leaves the role, and says why until the next", async () => {
  const user = userEvent.setup();
  let refuse: ((answer: Response) => void) | undefined;
  let patches = 0;
  const server = membersServer({
    update: () =>
      ++patches === 1 ? new Promise<Response>((resolve) => (refuse = resolve)) : json({ ...bob, role: "admin" }),
  });
  renderApp("/lab/settings/members", server.app);

  const button = await roleButton("Member", "Bob (bob@example.com)");
  await choose(user, button, "Guest");
  expect(button.getAttribute("aria-busy")).toBe("true");
  // Chosen while the first is out: not taken.
  await choose(user, button, "Admin");
  expect(server.sent.filter((request) => request.startsWith("PATCH"))).toEqual(["PATCH Bob guest"]);

  refuse?.(problem(403, "forbidden"));
  expect((await screen.findByRole("alert")).textContent).toBe("You do not have permission to do this.");
  expect(await roleButton("Member", "Bob (bob@example.com)")).toBe(button);

  await choose(user, button, "Admin");
  expect(await roleButton("Admin", "Bob (bob@example.com)")).toBeTruthy();
  expect(screen.queryByRole("alert")).toBeNull();
  expect(server.sent.filter((request) => request.startsWith("PATCH"))).toEqual(["PATCH Bob guest", "PATCH Bob admin"]);
});

test("a change refused reads the list and the workspaces again: the controls follow the account's role", async () => {
  const user = userEvent.setup();
  const server = membersServer({
    update: () => {
      server.mine = "member";
      server.members = server.members.map((m) => (m.id === ada.id ? { ...m, role: "member" } : m));
      return problem(403, "forbidden");
    },
  });
  renderApp("/lab/settings/members", server.app);

  await choose(user, await roleButton("Member", "Bob (bob@example.com)"), "Admin");

  expect((await screen.findByRole("alert")).textContent).toBe("You do not have permission to do this.");
  await waitFor(() => expect(screen.queryByRole("button", { name: /role of/ })).toBeNull());
  expect(screen.queryByRole("heading", { name: "Invitations" })).toBeNull();
  expect((await rows())[0]).toBe("AdaYouada@example.com · Joined Oct 1, 2026Member");
  expect(server.sent).toEqual(["GET members", "PATCH Bob admin", "GET members"]);
});

test.each([
  ["removed", undefined],
  ["gone already", () => problem(404, "workspace.member_not_found")],
])("an admin removes a member once confirmed, and the invitations pending to them go too: %s", async (_, remove) => {
  const user = userEvent.setup();
  const { app, sent } = membersServer({ remove });
  renderApp("/lab/settings/members", app);
  expect(await screen.findByText("bob@example.com", { selector: "p" })).toBeTruthy();

  await user.click(await screen.findByRole("button", { name: "Remove Bob (bob@example.com)" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Remove Bob from Lab?" });
  await user.click(within(dialog).getByRole("button", { name: "Remove" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(await names()).toEqual(["Ada", "Cy"]);
  expect(document.activeElement).toBe(screen.getByRole("heading", { name: "Members" }));
  expect(await screen.findByText("No invitations pending.")).toBeTruthy();
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

afterEach(() => vi.useRealTimers());

test("the lists read again show what changed elsewhere", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = membersServer();
  const { router } = renderApp("/lab/settings/members", server.app);
  expect(await names()).toEqual(["Ada", "Bob", "Cy"]);
  expect(await screen.findByText("bob@example.com", { selector: "p" })).toBeTruthy();

  // Elsewhere, Cy leaves and the invitation is withdrawn; coming back reads both lists again.
  server.members = server.members.filter((m) => m.id !== cy.id);
  server.invitations = [];
  await act(() => router.navigate("/lab/settings/general"));
  await act(() => vi.advanceTimersByTimeAsync(2_000));
  await act(() => router.navigate("/lab/settings/members"));

  await waitFor(async () => expect(await names()).toEqual(["Ada", "Bob"]));
  expect(await screen.findByText("No invitations pending.")).toBeTruthy();
});

test("each workspace's members page lists its own members and invitations", async () => {
  const server = membersServer();
  const { router } = renderApp("/lab/settings/members", server.app);
  expect(await names()).toEqual(["Ada", "Bob", "Cy"]);
  expect(await screen.findByRole("button", { name: "Copy link: bob@example.com" })).toBeTruthy();

  await act(() => router.navigate("/acme/settings/members"));

  await waitFor(async () => expect(await names()).toEqual(["Ada"]));
  expect(await screen.findByText("No invitations pending.")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Copy link: bob@example.com" })).toBeNull();
});

test("the members say so when they cannot be read; Try again reads them", async () => {
  const user = userEvent.setup();
  const server = membersServer({ role: "member" });
  server.down = true;
  renderApp("/lab/settings/members", server.app);
  expect((await screen.findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );

  server.down = false;
  await user.click(screen.getByRole("button", { name: "Try again" }));

  expect(await names()).toEqual(["Ada", "Bob", "Cy"]);
});
