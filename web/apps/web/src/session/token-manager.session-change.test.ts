import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { RecordingLock, SharedStorage, gate } from "./testing/fake-browser";
import { FakeServer, json, noContent, problem } from "./testing/fake-server";
import { track, until } from "./testing/fake-time";
import { LEASE_KEY, leaseLock, webLock } from "./refresh-lock";
import { AUTH_KEY, SessionChangedError, SessionUnavailableError, TokenManager } from "./token-manager";

// An operation that changes the session acts only on the session it was asked for (M1/P5 design 3.2): a
// refresh, a sign-out or the end of a session asked for as X, whose turn at the lock comes after another
// tab signed in as Y, never refreshes, logs out or removes Y's session, although the tab has followed Y by
// the time it holds the lock. What the tab keeps belongs to its session: following the record of the session
// it is in already changes nothing, neither its access token nor its state, and a session it follows starts
// without the back-off of the one it left. With both kinds of lock, as in token-manager.tabs.test.ts; where
// the order matters, the storage events are held back and reach the tabs as A's task gets the lock, so it
// does not rest on the fake's. The same holds when the change comes while the operation awaits, after it
// read the record: it reads the record again before it writes or decides anything (the last describe).

const REFRESH = "/api/v0/auth/refresh";
const LOGOUT = "/api/v0/auth/logout";
const X = "0000000000000000000000000000000a";
/** The login_id of the first sign-in in these tabs: their randomHex numbers the sign-ins. */
const Y = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb1";

type Kind = "navigator.locks" | "the lease";

/** One browser with the record of X, whose tabs record their session each time the lock is granted. */
function browser(kind: Kind) {
  const storage = new SharedStorage();
  storage.data.set(AUTH_KEY, JSON.stringify({ refresh_token: "rt-0", login_id: X }));
  const server = new FakeServer();
  // navigator.locks: one queue for the whole browser, as the browser keeps it.
  const shared = new RecordingLock();
  const locks = {
    request: ((_name: string, task: () => Promise<unknown>) => shared.run(task)) as LockManager["request"],
  };
  let logins = 0;
  /** A tab; atGrant runs each time one of its tasks gets the lock, before the task. */
  const tab = (id: string, atGrant?: () => void) => {
    const view = storage.tab(id);
    const lock =
      kind === "navigator.locks"
        ? webLock(locks)
        : leaseLock({ storage: view, onStorage: view.onStorage, now: () => Date.now(), tabId: id });
    // The tab's session each time the lock is granted to one of its tasks, and each change of its state.
    const grantedAs: (string | undefined)[] = [];
    const changes: string[] = [];
    const tm: TokenManager = new TokenManager({
      storage: view,
      lock: {
        run: (task) =>
          lock.run(() => {
            atGrant?.();
            grantedAs.push(tm.state.loginId);
            return task();
          }),
      },
      client: server.client(),
      now: () => Date.now(),
      randomHex: (bytes) => `${++logins}`.padStart(bytes * 2, "b"),
    });
    tm.subscribe(() => changes.push(`${tm.state.status} ${tm.state.loginId ?? "-"}`));
    view.onStorage((key) => {
      if (key === AUTH_KEY) tm.handleStorageChange();
    });
    return { tm, grantedAs, changes };
  };
  /**
   * Tab A, started as X with a 20 s token: it refreshes before every request. Each time one of its tasks
   * gets the lock, the events held back (storage.hold()) reach the tabs first: A has heard of B's sign-in
   * by then, one order a browser may give, forced here.
   */
  const tabA = async () => {
    const a = tab("A", () => storage.deliver());
    const started = track(a.tm.start());
    await until(() => server.calls.length === 1, "A's first refresh");
    server.calls[0]?.answer(json(200, server.tokens(20)));
    await until(() => started.settled, "A's start");
    a.grantedAs.length = 0;
    a.changes.length = 0;
    return a;
  };
  const stored = () => JSON.parse(storage.data.get(AUTH_KEY) ?? "null") as unknown;
  return { storage, server, tab, tabA, stored };
}

beforeEach(() => {
  vi.useFakeTimers({ now: 1_000_000 });
});
afterEach(() => {
  vi.useRealTimers();
});

describe.each<Kind>(["navigator.locks", "the lease"])("with %s", (kind) => {
  it("gives a request made as X a token of X's session or none, never Y's", async () => {
    const { storage, server, tab, tabA, stored } = browser(kind);
    const a = await tabA();

    // Tab B signs in as Y, holding the lock; meanwhile a request of A, made as X, needs a refresh.
    storage.hold();
    const b = tab("B");
    const signedIn = track(b.tm.signIn({ ...server.tokens(), refresh_token: "rt-y" }));
    const token = track(a.tm.accessToken());
    await until(() => signedIn.settled && a.grantedAs.length === 1, "B's sign-in, then A's refresh at the lock");
    expect(b.tm.state).toEqual({ status: "signed-in", loginId: Y });
    // A heard of B's sign-in as its refresh got the lock: the case this test is about.
    expect(a.grantedAs).toEqual([Y]);

    // Had A refreshed B's record for the request, the server would give it Y's tokens.
    await until(() => token.settled || server.calls.length === 2, "A's token or a refresh");
    server.calls[1]?.answer(json(200, server.tokens()));
    await until(() => token.settled, "A's token");
    expect(token.error).toBeInstanceOf(SessionChangedError);
    expect(server.to(REFRESH)).toHaveLength(1);
    // A follows B's session, whose record stays as B wrote it; the refresh found the session A follows
    // already, which changes nothing.
    expect(a.tm.state).toEqual({ status: "signed-in", loginId: Y });
    expect(a.changes).toEqual([`signed-in ${Y}`]);
    expect(stored()).toEqual({ refresh_token: "rt-y", login_id: Y });
  });

  it("logs nobody out when a sign-out asked for as X gets the lock after another tab's sign-in as Y", async () => {
    const { storage, server, tab, tabA, stored } = browser(kind);
    const a = await tabA();

    // Tab B's sign-in as Y takes the lock ahead of A's sign-out, asked for as X.
    storage.hold();
    const b = tab("B");
    const signedIn = track(b.tm.signIn({ ...server.tokens(), refresh_token: "rt-y" }));
    const out = track(a.tm.signOut());
    expect(a.tm.state.loginId).toBe(X);
    await until(() => signedIn.settled && out.settled, "B's sign-in and A's sign-out");
    // A heard of B's sign-in as its sign-out got the lock: the case this test is about.
    expect(a.grantedAs).toEqual([Y]);

    // B's sign-in replaced X's refresh token, so nothing of X is left to log out: A follows Y.
    expect(out.error).toBeUndefined();
    expect(server.to(LOGOUT)).toEqual([]);
    expect(stored()).toEqual({ refresh_token: "rt-y", login_id: Y });
    expect([a.tm.state, b.tm.state]).toEqual([
      { status: "signed-in", loginId: Y },
      { status: "signed-in", loginId: Y },
    ]);
    expect(a.changes).toEqual([`signed-in ${Y}`]);
  });

  it("keeps Y's session when a request of X, refused again, ends X's after another tab's sign-in as Y", async () => {
    const { storage, server, tab, tabA, stored } = browser(kind);
    const a = await tabA();

    // A request of A, made as X, was refused again after its replay, so A ends X's session; tab B's
    // sign-in as Y takes the lock first.
    storage.hold();
    const b = tab("B");
    const signedIn = track(b.tm.signIn({ ...server.tokens(), refresh_token: "rt-y" }));
    const ended = track(a.tm.endSession(X));
    expect(a.tm.state.loginId).toBe(X);
    await until(() => signedIn.settled && ended.settled, "B's sign-in and the end of X's session");
    // A heard of B's sign-in, and followed Y, as its end of X's session got the lock: the case this test
    // is about.
    expect(a.grantedAs).toEqual([Y]);

    expect(ended.error).toBeUndefined();
    expect(ended.value).toBe(false);
    expect(stored()).toEqual({ refresh_token: "rt-y", login_id: Y });
    expect([a.tm.state, b.tm.state]).toEqual([
      { status: "signed-in", loginId: Y },
      { status: "signed-in", loginId: Y },
    ]);
    expect(a.changes).toEqual([`signed-in ${Y}`]);
    expect(server.calls).toHaveLength(1);
  });

  it("keeps the access token of the session it follows already when ending X's session finds Y's record", async () => {
    const { server, tab, tabA } = browser(kind);
    const a = await tabA();
    // B signs in as Y; A follows, and a request of A's, made as Y, gets Y's token by a refresh.
    const b = tab("B");
    const signedIn = track(b.tm.signIn({ ...server.tokens(), refresh_token: "rt-y" }));
    await until(() => signedIn.settled && a.tm.state.loginId === Y, "B's sign-in, and A following Y");
    const token = track(a.tm.accessToken());
    await until(() => server.calls.length === 2, "A's refresh of Y");
    expect(server.calls[1]?.body).toEqual({ refresh_token: "rt-y" });
    server.calls[1]?.answer(json(200, server.tokens()));
    await until(() => token.settled, "Y's token");
    expect(token.value).toBe("at-3");

    // A request of A's made as X, refused again, ends X's session: the record is Y's, which A follows already.
    const ended = track(a.tm.endSession(X));
    await until(() => ended.settled, "the end of X's session");
    expect(ended.value).toBe(false);

    // A's next request, as Y, goes with the token A has: no other refresh.
    const next = track(a.tm.accessToken());
    await until(() => next.settled, "the next token");
    expect(next.value).toBe("at-3");
    expect(server.to(REFRESH)).toHaveLength(2);
    expect(a.tm.state).toEqual({ status: "signed-in", loginId: Y });
    expect(a.changes).toEqual([`signed-in ${Y}`]);
  });

  it("leaves X's back-off behind when it follows another tab's sign-in as Y", async () => {
    const { storage, server, tab } = browser(kind);
    const a = tab("A");
    void a.tm.start();
    await until(() => server.calls.length === 1, "A's first refresh");
    server.calls[0]?.answer(problem(503, "server_busy", { "Retry-After": "30" }));
    await until(() => a.tm.state.status === "unavailable", "unavailable");
    expect((a.tm.state.retryAt ?? 0) - Date.now()).toBeGreaterThan(29_000);

    // Another tab signs in as Y; A follows, and a request of A's, as Y, asks for Y's token at once.
    storage.write(AUTH_KEY, JSON.stringify({ refresh_token: "rt-y", login_id: Y }));
    await until(() => a.tm.state.loginId === Y, "A following Y");
    const token = track(a.tm.accessToken());
    await until(() => token.settled || server.calls.length === 2, "Y's token, or a refresh");
    expect(server.calls[1]?.body).toEqual({ refresh_token: "rt-y" });
    server.calls[1]?.answer(json(200, server.tokens()));
    await until(() => token.settled, "Y's token");
    expect(token.value).toBe("at-1");
    expect(a.tm.state).toEqual({ status: "signed-in", loginId: Y });
  });

  it("counts Y's back-off from Y's first failure, not from X's", async () => {
    const { storage, server, tab } = browser(kind);
    const a = tab("A");
    void a.tm.start();
    await until(() => server.calls.length === 1, "A's first refresh");
    server.calls[0]?.answer(problem(503, "server_busy", { "Retry-After": "30" }));
    await until(() => a.tm.state.status === "unavailable", "unavailable");

    storage.write(AUTH_KEY, JSON.stringify({ refresh_token: "rt-y", login_id: Y }));
    await until(() => a.tm.state.loginId === Y, "A following Y");
    const token = track(a.tm.accessToken());
    await until(() => server.calls.length === 2, "A's refresh of Y");
    const failedAt = Date.now();
    server.calls[1]?.fail();
    await until(() => token.settled, "the refresh's end");
    // The first failure of Y's session: 1 s, where X's count would make it 2 s.
    expect(token.error).toEqual(new SessionUnavailableError(failedAt + 1_000));
  });

  it("keeps an unavailable session and its retry when ending another session finds its record", async () => {
    const { server, tab } = browser(kind);
    const a = tab("A");
    void a.tm.start();
    await until(() => server.calls.length === 1, "A's first refresh");
    server.calls[0]?.answer(problem(503, "server_busy", { "Retry-After": "5" }));
    await until(() => a.tm.state.status === "unavailable", "unavailable");
    const unavailable = a.tm.state;

    // The record is X's, the session A is in: following it changes nothing, and the retry comes as planned.
    const ended = track(a.tm.endSession(Y));
    await until(() => ended.settled, "the end of Y's session");
    expect(ended.value).toBe(false);
    expect(a.tm.state).toBe(unavailable);
    await until(() => server.calls.length === 2, "the retry");
    expect(server.calls[1]?.body).toEqual({ refresh_token: "rt-0" });
    server.calls[1]?.answer(json(200, server.tokens()));
    await until(() => a.tm.state.status === "signed-in", "X's session");
    expect(a.tm.state).toEqual({ status: "signed-in", loginId: X });
  });
});

describe("after an await", () => {
  it("keeps the sign-in another tab made while a sign-out's logout was out and the lease had run out", async () => {
    const { storage, server, tab, tabA, stored } = browser("the lease");
    const a = await tabA();
    const out = track(a.tm.signOut());
    await until(() => server.to(LOGOUT).length === 1, "A's logout");
    expect(server.to(LOGOUT)[0]?.body).toEqual({ refresh_token: "rt-1" });

    // A is frozen past its lease while the logout is out: it hears nothing, and to the other tabs its lease has
    // run out. Fake time moves every tab at once, so the test runs A's lease out by hand; A's own 8 s timeout,
    // which the freeze holds back as well, does not come into it. Tab B then signs in as Y, lease and all.
    storage.hold();
    storage.write(LEASE_KEY, JSON.stringify({ owner: "A", expires: Date.now() - 1 }));
    const b = tab("B");
    const signedIn = track(b.tm.signIn({ ...server.tokens(), refresh_token: "rt-y" }));
    await until(() => signedIn.settled, "B's sign-in");
    expect(stored()).toEqual({ refresh_token: "rt-y", login_id: Y });

    // A thaws as the server answers its logout, before it hears of B's sign-in: Y's record stays, and A follows it.
    server.to(LOGOUT)[0]?.answer(noContent());
    await until(() => out.settled, "A's sign-out");
    storage.deliver();

    expect(out.error).toBeUndefined();
    expect(stored()).toEqual({ refresh_token: "rt-y", login_id: Y });
    expect([a.tm.state, b.tm.state]).toEqual([
      { status: "signed-in", loginId: Y },
      { status: "signed-in", loginId: Y },
    ]);
    expect(a.changes).toEqual([`signed-in ${Y}`]);
    expect(server.to(LOGOUT)).toHaveLength(1);
  });

  it("follows another tab's sign-in that came in before navigator.locks handed a failed first refresh back", async () => {
    const storage = new SharedStorage();
    storage.data.set(AUTH_KEY, JSON.stringify({ refresh_token: "rt-0", login_id: X }));
    const server = new FakeServer();
    const locks = new RecordingLock();
    const handBack = gate();
    const view = storage.tab("A");
    const a = new TokenManager({
      storage: view,
      // navigator.locks settles request() in a task of its own once the task is done, so another tab's storage
      // event may come in first: this lock holds the settling back until the test lets it go.
      lock: { run: (task) => locks.run(task).finally(() => handBack.promise) },
      client: server.client(),
      now: () => Date.now(),
      randomHex: (bytes) => "a".repeat(bytes * 2),
    });
    view.onStorage((key) => {
      if (key === AUTH_KEY) a.handleStorageChange();
    });
    const changes: string[] = [];
    a.subscribe(() => changes.push(`${a.state.status} ${a.state.loginId ?? "-"}`));

    // A's first refresh, as X, fails for a passing reason; another tab signs in as Y before the lock hands the
    // failure back, and A follows Y.
    const started = track(a.start());
    await until(() => server.calls.length === 1, "A's first refresh");
    server.calls[0]?.answer(problem(503, "server_busy"));
    await until(() => !locks.held, "the refresh's end under the lock");
    storage.write(AUTH_KEY, JSON.stringify({ refresh_token: "rt-y", login_id: Y }));
    await until(() => a.state.loginId === Y, "A following Y");
    handBack.open();
    await until(() => started.settled, "A's start");

    // X's failure leaves Y's session as it is: not unavailable, and no retry of it is due.
    expect(a.state).toEqual({ status: "signed-in", loginId: Y });
    expect(changes).toEqual([`starting ${X}`, `signed-in ${Y}`]);
    await vi.advanceTimersByTimeAsync(60_000);
    expect(server.calls).toHaveLength(1);
  });
});
