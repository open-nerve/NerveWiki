import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import type { User } from "../services/account.service";
import type { Workspace } from "../services/workspace.service";
import {
  byRoute,
  instanceJSON,
  json,
  problem,
  signedInApp,
  testApp,
  tokensJSON,
  userJSON,
  workspaceJSON,
  type Answer,
} from "../test/fakes";
import { renderApp } from "../test/render";

// The invitation page (M2/P6 design 3.4).

const link = { id: "0199a2b4-0000-7000-8000-0000000000e1", token: "nwk_inv_lab" };
const page = `/invitations/${link.id}#${link.token}`;

const acme: Workspace = { ...workspaceJSON, id: "0199a2b4-0000-7000-8000-0000000000a1", slug: "acme", name: "Acme" };

/**
 * The server of the invitation page: link previews Lab, as a member;
 * accepting answers as accept says, by default Lab with the role member,
 * which joins the account's workspaces, Acme before it (/ would land on
 * Acme); me is the account signed in. Each request goes into sent, as its
 * method and address, and each body into bodies. While state.down names
 * a path, it cannot be reached; refresh answers the session's refresh.
 */
function invitationServer({
  signedIn = true,
  me = userJSON,
  accept,
  register,
  refresh = () => json(tokensJSON),
}: { signedIn?: boolean; me?: User; accept?: Answer; register?: Answer; refresh?: Answer } = {}) {
  const sent: string[] = [];
  const bodies: unknown[] = [];
  const state = { down: "" };
  let joined: Workspace[] = [acme];
  const routes: Record<string, Answer> = {
    "POST /api/v0/auth/refresh": refresh,
    "GET /api/v0/instance": () => json(instanceJSON),
    "POST /api/v0/auth/login": () => json(tokensJSON),
    "POST /api/v0/auth/register": async (request) => (await register?.(request)) ?? json(tokensJSON, 201),
    "POST /api/v0/auth/logout": () => new Response(null, { status: 204 }),
    "GET /api/v0/me": () => json(me),
    "GET /api/v0/workspaces": () => json({ data: joined }),
    [`POST /api/v0/workspace-invitations/${link.id}/preview`]: async (request) => {
      const { token } = (await request.clone().json()) as { token: string };
      return token === link.token
        ? json({ workspace: { name: "Lab", slug: "lab" }, role: "member" })
        : problem(404, "workspace.invitation_not_found");
    },
    [`POST /api/v0/workspace-invitations/${link.id}/accept`]: async (request) => {
      const answer = (await accept?.(request)) ?? json({ ...workspaceJSON, role: "member" });
      if (answer.ok) {
        joined = [acme, (await answer.clone().json()) as Workspace];
      }
      return answer;
    },
  };
  const answer = byRoute(routes);
  const record: Answer = async (request) => {
    sent.push(`${request.method} ${request.url}`);
    if (request.method !== "GET") {
      bodies.push(await request.clone().text());
    }
    if (state.down !== "" && new URL(request.url).pathname.endsWith(state.down)) {
      throw new TypeError("offline");
    }
    return answer(request);
  };
  const app = signedIn
    ? signedInApp(Object.fromEntries(Object.keys(routes).map((key) => [key, record])))
    : testApp(record);
  return { app, sent, bodies, state };
}

async function signInWith(email: string) {
  const user = userEvent.setup();
  await user.type(await screen.findByLabelText("E-mail address"), email);
  await user.type(screen.getByLabelText("Password"), "correct horse battery");
  await user.click(screen.getByRole("button", { name: "Sign in" }));
}

test("a link whose fragment lost its token says it no longer works, and asks nothing", async () => {
  const { app, sent } = invitationServer({ signedIn: false });
  renderApp(`/invitations/${link.id}`, app);

  expect(
    await screen.findByText(
      "This invitation no longer works: it may have been accepted or withdrawn, or the link is incomplete."
    )
  ).toBeTruthy();
  expect(sent.filter((request) => request.includes("workspace-invitations"))).toEqual([]);
});

test("a link the server does not know says it no longer works", async () => {
  renderApp(`/invitations/${link.id}#nwk_inv_other`, invitationServer({ signedIn: false }).app);

  expect((await screen.findByRole("alert")).textContent).toBe(
    "This invitation no longer works: it may have been accepted or withdrawn, or the link is incomplete."
  );
});

test("the same invitation with another token is another link: it is asked anew", async () => {
  const { router } = renderApp(`/invitations/${link.id}#nwk_inv_other`, invitationServer({ signedIn: false }).app);
  expect(await screen.findByRole("alert")).toBeTruthy();

  await act(() => router.navigate(page));

  expect(await screen.findByText("You are invited to join Lab.")).toBeTruthy();
});

test("signed out, one sees what the link invites to, signs in on the page, which stays, then accepts and goes in", async () => {
  const user = userEvent.setup();
  const { app, sent, bodies } = invitationServer({ signedIn: false });
  const { router } = renderApp(page, app);

  expect(await screen.findByText("You are invited to join Lab.")).toBeTruthy();
  expect(screen.getByText("Your role there: Member")).toBeTruthy();
  await signInWith("ada@example.com");

  expect(await screen.findByText("Signed in as Ada (ada@example.com).")).toBeTruthy();
  expect(router.state.location.pathname + router.state.location.hash).toBe(page);
  await user.click(screen.getByRole("button", { name: "Accept invitation" }));

  expect(await screen.findByRole("heading", { name: "Lab" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/lab");
  expect(router.state.historyAction).toBe("REPLACE");
  // The token goes in the bodies, never in an address.
  expect(sent.filter((request) => request.includes(link.token))).toEqual([]);
  expect(bodies.filter((body) => String(body).includes(link.token))).toHaveLength(3);
});

test("signed out, one signs up with the invitation, then accepts", async () => {
  const user = userEvent.setup();
  const { app, bodies } = invitationServer({ signedIn: false });
  renderApp(page, app);

  await user.click(await screen.findByRole("button", { name: "Sign up" }));
  expect(screen.getByText("Create an account with the e-mail address the invitation was sent to.")).toBeTruthy();
  await user.type(screen.getByLabelText("E-mail address"), "ada@example.com");
  await user.type(screen.getByLabelText("Password"), "correct horse battery");
  await user.click(screen.getByRole("button", { name: "Sign up" }));

  expect(await screen.findByRole("button", { name: "Accept invitation" })).toBeTruthy();
  expect(JSON.parse(String(bodies.find((body) => String(body).includes("password"))))).toEqual({
    email: "ada@example.com",
    password: "correct horse battery",
    invitation: link,
  });
});

test("a sign-up the server refuses says why above the form, and the page stays", async () => {
  const user = userEvent.setup();
  const { app } = invitationServer({ signedIn: false, register: () => problem(403, "identity.signup_disabled") });
  renderApp(page, app);

  await user.click(await screen.findByRole("button", { name: "Sign up" }));
  await user.type(screen.getByLabelText("E-mail address"), "eve@example.com");
  await user.type(screen.getByLabelText("Password"), "correct horse battery");
  await user.click(screen.getByRole("button", { name: "Sign up" }));

  expect((await screen.findByRole("alert")).textContent).toBe(
    "This server takes new accounts only for the addresses invited: sign up with the one the invitation was sent to."
  );
  expect(screen.getByText("You are invited to join Lab.")).toBeTruthy();
});

test("an account of another address is told so, and signs out to sign in with the invited one", async () => {
  const user = userEvent.setup();
  const { app, sent } = invitationServer({ accept: () => problem(403, "workspace.invitation_email_mismatch") });
  const { router } = renderApp(page, app);

  await user.click(await screen.findByRole("button", { name: "Accept invitation" }));
  expect((await screen.findByRole("alert")).textContent).toBe(
    "This invitation was sent to another e-mail address. Sign in with that one to accept it."
  );
  await user.click(screen.getByRole("button", { name: "Sign out" }));

  expect(await screen.findByRole("button", { name: "Sign in" })).toBeTruthy();
  expect(sent.filter((request) => request.includes("/auth/logout"))).toHaveLength(1);
  expect(screen.getByText("You are invited to join Lab.")).toBeTruthy();
  expect(router.state.location.pathname + router.state.location.hash).toBe(page);
});

// The acceptance the server made is answered after the page signed out: the request, of the session the tab
// has left, stops there, and the page stays signed out at the link (R2 of the M2 Codex review).
test("an acceptance answered after signing out does not go into the workspace", async () => {
  const user = userEvent.setup();
  let release: (() => void) | undefined;
  let answered = false;
  const { app } = invitationServer({
    accept: async () => {
      await new Promise<void>((resolve) => (release = resolve));
      answered = true;
      return json({ ...workspaceJSON, role: "member" });
    },
  });
  const { router } = renderApp(page, app);

  await user.click(await screen.findByRole("button", { name: "Accept invitation" }));
  await waitFor(() => expect(release).toBeDefined());
  await user.click(screen.getByRole("button", { name: "Sign out" }));
  expect(await screen.findByRole("button", { name: "Sign in" })).toBeTruthy();
  await act(async () => {
    release?.();
    await waitFor(() => expect(answered).toBe(true));
    await new Promise((resolve) => setTimeout(resolve, 100));
  });

  expect(router.state.location.pathname + router.state.location.hash).toBe(page);
  expect(screen.getByRole("button", { name: "Sign in" })).toBeTruthy();
});

test("a member already goes in, keeping the role they have", async () => {
  const user = userEvent.setup();
  const { app } = invitationServer({ accept: () => json({ ...workspaceJSON, role: "admin" }) });
  const { router } = renderApp(page, app);

  await user.click(await screen.findByRole("button", { name: "Accept invitation" }));
  expect(await screen.findByRole("heading", { name: "Lab" })).toBeTruthy();
  await user.click(screen.getByRole("link", { name: "Settings" }));

  expect(await screen.findByRole("button", { name: "Save" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/lab/settings/general");
});

test("an account with onboarding left goes through it before the workspace", async () => {
  const user = userEvent.setup();
  const { app } = invitationServer({ me: { ...userJSON, onboarding_steps: [] } });
  const { router } = renderApp(page, app);

  await user.click(await screen.findByRole("button", { name: "Accept invitation" }));

  expect(await screen.findByText("Step 1 of 2")).toBeTruthy();
  await waitFor(() => expect(router.state.location.pathname).toBe("/onboarding"));
  expect(router.state.location.search).toBe("?next=%2Flab");
});

test("a link gone by the time one accepts says it no longer works", async () => {
  const user = userEvent.setup();
  const { app } = invitationServer({ accept: () => problem(404, "workspace.invitation_not_found") });
  const { router } = renderApp(page, app);

  await user.click(await screen.findByRole("button", { name: "Accept invitation" }));

  expect((await screen.findByRole("alert")).textContent).toBe(
    "This invitation no longer works: it may have been accepted or withdrawn, or the link is incomplete."
  );
  expect(router.state.location.pathname + router.state.location.hash).toBe(page);
});

test("a sign-up here is checked as a sign-up: a password too short is not sent", async () => {
  const user = userEvent.setup();
  const { app, bodies } = invitationServer({ signedIn: false });
  renderApp(page, app);

  await user.click(await screen.findByRole("button", { name: "Sign up" }));
  await user.type(screen.getByLabelText("E-mail address"), "ada@example.com");
  await user.type(screen.getByLabelText("Password"), "short");
  await user.click(screen.getByRole("button", { name: "Sign up" }));

  expect(await screen.findByText("At least 8 characters.")).toBeTruthy();
  expect(bodies.filter((body) => String(body).includes("password"))).toEqual([]);
});

test("a session that cannot be used for now says so under the invitation; Try again goes on", async () => {
  const user = userEvent.setup();
  let busy = true;
  const { app } = invitationServer({ refresh: () => (busy ? problem(503, "server_busy") : json(tokensJSON)) });
  const { router } = renderApp(page, app);

  expect(await screen.findByRole("heading", { name: "Cannot reach the server" })).toBeTruthy();
  expect(screen.getByText("You are invited to join Lab.")).toBeTruthy();
  busy = false;
  await user.click(screen.getByRole("button", { name: "Try again" }));

  expect(await screen.findByRole("button", { name: "Accept invitation" })).toBeTruthy();
  expect(router.state.location.pathname + router.state.location.hash).toBe(page);
});

test.each([
  ["the invitation", "/preview"],
  ["the account", "/api/v0/me"],
])("%s cannot be read: the page says so; Try again reads it", async (_, path) => {
  const user = userEvent.setup();
  const { app, state } = invitationServer();
  state.down = path;
  renderApp(page, app);

  expect((await screen.findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );
  state.down = "";
  await user.click(screen.getByRole("button", { name: "Try again" }));

  expect(await screen.findByRole("button", { name: "Accept invitation" })).toBeTruthy();
});
