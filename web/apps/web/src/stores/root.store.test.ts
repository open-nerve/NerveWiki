import { expect, test } from "vitest";

import { SessionChangedError } from "../session/token-manager";
import { json, storedSession, testApp, tokensJSON, workspaceJSON } from "../test/fakes";
import { RootStore } from "./root.store";

const tokens = (n: number) => ({
  token_type: "Bearer",
  access_token: `at-${n}`,
  access_token_expires_in: 900,
  refresh_token: `rt-${n}`,
  refresh_token_expires_at: "2026-10-31T00:00:00Z",
});
const me = {
  id: "0199a2b4-0000-7000-8000-000000000001",
  email: "ada@example.com",
  display_name: "ada",
  onboarding_steps: [],
};

// Each login gets its generation of stores (M1/P5 design 3.3): once the tab
// is in another session, the stores of the one before send nothing more,
// and the device's preferences and the instance's information carry over.
test("a generation of the session before sends nothing once the tab has signed in again", async () => {
  const sent: string[] = [];
  let issued = 0;
  const app = testApp((request) => {
    const { pathname } = new URL(request.url);
    sent.push(`${request.method} ${pathname}`);
    if (pathname === "/api/v0/me") return json(me);
    return json(tokens(++issued));
  }, storedSession("login-0"));
  await app.session.start();
  const before = new RootStore(app, "login-0");
  await before.account?.load();

  await before.auth.signIn("bob@example.com", "correct horse battery");
  const after = new RootStore(app, app.session.tokens.state.loginId);

  await expect(before.account?.load()).rejects.toBeInstanceOf(SessionChangedError);
  expect(sent).toEqual(["POST /api/v0/auth/refresh", "GET /api/v0/me", "POST /api/v0/auth/login"]);
  expect(after.loginId).toBe("login-1");
  expect(after.account?.me).toBeUndefined();
  expect(after.workspaces).not.toBe(before.workspaces);
  expect(after.workspaces?.list).toBeUndefined();
  expect(after.preferences).toBe(before.preferences);
  expect(after.instance).toBe(before.instance);
});

test("a signed-out generation has no account, nor its workspaces", () => {
  const store = new RootStore(testApp(), undefined);
  expect([store.account, store.workspaces, store.membersOf(workspaceJSON), store.invitationsOf(workspaceJSON)]).toEqual(
    [undefined, undefined, undefined, undefined]
  );
});

test("a workspace's member list is the same for the generation; another workspace's, or another generation's, is another", async () => {
  const app = testApp(() => json(tokensJSON), storedSession("login-0"));
  await app.session.start();
  const store = new RootStore(app, "login-0");
  const sameSlug = { ...workspaceJSON, id: "0199a2b4-0000-7000-8000-0000000000b2" };

  const members = store.membersOf(workspaceJSON);

  expect(members).toBeDefined();
  expect(store.membersOf({ ...workspaceJSON, name: "Lab renamed" })).toBe(members);
  expect(store.membersOf(sameSlug)).not.toBe(members);
  expect(new RootStore(app, "login-0").membersOf(workspaceJSON)).not.toBe(members);
});

test("a workspace's invitations are the same for the generation; another workspace's are another", async () => {
  const app = testApp(() => json(tokensJSON), storedSession("login-0"));
  await app.session.start();
  const store = new RootStore(app, "login-0");

  const invitations = store.invitationsOf(workspaceJSON);

  expect(invitations).toBeDefined();
  expect(store.invitationsOf({ ...workspaceJSON })).toBe(invitations);
  expect(store.invitationsOf({ ...workspaceJSON, id: "0199a2b4-0000-7000-8000-0000000000b2" })).not.toBe(invitations);
});
