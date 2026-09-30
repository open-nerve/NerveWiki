import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { RecordingLock, SharedStorage } from "./testing/fake-browser";
import { FakeServer, json, noContent, problem } from "./testing/fake-server";
import { settle, track, until } from "./testing/fake-time";
import { AUTH_KEY, SessionChangedError, SessionUnavailableError, TokenManager } from "./token-manager";

// One tab of the token manager (M1/P5 design 3.2, 3.3), with fake timers: the server answers when the test says
// so, and time moves only when the test moves it. The tabs' coordination is token-manager.tabs.test.ts.

const REFRESH = "/api/v0/auth/refresh";
const LOGOUT = "/api/v0/auth/logout";

function setUp(record?: { refresh_token: string; login_id: string }) {
  const storage = new SharedStorage();
  if (record) storage.data.set(AUTH_KEY, JSON.stringify(record));
  const server = new FakeServer();
  const lock = new RecordingLock();
  const view = storage.tab("A");
  // Every write of the record must happen while the lock is held (M1/P5 design 3.2).
  const writes: { key: string; held: boolean }[] = [];
  const tm = new TokenManager({
    storage: {
      getItem: view.getItem,
      setItem: (key, value) => {
        writes.push({ key, held: lock.held });
        view.setItem(key, value);
      },
      removeItem: (key) => {
        writes.push({ key, held: lock.held });
        view.removeItem(key);
      },
    },
    lock,
    client: server.client(),
    now: () => Date.now(),
    randomHex: (bytes) => `${bytes}`.padStart(bytes * 2, "a"),
  });
  const stored = () => JSON.parse(storage.data.get(AUTH_KEY) ?? "null") as unknown;
  return { storage, server, lock, tm, writes, stored };
}

const loginId = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa16";
const other = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb";
/** A write of the record while the lock was held, as every write must be. */
const underLock = { key: AUTH_KEY, held: true };

beforeEach(() => {
  vi.useFakeTimers({ now: 1_000_000 });
});
afterEach(() => {
  vi.useRealTimers();
});

describe("start", () => {
  it("is signed out without a record, and asks the server nothing", async () => {
    const { tm, server } = setUp();
    await tm.start();
    expect(tm.state).toEqual({ status: "signed-out" });
    expect(await settle(tm.accessToken(), "the token")).toEqual({ settled: true, value: undefined });
    expect(server.calls).toEqual([]);
  });

  it("is signed in when the first refresh succeeds", async () => {
    const { tm, server, stored, writes } = setUp({ refresh_token: "rt-0", login_id: loginId });
    const started = tm.start();
    expect(tm.state).toEqual({ status: "starting", loginId });
    await until(() => server.to(REFRESH).length === 1, "the refresh");
    expect(server.calls[0]?.body).toEqual({ refresh_token: "rt-0" });
    server.calls[0]?.answer(json(200, server.tokens()));
    await started;

    expect(tm.state).toEqual({ status: "signed-in", loginId });
    expect(stored()).toEqual({ refresh_token: "rt-1", login_id: loginId });
    expect(writes).toEqual([underLock]);
    expect((await settle(tm.accessToken(), "the token")).value).toBe("at-1");
    expect(server.calls).toHaveLength(1);
  });

  it("is signed out, the record gone, when the first refresh answers 401", async () => {
    const { tm, server, storage, writes } = setUp({ refresh_token: "rt-0", login_id: loginId });
    const started = tm.start();
    await until(() => server.calls.length === 1, "the refresh");
    server.calls[0]?.answer(problem(401, "identity.refresh_token_invalid"));
    await started;

    expect(tm.state).toEqual({ status: "signed-out" });
    expect(storage.data.has(AUTH_KEY)).toBe(false);
    expect(writes).toEqual([underLock]);
  });

  it.each([
    ["429", () => problem(429, "rate_limited", { "Retry-After": "5" }), 5_000],
    ["500", () => problem(500, "internal_error"), 1_000],
    ["503", () => problem(503, "server_busy"), 1_000],
  ])("keeps the record and is unavailable on a %s, then tries again by itself", async (_, answer, delay) => {
    const { tm, server, stored, writes } = setUp({ refresh_token: "rt-0", login_id: loginId });
    const started = tm.start();
    await until(() => server.calls.length === 1, "the refresh");
    server.calls[0]?.answer(answer());
    await started;

    const retryAt = Date.now() + delay;
    expect(tm.state).toEqual({ status: "unavailable", loginId, retryAt });
    expect(stored()).toEqual({ refresh_token: "rt-0", login_id: loginId });
    await vi.advanceTimersByTimeAsync(delay - 1);
    expect(server.calls).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(1);
    await until(() => server.calls.length === 2, "the second refresh");
    expect(server.calls[1]?.body).toEqual({ refresh_token: "rt-0" });
    server.calls[1]?.answer(json(200, server.tokens()));
    await until(() => tm.state.status === "signed-in", "the session");
    expect(writes).toEqual([underLock]);
  });

  it("keeps the record and is unavailable without a network, backing off 1, 2, 4 … up to 30 s", async () => {
    const { tm, server, stored, writes } = setUp({ refresh_token: "rt-0", login_id: loginId });
    void tm.start();
    // Fails refreshes n … 7 as they come, and returns how long the manager waited after each.
    const fail = async (n: number, waits: number[] = []): Promise<number[]> => {
      if (n > 7) return waits;
      await until(() => server.calls.length === n, `refresh ${n}`);
      const before = tm.state.retryAt;
      const failedAt = Date.now();
      server.calls[n - 1]?.answer(Response.error());
      await until(() => tm.state.status === "unavailable" && tm.state.retryAt !== before, "unavailable");
      const wait = (tm.state.retryAt ?? 0) - failedAt;
      await vi.advanceTimersByTimeAsync(wait);
      return fail(n + 1, [...waits, wait]);
    };
    expect(await fail(1)).toEqual([1_000, 2_000, 4_000, 8_000, 16_000, 30_000, 30_000]);
    expect(stored()).toEqual({ refresh_token: "rt-0", login_id: loginId });

    // A success starts the count again: the next failure waits 1 s.
    await until(() => server.calls.length === 8, "refresh 8");
    server.calls[7]?.answer(json(200, server.tokens()));
    await until(() => tm.state.status === "signed-in", "the session");
    const renewed = track(tm.renew("at-1"));
    await until(() => server.calls.length === 9, "refresh 9");
    const failedAt = Date.now();
    server.calls[8]?.answer(Response.error());
    await until(() => renewed.settled, "the refresh's end");
    expect(renewed.error).toEqual(new SessionUnavailableError(failedAt + 1_000));
    expect(writes).toEqual([underLock]);
  });

  it("tries again at once from the button", async () => {
    const { tm, server, writes } = setUp({ refresh_token: "rt-0", login_id: loginId });
    void tm.start();
    await until(() => server.calls.length === 1, "the refresh");
    server.calls[0]?.answer(problem(503, "server_busy", { "Retry-After": "20" }));
    await until(() => tm.state.status === "unavailable", "unavailable");

    void tm.retry();
    await until(() => server.calls.length === 2, "the retry");
    server.calls[1]?.answer(json(200, server.tokens()));
    await until(() => tm.state.status === "signed-in", "the session");
    expect(writes).toEqual([underLock]);
    // The timer of the automatic retry is gone with the unavailability.
    await vi.advanceTimersByTimeAsync(30_000);
    expect(server.calls).toHaveLength(2);
  });

  it("stops trying again when another tab changes the session meanwhile", async () => {
    const { tm, server, storage } = setUp({ refresh_token: "rt-0", login_id: loginId });
    storage.tab("A").onStorage((key) => {
      if (key === AUTH_KEY) tm.handleStorageChange();
    });
    void tm.start();
    await until(() => server.calls.length === 1, "the refresh");
    server.calls[0]?.answer(problem(503, "server_busy", { "Retry-After": "20" }));
    await until(() => tm.state.status === "unavailable", "unavailable");

    storage.write(AUTH_KEY, JSON.stringify({ refresh_token: "rt-y", login_id: other }));
    await until(() => tm.state.loginId === other, "the switch");
    expect(tm.state).toEqual({ status: "signed-in", loginId: other });
    await vi.advanceTimersByTimeAsync(30_000);
    expect(server.calls).toHaveLength(1);
  });
});

/** A tab signed in through its first refresh, the access token lasting expiresIn seconds. */
async function signedIn(expiresIn = 900) {
  const s = setUp({ refresh_token: "rt-0", login_id: loginId });
  const started = s.tm.start();
  await until(() => s.server.calls.length === 1, "the first refresh");
  s.server.calls[0]?.answer(json(200, s.server.tokens(expiresIn)));
  await started;
  return s;
}

describe("refresh", () => {
  it("uses the access token until 30 s before its end, counted from when it arrived", async () => {
    const s = setUp({ refresh_token: "rt-0", login_id: loginId });
    const started = s.tm.start();
    await until(() => s.server.calls.length === 1, "the first refresh");
    // The answer takes 5 s; the token's 900 s count from its arrival, not from the request.
    await vi.advanceTimersByTimeAsync(5_000);
    const arrived = Date.now();
    // The token is not a JWT: the manager never reads it, nor the server's clock (M1/P5 design 3.2).
    s.server.calls[0]?.answer(json(200, { ...s.server.tokens(900), access_token: "opaque" }));
    await started;

    await vi.advanceTimersByTimeAsync(arrived + 870_000 - 1 - Date.now());
    expect((await settle(s.tm.accessToken(), "the token")).value).toBe("opaque");
    expect(s.server.calls).toHaveLength(1);

    await vi.advanceTimersByTimeAsync(1);
    const next = track(s.tm.accessToken());
    await until(() => s.server.calls.length === 2, "the refresh");
    expect(s.server.calls[1]?.body).toEqual({ refresh_token: "rt-1" });
    s.server.calls[1]?.answer(json(200, s.server.tokens()));
    await until(() => next.settled, "the token");
    expect(next.value).toBe("at-2");
    expect(s.writes).toEqual([underLock, underLock]);
  });

  it("refreshes once for the requests of a tab that need a token at the same time", async () => {
    const s = await signedIn(20);
    const tokens = [track(s.tm.accessToken()), track(s.tm.accessToken()), track(s.tm.accessToken())];
    await until(() => s.server.calls.length === 2, "the refresh");
    await vi.advanceTimersByTimeAsync(100);
    expect(s.server.to(REFRESH)).toHaveLength(2);
    s.server.calls[1]?.answer(json(200, s.server.tokens()));
    await until(() => tokens.every((t) => t.settled), "the tokens");
    expect(tokens.map((t) => t.value)).toEqual(["at-2", "at-2", "at-2"]);
    expect(s.writes).toEqual([underLock, underLock]);
  });

  it("keeps login_id and writes the record in one piece", async () => {
    const s = await signedIn(20);
    const next = track(s.tm.accessToken());
    await until(() => s.server.calls.length === 2, "the refresh");
    s.server.calls[1]?.answer(json(200, s.server.tokens()));
    await until(() => next.settled, "the token");
    expect(s.stored()).toEqual({ refresh_token: "rt-2", login_id: loginId });
    expect(s.writes).toEqual([underLock, underLock]);
  });

  it.each([
    ["429", () => problem(429, "rate_limited")],
    ["500", () => problem(500, "internal_error")],
    ["503", () => problem(503, "server_busy")],
    ["a 400", () => problem(400, "bad_request")],
    ["no network", () => Response.error()],
  ])("keeps the session on %s, and gives the caller the error", async (_, answer) => {
    const s = await signedIn(20);
    const next = track(s.tm.accessToken());
    await until(() => s.server.calls.length === 2, "the refresh");
    s.server.calls[1]?.answer(answer());
    await until(() => next.settled, "the refresh's end");

    expect(next.error).toBeInstanceOf(SessionUnavailableError);
    expect(s.tm.state).toEqual({ status: "signed-in", loginId });
    expect(s.stored()).toEqual({ refresh_token: "rt-1", login_id: loginId });
  });

  it("keeps the session when the refresh's fetch is rejected, as without a network", async () => {
    const s = await signedIn(20);
    const next = track(s.tm.accessToken());
    await until(() => s.server.calls.length === 2, "the refresh");
    const failedAt = Date.now();
    s.server.calls[1]?.fail();
    await until(() => next.settled, "the refresh's end");

    expect(next.error).toEqual(new SessionUnavailableError(failedAt + 1_000));
    expect(s.tm.state).toEqual({ status: "signed-in", loginId });
    expect(s.stored()).toEqual({ refresh_token: "rt-1", login_id: loginId });
    expect(s.writes).toEqual([underLock]);
  });

  it("does not ask again before the back-off ends", async () => {
    const s = await signedIn(20);
    const first = track(s.tm.accessToken());
    await until(() => s.server.calls.length === 2, "the refresh");
    const answeredAt = Date.now();
    s.server.calls[1]?.answer(problem(503, "server_busy", { "Retry-After": "3" }));
    await until(() => first.settled, "the refresh's end");
    expect(first.error).toEqual(new SessionUnavailableError(answeredAt + 3_000));

    await vi.advanceTimersByTimeAsync(answeredAt + 3_000 - 1 - Date.now());
    const early = track(s.tm.accessToken());
    await vi.advanceTimersByTimeAsync(0);
    expect(early.error).toBeInstanceOf(SessionUnavailableError);
    expect(s.server.calls).toHaveLength(2);
    await vi.advanceTimersByTimeAsync(1);
    const again = track(s.tm.accessToken());
    await until(() => s.server.calls.length === 3, "the next refresh");
    s.server.calls[2]?.answer(json(200, s.server.tokens()));
    await until(() => again.settled, "the token");
    expect(again.value).toBe("at-2");
    expect(s.writes).toEqual([underLock, underLock]);
  });

  it("ends the session only when the refresh answers 401", async () => {
    const s = await signedIn(20);
    const next = track(s.tm.accessToken());
    await until(() => s.server.calls.length === 2, "the refresh");
    s.server.calls[1]?.answer(problem(401, "identity.refresh_token_invalid"));
    await until(() => next.settled, "the refresh's end");

    expect(next.value).toBeUndefined();
    expect(next.error).toBeUndefined();
    expect(s.tm.state).toEqual({ status: "signed-out" });
    expect(s.storage.data.has(AUTH_KEY)).toBe(false);
    expect(s.writes).toEqual([underLock, underLock]);
  });

  it("gives up a refresh after 8 s, keeping the session", async () => {
    const s = await signedIn(20);
    const next = track(s.tm.accessToken());
    await until(() => s.server.calls.length === 2, "the refresh");
    const call = s.server.calls[1];

    await vi.advanceTimersByTimeAsync((call?.at ?? 0) + 7_999 - Date.now());
    expect(next.settled).toBe(false);
    expect(call?.aborted()).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    expect(call?.aborted()).toBe(true);
    await until(() => next.settled, "the refresh's end");
    expect(next.error).toBeInstanceOf(SessionUnavailableError);
    expect(s.tm.state.status).toBe("signed-in");
    expect(s.stored()).toEqual({ refresh_token: "rt-1", login_id: loginId });
  });

  it("does not refresh a record of another sign-in it has not heard of yet", async () => {
    const s = await signedIn(20);
    // Another tab signed in; its storage event has not reached this tab when the lock is granted.
    s.storage.data.set(AUTH_KEY, JSON.stringify({ refresh_token: "rt-y", login_id: other }));
    const next = await settle(s.tm.accessToken(), "the token");

    expect(next.error).toBeInstanceOf(SessionChangedError);
    expect(s.server.calls).toHaveLength(1);
    expect(s.tm.state).toEqual({ status: "signed-in", loginId: other });
    expect(s.stored()).toEqual({ refresh_token: "rt-y", login_id: other });
  });

  it("gives a request made after another tab's sign-in a refresh of its own, not the old session's", async () => {
    const s = await signedIn(20);
    s.storage.tab("A").onStorage((key) => {
      if (key === AUTH_KEY) s.tm.handleStorageChange();
    });
    // The old session's refresh is out when another tab's sign-in lands (the lease's race window).
    const old = track(s.tm.accessToken());
    await until(() => s.server.calls.length === 2, "the old session's refresh");
    s.storage.write(AUTH_KEY, JSON.stringify({ refresh_token: "rt-y", login_id: other }));
    await until(() => s.tm.state.loginId === other, "the switch");
    const mine = track(s.tm.accessToken());
    s.server.calls[1]?.answer(json(200, s.server.tokens()));
    await until(() => old.settled, "the old session's request");
    expect(old.error).toBeInstanceOf(SessionChangedError);

    // The new session's request refreshes the new record once the old refresh has let the lock go.
    await until(() => s.server.calls.length === 3, "the new session's refresh");
    expect(s.server.calls[2]?.body).toEqual({ refresh_token: "rt-y" });
    // Another request meanwhile shares it: the old refresh's end does not start a second one.
    const also = track(s.tm.accessToken());
    s.server.calls[2]?.answer(json(200, s.server.tokens()));
    await until(() => mine.settled && also.settled, "the new session's tokens");
    expect([mine.value, also.value]).toEqual(["at-3", "at-3"]);
    expect(s.server.to(REFRESH)).toHaveLength(3);
    expect(s.stored()).toEqual({ refresh_token: "rt-3", login_id: other });
    expect(s.writes).toEqual([underLock, underLock]);
  });

  it("renews once after a 401, with the token another request got meanwhile if there is one", async () => {
    const s = await signedIn();
    const renewed = track(s.tm.renew("at-1"));
    await until(() => s.server.calls.length === 2, "the refresh");
    s.server.calls[1]?.answer(json(200, s.server.tokens()));
    await until(() => renewed.settled, "the token");
    expect(renewed.value).toBe("at-2");

    // A second request refused with the old token takes the new one, without another refresh.
    expect((await settle(s.tm.renew("at-1"), "the token")).value).toBe("at-2");
    expect(s.server.to(REFRESH)).toHaveLength(2);
    expect(s.writes).toEqual([underLock, underLock]);
  });

  it("gives a request refused after the session ended no token, asking the server nothing", async () => {
    const s = await signedIn();
    // Two requests with at-1 were refused; the first one's refresh answers 401 and ends the session.
    const first = track(s.tm.renew("at-1"));
    await until(() => s.server.calls.length === 2, "the refresh");
    s.server.calls[1]?.answer(problem(401, "identity.refresh_token_invalid"));
    await until(() => first.settled, "the refresh's end");
    expect(first.value).toBeUndefined();
    expect(s.tm.state).toEqual({ status: "signed-out" });

    expect(await settle(s.tm.renew("at-1"), "the token")).toEqual({ settled: true, value: undefined });
    expect(s.server.calls).toHaveLength(2);
    expect(s.writes).toEqual([underLock, underLock]);
  });
});

describe("sign-in and sign-out", () => {
  it("keeps the tokens of a sign-in with a new login_id, under the lock", async () => {
    const { tm, server, stored, writes } = setUp();
    await tm.start();
    await tm.signIn(server.tokens());
    const first = stored() as { login_id: string };
    expect(first).toEqual({ refresh_token: "rt-1", login_id: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa16" });
    expect(tm.state).toEqual({ status: "signed-in", loginId: first.login_id });
    expect((await settle(tm.accessToken(), "the token")).value).toBe("at-1");
    expect(server.calls).toEqual([]);
    expect(writes).toEqual([{ key: AUTH_KEY, held: true }]);
  });

  it("makes a new login_id at every sign-in", async () => {
    const storage = new SharedStorage();
    const server = new FakeServer();
    let n = 0;
    const tm = new TokenManager({
      storage: storage.tab("A"),
      lock: new RecordingLock(),
      client: server.client(),
      now: () => Date.now(),
      randomHex: (bytes) => `${++n}`.padStart(bytes * 2, "0"),
    });
    await tm.start();
    await tm.signIn(server.tokens());
    const first = tm.state.loginId;
    await tm.signIn(server.tokens());
    expect(first).toMatch(/^[0-9a-f]{32}$/);
    expect(tm.state.loginId).toMatch(/^[0-9a-f]{32}$/);
    expect(tm.state.loginId).not.toBe(first);
    expect(JSON.parse(storage.data.get(AUTH_KEY) ?? "null")).toEqual({
      refresh_token: "rt-2",
      login_id: tm.state.loginId,
    });
  });

  it("signs out with the latest refresh token, under the lock, and forgets the tokens", async () => {
    const { tm, server, storage, writes } = setUp();
    await tm.start();
    await tm.signIn(server.tokens());
    const out = track(tm.signOut());
    await until(() => server.calls.length === 1, "the logout");
    expect(server.calls[0]).toMatchObject({ path: LOGOUT, body: { refresh_token: "rt-1" } });
    server.calls[0]?.answer(noContent());
    await until(() => out.settled, "the sign-out");

    expect(tm.state).toEqual({ status: "signed-out" });
    expect(storage.data.has(AUTH_KEY)).toBe(false);
    expect(await settle(tm.accessToken(), "the token")).toEqual({ settled: true, value: undefined });
    expect(server.calls).toHaveLength(1);
    expect(writes).toEqual([
      { key: AUTH_KEY, held: true },
      { key: AUTH_KEY, held: true },
    ]);
  });

  it("signs out locally when the logout takes over 8 s", async () => {
    const { tm, server, storage } = setUp();
    await tm.start();
    await tm.signIn(server.tokens());
    const out = track(tm.signOut());
    await until(() => server.calls.length === 1, "the logout");
    await vi.advanceTimersByTimeAsync((server.calls[0]?.at ?? 0) + 7_999 - Date.now());
    expect(server.calls[0]?.aborted()).toBe(false);
    expect(tm.state.status).toBe("signed-in");
    await vi.advanceTimersByTimeAsync(1);
    await until(() => out.settled, "the sign-out");
    expect(server.calls[0]?.aborted()).toBe(true);
    expect(out.error).toBeUndefined();
    expect(tm.state).toEqual({ status: "signed-out" });
    expect(storage.data.has(AUTH_KEY)).toBe(false);
  });

  it("signs out locally when the logout's fetch is rejected, as without a network", async () => {
    const { tm, server, storage, writes } = setUp();
    await tm.start();
    await tm.signIn(server.tokens());
    const out = track(tm.signOut());
    await until(() => server.calls.length === 1, "the logout");
    server.calls[0]?.fail();
    await until(() => out.settled, "the sign-out");
    expect(out.error).toBeUndefined();
    expect(tm.state).toEqual({ status: "signed-out" });
    expect(storage.data.has(AUTH_KEY)).toBe(false);
    expect(writes).toEqual([underLock, underLock]);
  });

  it("ends the session after a refused replay, under the lock", async () => {
    const { tm, server, storage, writes } = setUp();
    await tm.start();
    await tm.signIn(server.tokens());
    expect(await tm.endSession(loginId)).toBe(true);
    expect(tm.state).toEqual({ status: "signed-out" });
    expect(storage.data.has(AUTH_KEY)).toBe(false);
    expect(writes).toEqual([
      { key: AUTH_KEY, held: true },
      { key: AUTH_KEY, held: true },
    ]);
    expect(server.calls).toEqual([]);
  });

  it("tells its subscribers of every change of state", async () => {
    const { tm, server } = setUp();
    const seen: string[] = [];
    const unsubscribe = tm.subscribe(() => seen.push(tm.state.status));
    await tm.start();
    await tm.signIn(server.tokens());
    unsubscribe();
    const out = track(tm.signOut());
    await until(() => server.calls.length === 1, "the logout");
    server.calls[0]?.answer(noContent());
    await until(() => out.settled, "the sign-out");
    expect(seen).toEqual(["signed-out", "signed-in"]);
  });
});
