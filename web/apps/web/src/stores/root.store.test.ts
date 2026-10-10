import { expect, test, vi } from "vitest";

import type { EventDeps } from "../events/deps";
import { EventHub } from "../events/hub";
import { FakeChannels } from "../events/testing/fake-channels";
import { FakePage } from "../events/testing/fake-page";
import { SharedStorage } from "../session/testing/fake-browser";
import { SessionChangedError } from "../session/token-manager";
import { withEvents } from "../test/event-server";
import { json, notebookJSON, storedSession, testApp, tokensJSON, workspaceJSON } from "../test/fakes";
import type { transferTo } from "../test/transfer";
import { AppStores, RootStore } from "./root.store";

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

/** leaving is whether the page, left now, would ask first: its beforeunload's default prevented. */
function leaving(): boolean {
  const event = new Event("beforeunload", { cancelable: true });
  window.dispatchEvent(event);
  return event.defaultPrevented;
}

// A generation's uploads stop as the tab leaves its login (M7/P4 design 3.3): the page no longer warns before it is
// left for them, and the next generation has none.
test("a generation's uploads stop once the tab has signed in again", async () => {
  let issued = 0;
  const app = testApp((request) => {
    const { pathname } = new URL(request.url);
    if (pathname === "/api/v0/me") return json(me);
    if (pathname.endsWith("/assets")) return new Promise<Response>(() => undefined);
    if (pathname.endsWith("/nodes")) return json({ data: [] });
    return json(tokens(++issued));
  }, storedSession("login-0"));
  await app.session.start();
  const before = new RootStore(app, "login-0");
  const assets = before.assetsOf(notebookJSON);

  const [upload] = assets?.upload(null, [new File(["x"], "a.png")], "Untitled", { maxBytes: undefined }) ?? [];
  await vi.waitFor(() => expect((app.transfer as ReturnType<typeof transferTo>).made).toHaveLength(1));
  expect(leaving()).toBe(true);
  await before.auth.signIn("bob@example.com", "correct horse battery");

  expect(upload?.signal.aborted).toBe(true);
  await vi.waitFor(() => expect(assets?.uploads).toEqual([]));
  expect(leaving()).toBe(false);
});

// An import's upload asks before the page is left under its own key (M7/P6 design 4.1): an attachment's upload of
// the notebook, which goes on across pages, ending does not end it.
test("an import's upload keeps the page asking once an attachment's upload of the notebook has ended", async () => {
  const app = testApp((request) => {
    const { pathname } = new URL(request.url);
    if (pathname === "/api/v0/me") return json(me);
    if (pathname.endsWith("/assets") || pathname.endsWith("/imports")) return new Promise<Response>(() => undefined);
    if (pathname.endsWith("/nodes")) return json({ data: [] });
    return json(tokens(1));
  }, storedSession("login-0"));
  await app.session.start();
  const store = new RootStore(app, "login-0");
  const stop = new AbortController();
  const imported = store
    .transfersOf(notebookJSON)
    ?.startImport(null, new File(["PK"], "Vault.zip"), { signal: stop.signal })
    .catch(() => undefined);
  const assets = store.assetsOf(notebookJSON);
  const [attachment] = assets?.upload(null, [new File(["x"], "a.png")], "Untitled", { maxBytes: undefined }) ?? [];
  await vi.waitFor(() => expect((app.transfer as ReturnType<typeof transferTo>).made).toHaveLength(2));

  attachment?.cancel();
  await vi.waitFor(() => expect(assets?.uploads).toEqual([]));
  expect(leaving()).toBe(true);
  stop.abort();
  await imported;
  expect(leaving()).toBe(false);
});

test("a generation whose login has gone before its attachments are first asked for cancels their uploads at once", async () => {
  let issued = 0;
  const app = testApp((request) => {
    const { pathname } = new URL(request.url);
    if (pathname === "/api/v0/me") return json(me);
    if (pathname.endsWith("/nodes")) return json({ data: [] });
    return json(tokens(++issued));
  }, storedSession("login-0"));
  await app.session.start();
  const before = new RootStore(app, "login-0");
  await before.auth.signIn("bob@example.com", "correct horse battery");

  const [upload] =
    before.assetsOf(notebookJSON)?.upload(null, [new File(["x"], "a.png")], "Untitled", { maxBytes: undefined }) ?? [];

  expect(upload?.signal.aborted).toBe(true);
  await new Promise((resolve) => setTimeout(resolve, 0));
  expect((app.transfer as ReturnType<typeof transferTo>).made).toEqual([]);
});

test("a generation made and dropped, its attachments never asked for, does not watch the session", async () => {
  const app = testApp(() => json(tokens(1)), storedSession("login-0"));
  await app.session.start();
  const watching = vi.spyOn(app.session.tokens, "subscribe");

  const dropped = new RootStore(app, "login-0");
  expect(watching).not.toHaveBeenCalled();
  dropped.assetsOf(notebookJSON);
  dropped.assetsOf({ ...notebookJSON, id: "other" });

  expect(watching).toHaveBeenCalledTimes(1);
});

test("a signed-out generation has no account, nor its workspaces", () => {
  const store = new RootStore(testApp(), undefined);
  expect([
    store.account,
    store.workspaces,
    store.membersOf(workspaceJSON),
    store.invitationsOf(workspaceJSON),
    store.notebooksOf(workspaceJSON),
    store.notebookMembersOf(notebookJSON),
    store.ownerlessOf(workspaceJSON),
    store.auditOf(workspaceJSON),
    store.assetsOf(notebookJSON),
    store.transfersOf(notebookJSON),
  ]).toEqual([
    undefined,
    undefined,
    undefined,
    undefined,
    undefined,
    undefined,
    undefined,
    undefined,
    undefined,
    undefined,
  ]);
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

// The notebooks go by the workspace's id, their members by the notebook's
// (v0.1 design 13.2, item 15).
test("a workspace's notebooks, and a notebook's members, are the same for the generation; another's, or another generation's, are other", async () => {
  const app = testApp(() => json(tokensJSON), storedSession("login-0"));
  await app.session.start();
  const store = new RootStore(app, "login-0");
  const other = "0199a2b4-0000-7000-8000-0000000000b2";

  const notebooks = store.notebooksOf(workspaceJSON);
  const members = store.notebookMembersOf(notebookJSON);

  expect([notebooks, members]).not.toContain(undefined);
  expect(store.notebooksOf({ ...workspaceJSON, name: "Lab renamed" })).toBe(notebooks);
  expect(store.notebooksOf({ ...workspaceJSON, id: other })).not.toBe(notebooks);
  expect(store.notebookMembersOf({ ...notebookJSON, name: "Renamed", member_count: 3 })).toBe(members);
  expect(store.notebookMembersOf({ ...notebookJSON, id: other })).not.toBe(members);
  const next = new RootStore(app, "login-0");
  expect([next.notebooksOf(workspaceJSON), next.notebookMembersOf(notebookJSON)]).not.toContain(notebooks);
  expect(next.notebookMembersOf(notebookJSON)).not.toBe(members);
});

// A notebook's page tree goes by the notebook's id (M4/P5 design 3.4).
test("a notebook's page tree is the same for the generation; another's, or another generation's, is other", async () => {
  const app = testApp(() => json(tokensJSON), storedSession("login-0"));
  await app.session.start();
  const store = new RootStore(app, "login-0");

  const pages = store.pagesOf(notebookJSON);

  expect(pages).toBeDefined();
  expect(store.pagesOf({ ...notebookJSON, name: "Renamed" })).toBe(pages);
  expect(store.pagesOf({ ...notebookJSON, id: "0199a2b4-0000-7000-8000-0000000000b2" })).not.toBe(pages);
  expect(new RootStore(app, "login-0").pagesOf(notebookJSON)).not.toBe(pages);
  expect(new RootStore(app, undefined).pagesOf(notebookJSON)).toBeUndefined();
});

// A notebook's imports and exports go by the notebook's id (M7/P5 design 4.2).
test("a notebook's jobs are the same for the generation; another's, or another generation's, are other", async () => {
  const app = testApp(() => json(tokensJSON), storedSession("login-0"));
  await app.session.start();
  const store = new RootStore(app, "login-0");

  const jobs = store.transfersOf(notebookJSON);

  expect(jobs).toBeDefined();
  expect(store.transfersOf({ ...notebookJSON, name: "Renamed" })).toBe(jobs);
  expect(store.transfersOf({ ...notebookJSON, id: "0199a2b4-0000-7000-8000-0000000000b2" })).not.toBe(jobs);
  expect(new RootStore(app, "login-0").transfersOf(notebookJSON)).not.toBe(jobs);
});

// A notebook's attachments go by the notebook's id (M7/P4 design 3.3).
test("a notebook's attachments are the same for the generation; another's, or another generation's, are other", async () => {
  const app = testApp(() => json(tokensJSON), storedSession("login-0"));
  await app.session.start();
  const store = new RootStore(app, "login-0");

  const assets = store.assetsOf(notebookJSON);

  expect(assets).toBeDefined();
  expect(store.assetsOf({ ...notebookJSON, name: "Renamed" })).toBe(assets);
  expect(store.assetsOf({ ...notebookJSON, id: "0199a2b4-0000-7000-8000-0000000000b2" })).not.toBe(assets);
  expect(new RootStore(app, "login-0").assetsOf(notebookJSON)).not.toBe(assets);
});

// The ownerless notebooks and the audit events go by the workspace's id (M3/P5 design 3.2).
test("a workspace's ownerless notebooks and audit events are the same for the generation; another's are other", async () => {
  const app = testApp(() => json(tokensJSON), storedSession("login-0"));
  await app.session.start();
  const store = new RootStore(app, "login-0");
  const other = { ...workspaceJSON, id: "0199a2b4-0000-7000-8000-0000000000b2" };

  const ownerless = store.ownerlessOf(workspaceJSON);
  const audit = store.auditOf(workspaceJSON);

  expect([ownerless, audit]).not.toContain(undefined);
  expect(store.ownerlessOf({ ...workspaceJSON, name: "Lab renamed" })).toBe(ownerless);
  expect(store.auditOf({ ...workspaceJSON, name: "Lab renamed" })).toBe(audit);
  expect([store.ownerlessOf(other), store.auditOf(other)]).not.toContain(ownerless);
  expect(store.auditOf(other)).not.toBe(audit);
  const next = new RootStore(app, "login-0");
  expect(next.ownerlessOf(workspaceJSON)).not.toBe(ownerless);
  expect(next.auditOf(workspaceJSON)).not.toBe(audit);
});

// The event stream (M5/P3 design 3.7): one hub for the generation, which
// the app starts; none signed out, nor where the page has no EventDeps.
test("a signed-in generation has one event hub, not started, where the page has the event stream's deps", async () => {
  const base = testApp(() => json(tokensJSON), storedSession("login-0"));
  await base.session.start();
  const storage = new SharedStorage().tab("tab-0");
  const channel = vi.fn<EventDeps["channel"]>();
  const deps: EventDeps = {
    locks: undefined,
    storage,
    onStorage: storage.onStorage,
    channel,
    page: { visible: () => true, on: () => () => undefined },
    now: () => Date.now(),
    tabId: "tab-0",
  };
  const app = new AppStores(base.preferences, base.session, deps);
  const store = new RootStore(app, "login-0");

  const hub = store.events();

  expect(hub).toBeInstanceOf(EventHub);
  expect(store.events()).toBe(hub);
  expect(channel).not.toHaveBeenCalled();
  expect(new RootStore(app, "login-0").events()).not.toBe(hub);
  expect(new RootStore(app, undefined).events()).toBeUndefined();
  expect(new RootStore(base, "login-0").events()).toBeUndefined();
});

// An edit holds its page's lock (M5/P4 design 3.4, 3.5): its session ends as
// the page is left, synchronously, with the login's token as it is.
test("an edit's session ends as the page is left, with the login's token while it is valid; once it expired, nothing goes", async () => {
  vi.useFakeTimers();
  try {
    const sent: string[] = [];
    let refreshes = 0;
    const base = testApp((request) => {
      const { pathname } = new URL(request.url);
      sent.push(`${request.method} ${pathname} ${request.headers.get("Authorization") ?? ""}`);
      if (pathname === "/api/v0/auth/refresh") {
        // The first refresh signs the tab in; the beats' later ones fail, the token in memory left to expire.
        return ++refreshes === 1 ? json(tokensJSON) : json({ status: 503, code: "server_busy", title: "" }, 503);
      }
      return json({ id: `s${sent.length}`, page_id: "p1", expires_at: "2026-10-03T08:02:00Z" }, 201);
    }, storedSession("login-0"));
    await base.session.start();
    const page = new FakePage();
    const store = new RootStore(withEvents(base, page), "login-0");
    const editing = store.editPage({ id: "n1", workspace_id: "w1" }, "p1");
    expect(await editing?.begin(false)).toEqual({ opened: true });
    expect([...store.edits]).toEqual([editing]);

    sent.length = 0;
    page.fire("pagehide");
    expect(sent).toEqual(["DELETE /api/v0/edit-sessions/s2 Bearer at-1"]);

    sent.length = 0;
    await vi.advanceTimersByTimeAsync(tokensJSON.access_token_expires_in * 1000);
    sent.length = 0;
    page.fire("pagehide");
    expect(sent).toEqual([]);
    expect(new RootStore(base, undefined).editPage({ id: "n1", workspace_id: "w1" }, "p1")).toBeUndefined();
  } finally {
    vi.useRealTimers();
  }
});

/** A generation of login-0 whose edit of p1 is open; ends answers its end, unless it never comes. */
async function editingP1(ends: boolean) {
  const sent: string[] = [];
  const base = testApp((request) => {
    const { pathname } = new URL(request.url);
    sent.push(`${request.method} ${pathname}`);
    if (pathname === "/api/v0/auth/refresh") {
      return json(tokensJSON);
    }
    if (request.method === "DELETE") {
      return ends ? new Response(null, { status: 204 }) : new Promise<Response>(() => undefined);
    }
    if (pathname === "/api/v0/auth/logout") {
      return new Response(null, { status: 204 });
    }
    return json({ id: "s1", page_id: "p1", expires_at: "2026-10-03T08:02:00Z" }, 201);
  }, storedSession("login-0"));
  await base.session.start();
  const store = new RootStore(withEvents(base, new FakePage()), "login-0");
  await store.editPage({ id: "n1", workspace_id: "w1" }, "p1")?.begin(false);
  // The content, read once the lock is the edit's.
  await vi.advanceTimersByTimeAsync(0);
  sent.length = 0;
  return { store, sent };
}

test("unsavedEdit tells an edit with changes not saved, by its page, its notebook and its workspace", async () => {
  vi.useFakeTimers();
  try {
    const { store } = await editingP1(true);
    const [editing] = store.edits;
    const asked = [
      {},
      { pageId: "p1" },
      { notebookId: "n1" },
      { workspaceId: "w1" },
      { pageId: "p2" },
      { notebookId: "n2" },
    ];
    expect(asked.map((edit) => store.unsavedEdit(edit))).toEqual([false, false, false, false, false, false]);
    editing?.changed(1);
    expect(asked.map((edit) => store.unsavedEdit(edit))).toEqual([true, true, true, true, false, false]);
    expect(store.unsavedEdit({ workspaceId: "w2" })).toBe(false);
  } finally {
    vi.useRealTimers();
  }
});

test("signing out ends the generation's edits first, waiting for their ends 2 seconds at most", async () => {
  vi.useFakeTimers();
  try {
    const answered = await editingP1(true);
    // The other tabs have their time to say they have edits (stores/edit-closing.ts): none does.
    const signedOut = answered.store.auth.signOut();
    await vi.advanceTimersByTimeAsync(100);
    await signedOut;
    expect(answered.sent).toEqual(["DELETE /api/v0/edit-sessions/s1", "POST /api/v0/auth/logout"]);
    expect(answered.store.edits.size).toBe(0);

    const unanswered = await editingP1(false);
    const signingOut = unanswered.store.auth.signOut();
    await vi.advanceTimersByTimeAsync(1_999);
    expect(unanswered.sent).toEqual(["DELETE /api/v0/edit-sessions/s1"]);
    await vi.advanceTimersByTimeAsync(1);
    await signingOut;
    expect(unanswered.sent).toEqual(["DELETE /api/v0/edit-sessions/s1", "POST /api/v0/auth/logout"]);
  } finally {
    vi.useRealTimers();
  }
});

test("signing out while an edit's session opens ends it as it opens, before the logout", async () => {
  vi.useFakeTimers();
  try {
    const sent: string[] = [];
    let open: (() => void) | undefined;
    const base = testApp((request) => {
      const { pathname } = new URL(request.url);
      if (pathname === "/api/v0/auth/refresh") {
        return json(tokensJSON);
      }
      sent.push(`${request.method} ${pathname}`);
      if (pathname === "/api/v0/auth/logout" || request.method === "DELETE") {
        return new Response(null, { status: 204 });
      }
      return new Promise<Response>((resolve) => {
        open = () => resolve(json({ id: "s1", page_id: "p1", expires_at: "2026-10-03T08:02:00Z" }, 201));
      });
    }, storedSession("login-0"));
    await base.session.start();
    const store = new RootStore(withEvents(base, new FakePage()), "login-0");
    const beginning = store.editPage({ id: "n1", workspace_id: "w1" }, "p1")?.begin(false);
    await vi.advanceTimersByTimeAsync(0);

    const signingOut = store.auth.signOut();
    await vi.advanceTimersByTimeAsync(0);
    expect(sent).toEqual(["POST /api/v0/pages/p1/edit-sessions"]);
    open?.();
    const refused = expect(beginning).rejects.toThrow("The edit has ended.");
    await vi.advanceTimersByTimeAsync(100);
    await signingOut;
    await refused;
    expect(sent).toEqual([
      "POST /api/v0/pages/p1/edit-sessions",
      "DELETE /api/v0/edit-sessions/s1",
      "POST /api/v0/auth/logout",
    ]);
  } finally {
    vi.useRealTimers();
  }
});

test("signing out in one tab saves and ends another tab's edit of the login before the logout (M4–M5 Codex review R2)", async () => {
  vi.useFakeTimers();
  try {
    const sent: string[] = [];
    const base = testApp((request) => {
      const { pathname } = new URL(request.url);
      if (pathname === "/api/v0/auth/refresh") {
        return json(tokensJSON);
      }
      sent.push(`${request.method} ${pathname}`);
      if (pathname === "/api/v0/auth/logout" || request.method === "DELETE") {
        return new Response(null, { status: 204 });
      }
      if (request.method === "GET") {
        return json({ content: "Saved.\n", revision: 1 });
      }
      if (request.method === "PUT") {
        // A's save outlasts the window for the answers: the sign-out waits for it to be done, not just begun.
        return new Promise<Response>((resolve) => setTimeout(() => resolve(json({ revision: 2 })), 500));
      }
      return json({ id: "s1", page_id: "p1", expires_at: "2026-10-03T08:02:00Z" }, 201);
    }, storedSession("login-0"));
    await base.session.start();
    const channels = new FakeChannels();
    const tabOf = (tabId: string) => {
      const storage = new SharedStorage().tab(tabId);
      const deps: EventDeps = {
        locks: undefined,
        storage,
        onStorage: storage.onStorage,
        channel: (name) => channels.port(name),
        page: new FakePage(),
        now: () => Date.now(),
        tabId,
      };
      return new RootStore(new AppStores(base.preferences, base.session, deps), "login-0");
    };
    const [a, b] = [tabOf("tab-a"), tabOf("tab-b")];
    const answering = a.answerSignOuts();
    const editing = a.editPage({ id: "n1", workspace_id: "w1" }, "p1");
    await editing?.begin(false);
    await vi.advanceTimersByTimeAsync(0);
    editing?.changed(1);
    editing?.savesThrough(() => editing.save("Saved.\nTyped.\n", 1));
    sent.length = 0;

    const signingOut = b.auth.signOut();
    await vi.advanceTimersByTimeAsync(500);
    await signingOut;
    expect(sent).toEqual([
      "PUT /api/v0/pages/p1/content",
      "DELETE /api/v0/edit-sessions/s1",
      "POST /api/v0/auth/logout",
    ]);
    expect(a.edits.size).toBe(0);
    answering();
  } finally {
    vi.useRealTimers();
  }
});
