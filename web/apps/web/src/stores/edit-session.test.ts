import { afterEach, expect, test, vi } from "vitest";

import { EditEnded, editSessionHeartbeat, type EditLost } from "./edit-session";
import { fakeSession, lockedBy, lockEvent, refusal } from "./testing/fake-edit-sessions";

// An edit's session (M5/P4 design 3.2, 3.3): the page's lock, opened before
// the edit begins; its beats; what each refusal comes to; the page left and
// back.

afterEach(() => vi.useRealTimers());

/** An edit session of p1 opened, on fake timers. */
async function opened() {
  vi.useFakeTimers();
  const fake = fakeSession();
  expect(await fake.session.open(false)).toEqual({ opened: true });
  return fake;
}

/** A deferred answer: the promise, and how the test settles it. */
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

test("the heartbeat's interval is 20 seconds, a sixth of the server's lease of 120 (domain/session.go)", () => {
  expect(editSessionHeartbeat).toBe(20_000);
});

test("opened, the session beats every 20 seconds; its end stops the beats and listeners, and resolves once answered", async () => {
  const { session, sent, answers, page, events } = await opened();
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat * 2);
  expect(sent).toEqual(["OPEN", "BEAT s1", "BEAT s1"]);

  const answer = deferred<void>();
  answers.end = () => answer.promise;
  let ended = false;
  void session.end().then(() => (ended = true));
  await vi.advanceTimersByTimeAsync(0);
  expect(ended).toBe(false);
  answer.resolve();
  await vi.advanceTimersByTimeAsync(0);
  expect(ended).toBe(true);
  expect(vi.getTimerCount()).toBe(0);
  expect(page.listening()).toBe(0);
  expect(events.count()).toBe(0);
  await session.end();
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat * 3);
  expect(sent).toEqual(["OPEN", "BEAT s1", "BEAT s1", "END s1"]);
});

test("a page someone holds answers who; nothing beats or listens. Taking over says so; another failure is thrown", async () => {
  vi.useFakeTimers();
  const locked = fakeSession();
  locked.answers.open = () => {
    throw lockedBy("Bob");
  };
  expect(await locked.session.open(false)).toEqual({
    opened: false,
    holder: { page_id: "p1", user_id: "u-Bob", display_name: "Bob" },
  });
  expect(vi.getTimerCount()).toBe(0);
  expect(locked.page.listening()).toBe(0);
  expect(locked.session.lost).toBeUndefined();

  const taking = fakeSession();
  expect(await taking.session.open(true)).toEqual({ opened: true });
  expect(taking.sent).toEqual(["OPEN TAKE"]);
  await taking.session.end();

  const offline = fakeSession();
  offline.answers.open = () => Promise.reject(new TypeError("offline"));
  await expect(offline.session.open(false)).rejects.toBeInstanceOf(TypeError);
});

test("a lapsed session is opened anew, never taking over; the lock someone took meanwhile loses the edit, naming them", async () => {
  const { session, sent, answers } = await opened();
  answers.beat = () => {
    answers.beat = undefined;
    throw refusal(404, "page.edit_session_not_found");
  };
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  expect(sent).toEqual(["OPEN", "BEAT s1", "OPEN"]);
  expect(await session.current()).toBe("s2");

  answers.beat = () => {
    throw refusal(404, "page.edit_session_not_found");
  };
  answers.open = () => {
    throw lockedBy("Bob");
  };
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  expect(session.lost).toEqual({
    reason: "taken",
    holder: { page_id: "p1", user_id: "u-Bob", display_name: "Bob" },
  });
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat * 3);
  expect(sent).toEqual(["OPEN", "BEAT s1", "OPEN", "BEAT s2", "OPEN"]);
  expect(vi.getTimerCount()).toBe(0);
  await expect(session.current()).rejects.toBeInstanceOf(EditEnded);
});

test.each<[string, () => Error, (() => Error) | undefined, EditLost]>([
  ["taken over", () => refusal(409, "page.edit_session_taken_over"), undefined, { reason: "taken_over" }],
  [
    "unlocked by an admin",
    () => refusal(409, "page.edit_session_unlocked", { ended_by: { user_id: "u-Ada", display_name: "Ada" } }),
    undefined,
    { reason: "unlocked", by: "Ada" },
  ],
  ["forbidden", () => refusal(403, "forbidden"), undefined, { reason: "no_access" }],
  [
    "lapsed, the page gone",
    () => refusal(404, "page.edit_session_not_found"),
    () => refusal(404, "page.not_found"),
    { reason: "gone" },
  ],
  [
    "ended, the page out of reach",
    () => refusal(409, "page.edit_session_ended"),
    () => refusal(403, "forbidden"),
    { reason: "no_access" },
  ],
])("a beat %s loses the edit for good: nothing beats or opens again", async (_, beat, open, lost) => {
  const { session, sent, answers } = await opened();
  answers.beat = () => {
    throw beat();
  };
  if (open !== undefined) {
    answers.open = () => {
      throw open();
    };
  }
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  expect(session.lost).toEqual(lost);
  const before = sent.length;
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat * 3);
  expect(sent).toHaveLength(before);
  expect(sent.filter((line) => line.startsWith("OPEN"))).toHaveLength(open === undefined ? 1 : 2);
  // Lost, the session has nothing to end.
  await session.end();
  expect(sent).toHaveLength(before);
});

test("an opening anew that fails for a passing reason is tried again by the next beat", async () => {
  const { session, sent, answers } = await opened();
  answers.beat = () => {
    answers.beat = undefined;
    throw refusal(404, "page.edit_session_not_found");
  };
  answers.open = () => {
    answers.open = undefined;
    throw refusal(503, "server_busy", {}, 1);
  };
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  expect(sent).toEqual(["OPEN", "BEAT s1", "OPEN"]);
  expect(session.lost).toBeUndefined();

  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  expect(sent).toEqual(["OPEN", "BEAT s1", "OPEN", "OPEN"]);
  expect(await session.current()).toBe("s2");
  await session.end();
});

test("a beat not answered within a heartbeat is given up, so that the next goes", async () => {
  const { session, sent, answers } = await opened();
  answers.beat = () => {
    answers.beat = undefined;
    return new Promise(() => undefined);
  };
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  expect(sent).toEqual(["OPEN", "BEAT s1"]);

  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  expect(sent).toEqual(["OPEN", "BEAT s1", "BEAT s1"]);
  expect(session.lost).toBeUndefined();
  await session.end();
});

test("ended while a beat is out, the beat is given up: its answer could only find the session gone", async () => {
  const { session, sent, answers } = await opened();
  let beat: AbortSignal | undefined;
  answers.beat = (signal) => {
    beat = signal;
    return new Promise(() => undefined);
  };
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  expect(sent).toEqual(["OPEN", "BEAT s1"]);

  await session.end();
  expect(beat?.aborted).toBe(true);
  expect(sent).toEqual(["OPEN", "BEAT s1", "END s1"]);
  expect(session.lost).toBeUndefined();
});

test("a beat that fails otherwise, a network's, is tried again at the next", async () => {
  const { session, sent, answers } = await opened();
  answers.beat = () => {
    answers.beat = undefined;
    throw new TypeError("offline");
  };
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat * 2);
  expect(session.lost).toBeUndefined();
  expect(sent).toEqual(["OPEN", "BEAT s1", "BEAT s1"]);
  await session.end();
});

test("settle loses the edit on a save's refusal that ends it, not on one the edit goes on from", async () => {
  const { session } = await opened();
  expect(session.settle(refusal(409, "page.edit_session_ended"))).toBe(false);
  expect(session.settle(refusal(503, "server_busy", {}, 1))).toBe(false);
  expect(session.settle(refusal(409, "page.revision_mismatch"))).toBe(false);
  expect(session.lost).toBeUndefined();
  expect(session.settle(refusal(409, "page.edit_session_taken_over"))).toBe(true);
  expect(session.lost).toEqual({ reason: "taken_over" });
  expect(session.settle(new TypeError("offline"))).toBe(true);
  expect(session.lost).toEqual({ reason: "taken_over" });
});

test("a session forgotten is opened anew by the next save, one open for those asking meanwhile", async () => {
  const { session, sent } = await opened();
  session.forget("s0");
  expect(await session.current()).toBe("s1");
  session.forget("s1");
  expect(await Promise.all([session.current(), session.current()])).toEqual(["s2", "s2"]);
  expect(sent).toEqual(["OPEN", "OPEN"]);
  await session.end();
});

test("a beat answered after the session changed does not drop the new one", async () => {
  const { session, sent, answers } = await opened();
  const beat = deferred<unknown>();
  answers.beat = () => beat.promise;
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  session.forget("s1");
  expect(await session.current()).toBe("s2");
  beat.reject(refusal(404, "page.edit_session_not_found"));
  await vi.advanceTimersByTimeAsync(0);
  expect(await session.current()).toBe("s2");
  expect(sent).toEqual(["OPEN", "BEAT s1", "OPEN"]);
  await session.end();
});

test("the session beats at once when the tab is shown, on its page's lock events, on each connection of the stream", async () => {
  const { session, sent, page, events } = await opened();
  const beats = () => sent.filter((line) => line.startsWith("BEAT")).length;
  page.show(false);
  events.emit(lockEvent("p2"));
  events.emit({ type: "pages", data: { workspace_id: "w1", notebook_id: "n1", tree: true, pages: [] } });
  await vi.advanceTimersByTimeAsync(0);
  expect(beats()).toBe(0);
  page.show(true);
  await vi.advanceTimersByTimeAsync(0);
  expect(beats()).toBe(1);
  events.emit(lockEvent("p1", "s7"));
  await vi.advanceTimersByTimeAsync(0);
  expect(beats()).toBe(2);
  events.emit({ type: "connected" });
  await vi.advanceTimersByTimeAsync(0);
  expect(beats()).toBe(3);
  await session.end();
});

test("one beat is out at a time: those asked for meanwhile go once after it, none once it lost the edit", async () => {
  const { session, sent, answers, events } = await opened();
  const first = deferred<unknown>();
  answers.beat = () => first.promise;
  events.emit(lockEvent("p1"));
  events.emit(lockEvent("p1", "s2"));
  events.emit({ type: "connected" });
  await vi.advanceTimersByTimeAsync(0);
  expect(sent).toEqual(["OPEN", "BEAT s1"]);
  answers.beat = undefined;
  first.resolve({});
  await vi.advanceTimersByTimeAsync(0);
  expect(sent).toEqual(["OPEN", "BEAT s1", "BEAT s1"]);

  const taken = deferred<unknown>();
  answers.beat = () => taken.promise;
  events.emit(lockEvent("p1"));
  events.emit(lockEvent("p1", "s2"));
  await vi.advanceTimersByTimeAsync(0);
  taken.reject(refusal(409, "page.edit_session_taken_over"));
  await vi.advanceTimersByTimeAsync(0);
  expect(session.lost).toEqual({ reason: "taken_over" });
  expect(sent).toEqual(["OPEN", "BEAT s1", "BEAT s1", "BEAT s1"]);
});

test("the page left sends the session's end at once; back from the back-forward cache, the session beats first and goes on", async () => {
  const { session, sent, page, left } = await opened();
  page.fire("pagehide");
  expect(left).toEqual(["s1"]);
  page.fire("pageshow", false);
  await vi.advanceTimersByTimeAsync(0);
  expect(sent).toEqual(["OPEN"]);

  // The end did not go out: the session is alive, and the edit's again.
  page.fire("pageshow", true);
  await vi.advanceTimersByTimeAsync(0);
  expect(sent).toEqual(["OPEN", "BEAT s1"]);
  expect(await session.current()).toBe("s1");
  await session.end();
  page.fire("pagehide");
  expect(left).toEqual(["s1"]);
});

test("back from the back-forward cache after the end went out, the lock is taken again, or the edit lost to who took it", async () => {
  const back = await opened();
  back.page.fire("pagehide");
  back.answers.beat = () => {
    throw refusal(404, "page.edit_session_not_found");
  };
  back.page.fire("pageshow", true);
  await vi.advanceTimersByTimeAsync(0);
  expect(back.sent).toEqual(["OPEN", "BEAT s1", "OPEN"]);
  expect(await back.session.current()).toBe("s2");
  await back.session.end();

  const taken = await opened();
  taken.page.fire("pagehide");
  taken.answers.beat = () => {
    throw refusal(404, "page.edit_session_not_found");
  };
  taken.answers.open = () => {
    throw lockedBy("Bob");
  };
  taken.page.fire("pageshow", true);
  await vi.advanceTimersByTimeAsync(0);
  expect(taken.session.lost).toMatchObject({ reason: "taken" });
});

test("ended while it opens, the session is ended as it opens, the end resolving once that is answered; nothing follows the page", async () => {
  vi.useFakeTimers();
  const { session, sent, answers, page } = fakeSession();
  const open = deferred<{ id: string }>();
  answers.open = () => open.promise;
  const opening = session.open(false);
  await vi.advanceTimersByTimeAsync(0);
  const ended = deferred<void>();
  answers.end = () => ended.promise;
  let done = false;
  void session.end().then(() => (done = true));
  open.resolve({ id: "s9" });
  await expect(opening).rejects.toBeInstanceOf(EditEnded);
  expect(sent).toEqual(["OPEN", "END s9"]);
  await vi.advanceTimersByTimeAsync(0);
  expect(done).toBe(false);
  ended.resolve();
  await vi.advanceTimersByTimeAsync(0);
  expect(done).toBe(true);
  expect(page.listening()).toBe(0);
  expect(vi.getTimerCount()).toBe(0);
});
