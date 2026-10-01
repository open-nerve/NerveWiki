import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { browserSessionDeps, Session, type SessionDeps } from "./session";
import { SharedStorage } from "./testing/fake-browser";
import { FakeServer, json, noContent } from "./testing/fake-server";
import { settle, track, until } from "./testing/fake-time";
import { AUTH_KEY, SessionChangedError, SessionStorageError } from "./token-manager";
import { LEASE_KEY, LOCK_NAME } from "./refresh-lock";

// The session's assembly (M1/P5 design 3.2): the clients, the lock, the storage events, the start.

const ME = "/api/v0/me";
const INSTANCE = "/api/v0/instance";
const loginId = "0123456789abcdef0123456789abcdef";
const loginIdY = "fedcba9876543210fedcba9876543210";

/** A lock manager that runs every task at once and records the locks asked for. */
function fakeLocks() {
  const names: string[] = [];
  const locks = {
    request: (name: string, task: () => Promise<unknown>) => {
      names.push(name);
      return task();
    },
  } as unknown as Pick<LockManager, "request">;
  return { locks, names };
}

function newSession(
  storage: SharedStorage,
  server: FakeServer,
  locks?: SessionDeps["locks"],
  tab: ReturnType<SharedStorage["tab"]> = storage.tab("A")
) {
  return new Session({
    storage: tab,
    onStorage: tab.onStorage,
    locks,
    now: () => Date.now(),
    randomHex: () => loginId,
    client: { baseUrl: "http://nervewiki.test", fetch: server.fetch },
  });
}

/** A session signed in to loginId through its first refresh. */
async function signedIn(storage: SharedStorage, server: FakeServer, locks = fakeLocks().locks) {
  storage.data.set(AUTH_KEY, JSON.stringify({ refresh_token: "rt-0", login_id: loginId }));
  const session = newSession(storage, server, locks);
  const started = track(session.start());
  await until(() => server.calls.length === 1, "the first refresh");
  server.calls[0]?.answer(json(200, server.tokens()));
  await until(() => started.settled, "the start");
  server.calls.length = 0;
  return session;
}

/** Tab A's view of storage in a browser whose storage for the site is full: it reads and removes, and writes nothing. */
function full(storage: SharedStorage) {
  return {
    ...storage.tab("A"),
    setItem: () => {
      throw new DOMException("The quota has been exceeded.", "QuotaExceededError");
    },
  };
}

beforeEach(() => {
  vi.useFakeTimers({ now: 1_000_000 });
});
afterEach(() => {
  vi.useRealTimers();
});

describe("Session", () => {
  it("starts signed out without a record, asking nothing", async () => {
    const server = new FakeServer();
    const session = newSession(new SharedStorage(), server, fakeLocks().locks);

    await settle(session.start(), "the start");

    expect(session.tokens.state).toEqual({ status: "signed-out" });
    expect(server.calls).toEqual([]);
  });

  it("sends the public operations without a token and the session's with its access token", async () => {
    const server = new FakeServer();
    const session = await signedIn(new SharedStorage(), server);

    const instance = track(session.public.GET(INSTANCE));
    const me = track(session.clientFor(loginId).GET(ME));
    await until(() => server.calls.length === 2, "the requests");

    expect(server.to(INSTANCE)[0]?.authorization).toBeNull();
    expect(server.to(ME)[0]?.authorization).toBe("Bearer at-1");
    for (const call of server.calls) call.answer(json(200, {}));
    await until(() => instance.settled && me.settled, "the answers");
  });

  it("follows another tab's sign-in, and a client of the session before sends nothing", async () => {
    const storage = new SharedStorage();
    const server = new FakeServer();
    const session = await signedIn(storage, server);
    const before = session.clientFor(loginId);

    storage.write(AUTH_KEY, JSON.stringify({ refresh_token: "rt-y", login_id: loginIdY }));
    await until(() => session.tokens.state.loginId === loginIdY, "the other tab's session");
    const me = await settle(before.GET(ME), "the request");

    expect(me.error).toBeInstanceOf(SessionChangedError);
    expect(server.calls).toEqual([]);
  });

  it("stops following the other tabs once disposed", async () => {
    const storage = new SharedStorage();
    const session = await signedIn(storage, new FakeServer());

    session.dispose();
    storage.write(AUTH_KEY, null);
    await vi.advanceTimersByTimeAsync(100);

    expect(session.tokens.state).toMatchObject({ status: "signed-in", loginId });
  });

  it("refreshes under navigator.locks where the page has it", async () => {
    const { locks, names } = fakeLocks();
    const storage = new SharedStorage();
    await signedIn(storage, new FakeServer(), locks);

    expect(names).toContain(LOCK_NAME);
    expect(storage.data.has(LEASE_KEY)).toBe(false);
  });

  // A browser that will not write the record cannot keep a session the tabs share: the tab ends the one it
  // had, logging it out with the newest refresh token it has, and signs out rather than hanging (R1 of the
  // M1 adversarial review).
  it("ends the session when the browser will not write the refreshed record", async () => {
    const storage = new SharedStorage();
    storage.data.set(AUTH_KEY, JSON.stringify({ refresh_token: "rt-0", login_id: loginId }));
    const server = new FakeServer();
    const session = newSession(storage, server, fakeLocks().locks, full(storage));

    const started = track(session.start());
    await until(() => server.calls.length === 1, "the first refresh");
    server.calls[0]?.answer(json(200, server.tokens()));
    await until(() => server.calls.length === 2, "the logout");
    server.calls[1]?.answer(noContent());
    await until(() => started.settled, "the start");

    expect(server.calls.map((c) => [c.path, c.body])).toEqual([
      ["/api/v0/auth/refresh", { refresh_token: "rt-0" }],
      ["/api/v0/auth/logout", { refresh_token: "rt-1" }],
    ]);
    expect(session.tokens.state).toEqual({ status: "signed-out" });
    expect(storage.data.has(AUTH_KEY)).toBe(false);
  });

  it("ends the session when the browser will not write the lease, without refreshing it", async () => {
    const storage = new SharedStorage();
    storage.data.set(AUTH_KEY, JSON.stringify({ refresh_token: "rt-0", login_id: loginId }));
    const server = new FakeServer();
    const session = newSession(storage, server, undefined, full(storage));

    const started = track(session.start());
    await until(() => server.calls.length === 1, "the logout");
    server.calls[0]?.answer(noContent());
    await until(() => started.settled, "the start");

    expect(server.calls.map((c) => [c.path, c.body])).toEqual([["/api/v0/auth/logout", { refresh_token: "rt-0" }]]);
    expect(session.tokens.state).toEqual({ status: "signed-out" });
    expect(storage.data.has(AUTH_KEY)).toBe(false);
  });

  it("undoes a sign-in the browser will not save, and says why", async () => {
    const storage = new SharedStorage();
    const server = new FakeServer();
    const session = newSession(storage, server, fakeLocks().locks, full(storage));
    await settle(session.start(), "the start");

    const signIn = track(session.tokens.signIn(server.tokens()));
    await until(() => server.calls.length === 1, "the logout");
    server.calls[0]?.answer(noContent());
    await until(() => signIn.settled, "the sign-in");

    expect(signIn.error).toBeInstanceOf(SessionStorageError);
    expect(server.calls.map((c) => [c.path, c.body])).toEqual([["/api/v0/auth/logout", { refresh_token: "rt-1" }]]);
    expect(session.tokens.state).toEqual({ status: "signed-out" });
  });

  it("refreshes under a lease in storage without navigator.locks", async () => {
    const storage = new SharedStorage();
    storage.data.set(AUTH_KEY, JSON.stringify({ refresh_token: "rt-0", login_id: loginId }));
    const server = new FakeServer();
    const session = newSession(storage, server, undefined);

    const started = track(session.start());
    await until(() => server.calls.length === 1, "the first refresh");

    expect(JSON.parse(storage.data.get(LEASE_KEY) ?? "null")).toMatchObject({ owner: loginId });
    server.calls[0]?.answer(json(200, server.tokens()));
    await until(() => started.settled, "the start");
    expect(storage.data.has(LEASE_KEY)).toBe(false);
  });
});

describe("browserSessionDeps", () => {
  it("hears the storage events of the record's storage only, and has a lease where there is no navigator.locks", () => {
    const deps = browserSessionDeps(localStorage);
    const keys: (string | null)[] = [];
    const unsubscribe = deps.onStorage((key) => keys.push(key));

    window.dispatchEvent(new StorageEvent("storage", { key: AUTH_KEY, storageArea: localStorage }));
    window.dispatchEvent(new StorageEvent("storage", { key: "other", storageArea: sessionStorage }));
    unsubscribe();
    window.dispatchEvent(new StorageEvent("storage", { key: AUTH_KEY, storageArea: localStorage }));

    expect(keys).toEqual([AUTH_KEY]);
    expect(deps.locks).toBe("locks" in navigator ? navigator.locks : undefined);
    expect(deps.randomHex(16)).toMatch(/^[0-9a-f]{32}$/);
  });
});
