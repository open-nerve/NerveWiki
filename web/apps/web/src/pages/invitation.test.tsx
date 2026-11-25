import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { FakeChannels } from "../events/testing/fake-channels";
import { FakePage } from "../events/testing/fake-page";
import type { User } from "../services/account.service";
import type { Workspace } from "../services/workspace.service";
import { answerClosings } from "../stores/edit-closing";
import { withEvents } from "../test/event-server";
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
 * which joins the account's workspaces, joined, by default Acme alone
 * (/ would land on Acme); me is the account signed in. Each request goes
 * into sent, as its method and address, and each body into bodies. While
 * state.down names a path, it cannot be reached; refresh answers the
 * session's refresh. firstList, given, answers the first read of the
 * workspaces, the others answering joined.
 */
function invitationServer({
  signedIn = true,
  me = userJSON,
  accept,
  register,
  refresh = () => json(tokensJSON),
  joined: listed = [acme],
  firstList,
}: {
  signedIn?: boolean;
  me?: User;
  accept?: Answer;
  register?: Answer;
  refresh?: Answer;
  joined?: Workspace[];
  firstList?: Answer;
} = {}) {
  const sent: string[] = [];
  const bodies: unknown[] = [];
  const state = { down: "" };
  let joined: Workspace[] = listed;
  let lists = 0;
  const routes: Record<string, Answer> = {
    "POST /api/v0/auth/refresh": refresh,
    "GET /api/v0/instance": () => json(instanceJSON),
    "POST /api/v0/auth/login": () => json(tokensJSON),
    "POST /api/v0/auth/register": async (request) => (await register?.(request)) ?? json(tokensJSON, 201),
    "POST /api/v0/auth/logout": () => new Response(null, { status: 204 }),
    "GET /api/v0/me": () => json(me),
    "GET /api/v0/workspaces": async (request) =>
      (lists += 1) === 1 && firstList !== undefined ? firstList(request) : json({ data: joined }),
    [`POST /api/v0/workspace-invitations/${link.id}/preview`]: async (request) => {
      const { token } = (await request.clone().json()) as { token: string };
      return token === link.token
        ? json({ workspace: { name: "Lab", slug: "lab" }, role: "member" })
        : problem(404, "workspace.invitation_not_found");
    },
    [`POST /api/v0/workspace-invitations/${link.id}/accept`]: async (request) => {
      const answer = (await accept?.(request)) ?? json({ ...workspaceJSON, role: "member" });
      if (answer.ok) {
        const workspace = (await answer.clone().json()) as Workspace;
        joined = [...joined.filter((each) => each.id !== workspace.id), workspace];
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

test("signed out, one sees what the link invites to, signs in on the page, which stays, then accepts and goes in, the workspace's heading focused", async () => {
  const user = userEvent.setup();
  const { app, sent, bodies } = invitationServer({ signedIn: false });
  const { router } = renderApp(page, app);

  expect(await screen.findByText("You are invited to join Lab.")).toBeTruthy();
  expect(screen.getByText("Your role there: Member")).toBeTruthy();
  await signInWith("ada@example.com");

  expect(await screen.findByText("Signed in as Ada (ada@example.com).")).toBeTruthy();
  expect(router.state.location.pathname + router.state.location.hash).toBe(page);
  await user.click(screen.getByRole("button", { name: "Accept invitation" }));

  const heading = await screen.findByRole("heading", { name: "Lab" });
  expect(router.state.location.pathname).toBe("/lab");
  expect(router.state.historyAction).toBe("REPLACE");
  await waitFor(() => expect(document.activeElement).toBe(heading));
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

// An acceptance answered once the page was left takes the user nowhere:
// they went elsewhere, and the switcher lists the workspace (v0.1 design
// 13.2, item 16; M3 Codex review R3).
test("an acceptance answered after the page was left does not go into the workspace", async () => {
  const user = userEvent.setup();
  let release: (() => void) | undefined;
  const { app } = invitationServer({
    accept: async () => {
      await new Promise<void>((resolve) => (release = resolve));
      return json({ ...workspaceJSON, role: "member" });
    },
  });
  const { router } = renderApp(page, app);

  await user.click(await screen.findByRole("button", { name: "Accept invitation" }));
  await waitFor(() => expect(release).toBeDefined());
  await act(() => router.navigate("/settings/profile"));
  await screen.findByRole("heading", { level: 1, name: "Profile" });
  release?.();
  await act(() => new Promise((resolve) => setTimeout(resolve, 20)));

  expect(router.state.location.pathname).toBe("/settings/profile");
});

test("a member already goes in, keeping the role they have", async () => {
  const user = userEvent.setup();
  const { app } = invitationServer({ accept: () => json({ ...workspaceJSON, role: "admin" }) });
  const { router } = renderApp(page, app);

  await user.click(await screen.findByRole("button", { name: "Accept invitation" }));
  expect(await screen.findByRole("heading", { name: "Lab" })).toBeTruthy();
  await user.click(screen.getByRole("link", { name: "Settings" }));

  // An admin's: the name a field (NameRow, whose Save waits for an edit; M13/P3/S2).
  expect((await screen.findByLabelText<HTMLInputElement>("Name")).value).toBe("Lab");
  expect(router.state.location.pathname).toBe("/lab/settings/general");
});

test("an account with onboarding left goes through it before the workspace", async () => {
  const user = userEvent.setup();
  const { app } = invitationServer({ me: { ...userJSON, onboarding_steps: [] } });
  const { router } = renderApp(page, app);

  await user.click(await screen.findByRole("button", { name: "Accept invitation" }));

  expect(await screen.findByText("Step 1 of 3")).toBeTruthy();
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
  // The invitation keeps naming the tab, before and after.
  await waitFor(() => expect(document.title).toBe("Invitation · Nerve Wiki"));
  busy = false;
  await user.click(screen.getByRole("button", { name: "Try again" }));

  expect(await screen.findByRole("button", { name: "Accept invitation" })).toBeTruthy();
  expect(router.state.location.pathname + router.state.location.hash).toBe(page);
  expect(document.title).toBe("Invitation · Nerve Wiki");
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

// M12/P2/S1, item 7 (decision 383; reviews A's M2, C's M1): the invitation's Sign out is useSignOut's, as the user
// menu's: it asks first while another tab of the login has changes not saved, and it asks once however often it is
// pressed meanwhile.

test("the invitation's Sign out asks first while another tab of the login has an edit not saved; Stay keeps the session and gives the focus back to Sign out", async () => {
  const user = userEvent.setup();
  const channels = new FakeChannels();
  const { app: plain, sent } = invitationServer();
  const app = withEvents(plain, new FakePage(), channels);
  const other = answerClosings(
    channels.port("nwiki.edits"),
    { loginId: "login-0", tabId: "tab-1" },
    () => true,
    () => Promise.resolve(),
    () => true
  );
  renderApp(page, app);

  await user.click(await screen.findByRole("button", { name: "Sign out" }));
  const dialog = await screen.findByRole("alertdialog", {
    name: "Sign out anyway?",
    description:
      "A page you are editing in another tab has changes that are not saved. Signing out saves what it can first; what it cannot save is lost.",
  });
  await user.click(within(dialog).getByRole("button", { name: "Stay" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("button", { name: "Sign out" })));
  expect(sent.filter((request) => request.includes("/auth/logout"))).toEqual([]);
  expect(app.session.tokens.state.status).toBe("signed-in");
  other();
});

test("the invitation's Sign out pressed again while the other tabs are asked asks once and signs out once", async () => {
  const user = userEvent.setup();
  const channels = new FakeChannels();
  const { app: plain, sent } = invitationServer();
  const app = withEvents(plain, new FakePage(), channels);
  // Another tab of the login that answers nothing: the ask waits out its 100 milliseconds.
  const listening = channels.port("nwiki.edits");
  const asks: unknown[] = [];
  listening.addEventListener("message", (event) => {
    if ((event.data as { kind?: string }).kind === "ask") {
      asks.push(event.data);
    }
  });
  renderApp(page, app);

  const leave = await screen.findByRole("button", { name: "Sign out" });
  await user.click(leave);
  await user.click(leave);

  expect(await screen.findByRole("button", { name: "Sign in" })).toBeTruthy();
  expect(asks).toHaveLength(1);
  expect(sent.filter((request) => request.includes("/auth/logout"))).toHaveLength(1);
  listening.close();
});

// M13/P5/S1 (decision 922): the session unavailable in the invitation's card is its h2, the page's one h1 the
// invitation's.
test("a session that cannot be used for now says so in an alert under the invitation, its title an h2: the page's one h1 is the invitation's", async () => {
  const { app } = invitationServer({ refresh: () => problem(503, "server_busy") });
  renderApp(page, app);

  const title = await screen.findByRole("heading", { name: "Cannot reach the server" });
  expect(title.tagName).toBe("H2");
  expect(title.closest('[role="alert"]')).not.toBeNull();
  expect(screen.getAllByRole("heading", { level: 1 }).map((heading) => heading.textContent)).toEqual(["Invitation"]);
});

// M13/P5/S1 (decision 941; review A's P5A-Q2): signed in, the invitation's way back is the top right's Back to the
// workspace, as the page that creates a workspace has it: only where / lands on a workspace.

/** held is an answer kept until released; asked settles once it is asked for (as layout.test.tsx's). */
function held() {
  const waiting: ((response: Response) => void)[] = [];
  const askers: (() => void)[] = [];
  const asked = new Promise<void>((resolve) => {
    askers.push(resolve);
  });
  const answer: Answer = () =>
    new Promise<Response>((resolve) => {
      waiting.push(resolve);
      for (const ask of askers) ask();
    });
  return { answer, asked, release: (response: Response) => waiting.forEach((resolve) => resolve(response)) };
}

/**
 * listAnswered waits for the invitation's page signed in with its workspaces read: the account's button and Accept
 * shown, the read of the workspaces sent, and a moment for its answer, before what is not there is said not to be.
 */
async function listAnswered(sent: string[]) {
  expect(await screen.findByRole("button", { name: "Ada" })).toBeTruthy();
  expect(await screen.findByRole("button", { name: "Accept invitation" })).toBeTruthy();
  await waitFor(() =>
    expect(sent.some((each) => each.startsWith("GET ") && each.endsWith("/api/v0/workspaces"))).toBe(true)
  );
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
}

test("signed in, an account in a workspace and done with its onboarding has Back to the workspace at the top right: arrived at where / lands", async () => {
  const user = userEvent.setup();
  const { app } = invitationServer();
  const { router } = renderApp(page, app);

  await user.click(await screen.findByRole("link", { name: "Back to the workspace" }));

  const acmeHome = await screen.findByRole("heading", { level: 1, name: "Acme" });
  expect(router.state.location.pathname).toBe("/acme");
  await waitFor(() => expect(document.activeElement).toBe(acmeHome));
});

test("signed in, an account in no workspace has no Back to the workspace: / would land on the page that creates one", async () => {
  const { app, sent } = invitationServer({ joined: [] });
  renderApp(page, app);

  await listAnswered(sent);
  expect(screen.queryByRole("link", { name: "Back to the workspace" })).toBeNull();
});

test("signed in, an account with onboarding left has no Back to the workspace: / would go to the onboarding", async () => {
  const { app, sent } = invitationServer({ me: { ...userJSON, onboarding_steps: [] } });
  renderApp(page, app);

  await listAnswered(sent);
  expect(screen.queryByRole("link", { name: "Back to the workspace" })).toBeNull();
});

// Review A's P5A-R1-M1: the read the top right sends, out as the invitation is accepted, comes back without the
// workspace joined; the workspace's shell, keyed alike, would find it not.
test("an invitation accepted while the tab's first read of its workspaces is out goes into the workspace joined", async () => {
  const user = userEvent.setup();
  const first = held();
  const { app } = invitationServer({ firstList: first.answer });
  const { router } = renderApp(page, app);

  // Out before the acceptance: the invitation's top right reads the list.
  await first.asked;
  await user.click(await screen.findByRole("button", { name: "Accept invitation" }));
  await waitFor(() => expect(router.state.location.pathname).toBe("/lab"));
  // What it reads is the list as it was asked for: Acme alone.
  first.release(json({ data: [acme] }));

  expect(await screen.findByRole("heading", { level: 1, name: "Lab" })).toBeTruthy();
  expect(screen.queryByRole("heading", { name: "Page not found" })).toBeNull();
});
