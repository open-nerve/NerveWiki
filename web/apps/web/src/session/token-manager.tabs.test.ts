import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { RecordingLock, SharedStorage } from "./testing/fake-browser";
import { FakeServer, json, noContent, problem } from "./testing/fake-server";
import { track, until } from "./testing/fake-time";
import { leaseLock, webLock } from "./refresh-lock";
import { AUTH_KEY, SessionChangedError, SessionUnavailableError, TokenManager } from "./token-manager";

// Several tabs of one browser (M1/P5 design 3.2, 3.3): one localStorage, one server, and the tabs' lock,
// either navigator.locks or, where the page has none (plain HTTP on a LAN address), the lease. Each tab
// hears the others' writes of nwiki.auth as the browser's storage event. Time is fake throughout.

const REFRESH = "/api/v0/auth/refresh";
const LOGOUT = "/api/v0/auth/logout";
const X = "0000000000000000000000000000000a";
const Y = "0000000000000000000000000000000b";

type Kind = "navigator.locks" | "the lease";

function browser(kind: Kind) {
  const storage = new SharedStorage();
  const server = new FakeServer();
  // navigator.locks: one queue for the whole browser, as the browser keeps it.
  const shared = new RecordingLock();
  const locks = {
    request: ((_name: string, task: () => Promise<unknown>) => shared.run(task)) as LockManager["request"],
  };
  let logins = 0;
  function tab(id: string) {
    const view = storage.tab(id);
    const lock =
      kind === "navigator.locks"
        ? webLock(locks)
        : leaseLock({ storage: view, onStorage: view.onStorage, now: () => Date.now(), tabId: id });
    const tm = new TokenManager({
      storage: view,
      lock,
      client: server.client(),
      now: () => Date.now(),
      randomHex: (bytes) => `${++logins}`.padStart(bytes * 2, "c"),
    });
    const changes: string[] = [];
    tm.subscribe(() => changes.push(`${tm.state.status} ${tm.state.loginId ?? "-"}`));
    view.onStorage((key) => {
      if (key === AUTH_KEY) tm.handleStorageChange();
    });
    return { tm, changes };
  }
  const stored = () => JSON.parse(storage.data.get(AUTH_KEY) ?? "null") as unknown;
  return { storage, server, tab, stored };
}

/** Tabs that share the record of X, each with an access token that lasts expiresIn seconds. */
async function signedIn(b: ReturnType<typeof browser>, ids: string[], expiresIn = 20) {
  b.storage.data.set(AUTH_KEY, JSON.stringify({ refresh_token: "rt-0", login_id: X }));
  const tabs = ids.map((id) => b.tab(id));
  const started = tabs.map((t) => track(t.tm.start()));
  // The tabs' first refreshes, one after the other.
  const answer = async (n: number): Promise<void> => {
    if (n > tabs.length) return;
    await until(() => b.server.to(REFRESH).length === n, `refresh ${n}`);
    b.server.to(REFRESH)[n - 1]?.answer(json(200, b.server.tokens(expiresIn)));
    return answer(n + 1);
  };
  await answer(1);
  await until(() => started.every((s) => s.settled), "the starts");
  b.server.calls.length = 0;
  for (const t of tabs) t.changes.length = 0;
  return tabs;
}

beforeEach(() => {
  vi.useFakeTimers({ now: 1_000_000 });
});
afterEach(() => {
  vi.useRealTimers();
});

describe.each<Kind>(["navigator.locks", "the lease"])("tabs with %s", (kind) => {
  it("refresh one at a time, each with the refresh token the other left", async () => {
    const b = browser(kind);
    b.storage.data.set(AUTH_KEY, JSON.stringify({ refresh_token: "rt-0", login_id: X }));
    const [a, c] = [b.tab("A"), b.tab("C")];
    const starts = [track(a.tm.start()), track(c.tm.start())];

    await until(() => b.server.calls.length === 1, "the first refresh");
    await vi.advanceTimersByTimeAsync(2_000);
    expect(b.server.calls).toHaveLength(1);
    expect(b.server.calls[0]?.body).toEqual({ refresh_token: "rt-0" });
    b.server.calls[0]?.answer(json(200, b.server.tokens(20)));
    await until(() => b.server.calls.length === 2, "the second refresh");
    expect(b.server.calls[1]?.body).toEqual({ refresh_token: "rt-1" });
    b.server.calls[1]?.answer(json(200, b.server.tokens(20)));
    await until(() => starts.every((s) => s.settled), "the starts");
    expect([a.tm.state, c.tm.state]).toEqual([
      { status: "signed-in", loginId: X },
      { status: "signed-in", loginId: X },
    ]);

    // Both tokens run out; both tabs need one at once.
    await vi.advanceTimersByTimeAsync(10_000);
    const tokens = [track(a.tm.accessToken()), track(c.tm.accessToken())];
    await until(() => b.server.calls.length === 3, "a refresh");
    await vi.advanceTimersByTimeAsync(2_000);
    expect(b.server.calls).toHaveLength(3);
    expect(b.server.calls[2]?.body).toEqual({ refresh_token: "rt-2" });
    b.server.calls[2]?.answer(json(200, b.server.tokens(20)));
    await until(() => b.server.calls.length === 4, "the other refresh");
    expect(b.server.calls[3]?.body).toEqual({ refresh_token: "rt-3" });
    b.server.calls[3]?.answer(json(200, b.server.tokens(20)));
    await until(() => tokens.every((t) => t.settled), "the tokens");
    expect(new Set(tokens.map((t) => t.value))).toEqual(new Set(["at-3", "at-4"]));
    expect(b.stored()).toEqual({ refresh_token: "rt-4", login_id: X });
  });

  it("make a sign-in wait for another tab's refresh, which then leaves the new record alone", async () => {
    const b = browser(kind);
    const [a] = await signedIn(b, ["A"]);
    const other = b.tab("C");
    await vi.advanceTimersByTimeAsync(10_000);
    const token = track(a!.tm.accessToken());
    await until(() => b.server.calls.length === 1, "the refresh");

    const signIn = track(other.tm.signIn({ ...b.server.tokens(), refresh_token: "rt-y" }));
    await vi.advanceTimersByTimeAsync(2_000);
    expect(signIn.settled).toBe(false);
    expect(b.stored()).toEqual({ refresh_token: "rt-1", login_id: X });

    // As a browser may, the tabs hear of each other's writes only after the sign-in: C, of A's refresh.
    b.storage.hold();
    b.server.calls[0]?.answer(json(200, b.server.tokens()));
    await until(() => signIn.settled && token.settled, "the sign-in");
    const y = other.tm.state.loginId;
    expect(y).not.toBe(X);
    expect(b.stored()).toEqual({ refresh_token: "rt-y", login_id: y });
    b.storage.deliver();
    expect([a!.tm.state, other.tm.state]).toEqual([
      { status: "signed-in", loginId: y },
      { status: "signed-in", loginId: y },
    ]);
  });

  it.each([
    [
      "answers",
      () =>
        json(200, {
          token_type: "Bearer",
          access_token: "at-x",
          access_token_expires_in: 900,
          refresh_token: "rt-x",
          refresh_token_expires_at: "2026-10-27T00:00:00Z",
        }),
    ],
    ["refuses", () => problem(401, "identity.refresh_token_invalid")],
  ])("drop a refresh whose record changed to another account on the way, when the server %s it", async (_, answer) => {
    const b = browser(kind);
    const [a] = await signedIn(b, ["A"]);
    await vi.advanceTimersByTimeAsync(10_000);
    const token = track(a!.tm.accessToken());
    await until(() => b.server.calls.length === 1, "the refresh");
    expect(b.server.calls[0]?.body).toEqual({ refresh_token: "rt-1" });

    // Another tab signs in as Y while the refresh is out: without navigator.locks, the lease is not
    // atomic, so this can happen (M1/P5 design 3.2).
    b.storage.write(AUTH_KEY, JSON.stringify({ refresh_token: "rt-y", login_id: Y }));
    b.server.calls[0]?.answer(answer());
    await until(() => token.settled, "the refresh's end");

    expect(token.error).toBeInstanceOf(SessionChangedError);
    expect(b.stored()).toEqual({ refresh_token: "rt-y", login_id: Y });
    expect(a!.tm.state).toEqual({ status: "signed-in", loginId: Y });

    // The tab goes on as Y: its next token comes from Y's refresh token.
    const next = track(a!.tm.accessToken());
    await until(() => b.server.calls.length === 2, "Y's refresh");
    expect(b.server.calls[1]?.body).toEqual({ refresh_token: "rt-y" });
    b.server.calls[1]?.answer(json(200, b.server.tokens()));
    await until(() => next.settled, "Y's token");
    expect(b.stored()).toEqual({ refresh_token: "rt-2", login_id: Y });
  });

  it("end the session in every tab when one signs out", async () => {
    const b = browser(kind);
    const [a, c] = await signedIn(b, ["A", "C"], 900);
    const out = track(a!.tm.signOut());
    await until(() => b.server.calls.length === 1, "the logout");
    expect(b.server.calls[0]).toMatchObject({ path: LOGOUT, body: { refresh_token: "rt-2" } });
    b.server.calls[0]?.answer(noContent());
    await until(() => out.settled && c!.tm.state.status === "signed-out", "the other tab's sign-out");

    expect(c!.changes).toEqual(["signed-out -"]);
    expect(await c!.tm.accessToken()).toBeUndefined();
    expect(b.server.calls).toHaveLength(1);
  });

  it("sign out with the refresh token another tab's refresh just wrote", async () => {
    const b = browser(kind);
    const [a, c] = await signedIn(b, ["A", "C"], 900);
    const token = track(c!.tm.renew("at-2"));
    await until(() => b.server.calls.length === 1, "the refresh");
    const out = track(a!.tm.signOut());
    await vi.advanceTimersByTimeAsync(1_000);
    expect(b.server.calls).toHaveLength(1);

    // As a browser may, the tabs hear of each other's writes only after the sign-out: A, of C's refresh.
    b.storage.hold();
    b.server.calls[0]?.answer(json(200, b.server.tokens()));
    await until(() => b.server.calls.length === 2, "the logout");
    expect(b.server.calls[1]).toMatchObject({ path: LOGOUT, body: { refresh_token: "rt-3" } });
    b.server.calls[1]?.answer(noContent());
    await until(() => out.settled && token.settled, "the sign-out");
    expect(b.storage.data.has(AUTH_KEY)).toBe(false);
    b.storage.deliver();
    expect([a!.tm.state, c!.tm.state]).toEqual([{ status: "signed-out" }, { status: "signed-out" }]);
  });

  it("sign a signed-out tab in when another tab signs in", async () => {
    const b = browser(kind);
    const [a, c] = [b.tab("A"), b.tab("C")];
    await Promise.all([a.tm.start(), c.tm.start()]);
    expect(c.tm.state).toEqual({ status: "signed-out" });

    const signIn = track(a.tm.signIn(b.server.tokens()));
    await until(() => signIn.settled, "the sign-in");
    const x = a.tm.state.loginId;
    await until(() => c.tm.state.status === "signed-in", "the other tab's sign-in");
    expect(c.tm.state).toEqual({ status: "signed-in", loginId: x });

    // It has no access token yet: it refreshes with the record, under the lock.
    const token = track(c.tm.accessToken());
    await until(() => b.server.calls.length === 1, "its refresh");
    expect(b.server.calls[0]?.body).toEqual({ refresh_token: "rt-1" });
    b.server.calls[0]?.answer(json(200, b.server.tokens()));
    await until(() => token.settled, "its token");
    expect(token.value).toBe("at-2");
    expect(b.stored()).toEqual({ refresh_token: "rt-2", login_id: x });
  });

  it("switch every tab to the account another tab signs in as", async () => {
    const b = browser(kind);
    const [a, c] = await signedIn(b, ["A", "C"], 900);
    const other = b.tab("D");
    const signIn = track(other.tm.signIn({ ...b.server.tokens(), refresh_token: "rt-y" }));
    await until(() => signIn.settled, "the sign-in");
    const y = other.tm.state.loginId;
    expect(y).not.toBe(X);
    await until(() => a!.tm.state.loginId === y && c!.tm.state.loginId === y, "the switch");
    expect(a!.changes).toEqual([`signed-in ${y}`]);

    // The old access token is gone: the next request of either tab uses Y's session.
    const token = track(a!.tm.accessToken());
    await until(() => b.server.calls.length === 1, "Y's refresh");
    expect(b.server.calls[0]?.body).toEqual({ refresh_token: "rt-y" });
    b.server.calls[0]?.answer(json(200, b.server.tokens()));
    await until(() => token.settled, "Y's token");
    expect(token.value).toBe("at-4");
  });

  it("leave a tab alone when another tab only refreshed", async () => {
    const b = browser(kind);
    const [a, c] = await signedIn(b, ["A", "C"], 900);
    const before = c!.tm.state;
    const ownToken = track(c!.tm.accessToken());
    await until(() => ownToken.settled, "the token");

    const token = track(a!.tm.renew("at-1"));
    await until(() => b.server.calls.length === 1, "the refresh");
    b.storage.hold();
    b.server.calls[0]?.answer(json(200, b.server.tokens()));
    await until(() => token.settled, "the token");
    // C hears of A's refresh now.
    b.storage.deliver();

    expect(b.stored()).toEqual({ refresh_token: "rt-3", login_id: X });
    expect(c!.tm.state).toBe(before);
    expect(c!.changes).toEqual([]);
    const again = track(c!.tm.accessToken());
    await vi.advanceTimersByTimeAsync(0);
    expect(again.value).toBe(ownToken.value);
    expect(b.server.calls).toHaveLength(1);
  });

  it("free the lock 8 s into a refresh that does not come back, before a lease would expire", async () => {
    const b = browser(kind);
    const [a, c] = await signedIn(b, ["A", "C"], 20);
    await vi.advanceTimersByTimeAsync(10_000);
    const first = track(a!.tm.accessToken());
    await until(() => b.server.calls.length === 1, "the refresh");
    const sentAt = b.server.calls[0]?.at ?? 0;
    const second = track(c!.tm.accessToken());

    await vi.advanceTimersByTimeAsync(sentAt + 8_000 - 1 - Date.now());
    expect(b.server.calls[0]?.aborted()).toBe(false);
    expect(b.server.calls).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(b.server.calls[0]?.aborted()).toBe(true);
    await until(() => first.settled, "the first refresh's end");
    expect(first.error).toBeInstanceOf(SessionUnavailableError);

    // The other tab's refresh goes out at once, not when the 10 s lease would run out.
    await until(() => b.server.calls.length === 2, "the other tab's refresh");
    expect(Date.now() - sentAt).toBeLessThan(8_500);
    b.server.calls[1]?.answer(json(200, b.server.tokens()));
    await until(() => second.settled, "the other tab's token");
    expect(second.value).toBe("at-3");
  });
});
