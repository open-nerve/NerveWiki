import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { authMiddleware } from "./auth-middleware";
import { RecordingLock, SharedStorage } from "./testing/fake-browser";
import { FakeServer, json } from "./testing/fake-server";
import { track, until } from "./testing/fake-time";
import { AUTH_KEY, SessionChangedError, TokenManager } from "./token-manager";

// A client serves the session of the stores it was built for (M1/P5 design 3.2: a tab never writes as the wrong
// account): once the tab is in another session, it sends nothing; changes within its session do not stop it.
// A real token manager, against a fake server that answers when the test says.

const REFRESH = "/api/v0/auth/refresh";
const ME = "/api/v0/me";

const X = "0123456789abcdef0123456789abcdef";
/** The session of another account, Y, which another tab signs in to. */
const recordY = JSON.stringify({ refresh_token: "rt-y", login_id: "fedcba9876543210fedcba9876543210" });
/** The session this tab signs in to itself. */
const Z = "abababababababababababababababab";

/**
 * A tab in the session X, or signed out, and the client of its stores, built as the app builds it: for the
 * session the tab is in as it loads, before the first refresh has decided it (store-context.tsx).
 */
async function setUp(record = true) {
  const storage = new SharedStorage();
  if (record) storage.data.set(AUTH_KEY, JSON.stringify({ refresh_token: "rt-0", login_id: X }));
  const server = new FakeServer();
  const view = storage.tab("A");
  const tm = new TokenManager({
    storage: view,
    lock: new RecordingLock(),
    client: server.client(),
    now: () => Date.now(),
    randomHex: () => Z,
  });
  view.onStorage((key) => {
    if (key === AUTH_KEY) tm.handleStorageChange();
  });
  const started = track(tm.start());
  const api = server.client();
  api.use(authMiddleware(tm, tm.state.loginId));
  if (record) {
    await until(() => server.calls.length === 1, "the first refresh");
    server.calls[0]?.answer(json(200, server.tokens()));
  }
  await until(() => started.settled, "the start");
  server.calls.length = 0;
  return { storage, server, tm, api };
}

beforeEach(() => {
  vi.useFakeTimers({ now: 1_000_000 });
});
afterEach(() => {
  vi.useRealTimers();
});

describe("a client bound to a session", () => {
  it.each([
    ["signed in as another account", recordY],
    ["signed out", null],
  ])("sends nothing once the tab has followed another tab that %s", async (_, record) => {
    const { storage, server, tm, api } = await setUp();
    storage.write(AUTH_KEY, record);
    await until(() => tm.state.loginId !== X, "the tab following the other tab");
    const saved = track(api.PATCH(ME, { body: { display_name: "Alice" } }));
    await until(() => saved.settled || server.calls.length > 0, "the answer, or a request");

    // Not sent with the other session's token, nor as its refresh, nor without a token.
    expect(saved.error).toBeInstanceOf(SessionChangedError);
    await vi.advanceTimersByTimeAsync(1_000);
    expect(server.calls).toEqual([]);
  });

  it("still sends after changes within its session: the first refresh, another tab's refresh, its own", async () => {
    const { storage, server, tm, api } = await setUp();
    // Built while the session was starting; the first refresh made the tab signed in, in the same session.
    expect(tm.state).toEqual({ status: "signed-in", loginId: X });
    // Another tab refreshes X: a new refresh token in the record, the same login_id.
    storage.write(AUTH_KEY, JSON.stringify({ refresh_token: "rt-9", login_id: X }));
    await vi.advanceTimersByTimeAsync(0);
    const me = track(api.GET(ME));
    await until(() => server.calls.length === 1, "the request");
    expect(server.calls[0]).toMatchObject({ method: "GET", path: ME, authorization: "Bearer at-1" });
    server.calls[0]?.answer(json(200, {}));
    await until(() => me.settled, "the answer");
    expect(me.value?.response.status).toBe(200);

    // The access token runs out: the tab refreshes, with the other tab's refresh token, then sends.
    await vi.advanceTimersByTimeAsync(900_000);
    const saved = track(api.PATCH(ME, { body: { display_name: "Alice" } }));
    await until(() => server.calls.length === 2, "the refresh");
    expect(server.calls[1]).toMatchObject({ path: REFRESH, body: { refresh_token: "rt-9" } });
    server.calls[1]?.answer(json(200, server.tokens()));
    await until(() => server.calls.length === 3, "the request");
    expect(server.calls[2]).toMatchObject({ method: "PATCH", path: ME, authorization: "Bearer at-2" });
    server.calls[2]?.answer(json(200, { display_name: "Alice" }));
    await until(() => saved.settled, "the answer");
    expect(saved.value?.response.status).toBe(200);
  });

  it("of stores built without a session sends nothing once the tab has signed in", async () => {
    const { server, tm, api } = await setUp(false);
    await tm.signIn(server.tokens());
    expect(tm.state).toEqual({ status: "signed-in", loginId: Z });
    const me = track(api.GET(ME));
    await until(() => me.settled || server.calls.length > 0, "the answer, or a request");

    expect(me.error).toBeInstanceOf(SessionChangedError);
    await vi.advanceTimersByTimeAsync(1_000);
    expect(server.calls).toEqual([]);
  });
});
