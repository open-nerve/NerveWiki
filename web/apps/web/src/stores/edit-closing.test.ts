import { afterEach, beforeEach, expect, test, vi } from "vitest";

import { FakeChannels } from "../events/testing/fake-channels";
import { answerClosings, closeOtherTabs, closingAnswerWait } from "./edit-closing";

// A sign-out's closing of the other tabs' edits (M4–M5 Codex review R2),
// over the channels of one browser.

beforeEach(() => void vi.useFakeTimers());
afterEach(() => void vi.useRealTimers());

const login = { loginId: "login-0", tabId: "tab-a" };

/** Whether promise has resolved, once the microtasks so far have run. */
async function settled(promise: Promise<unknown>): Promise<boolean> {
  let done = false;
  void promise.then(() => (done = true));
  await vi.advanceTimersByTimeAsync(0);
  return done;
}

/** A tab of login-0, tabId, whose edits close as close resolves. */
function tab(channels: FakeChannels, tabId: string, hasEdits = true) {
  let closed: (() => void) | undefined;
  const closes: string[] = [];
  const unsubscribe = answerClosings(
    channels.port("nwiki.edits"),
    { loginId: "login-0", tabId },
    () => hasEdits,
    () => {
      closes.push(tabId);
      return new Promise<void>((resolve) => (closed = resolve));
    }
  );
  return { closes, close: () => closed?.(), unsubscribe };
}

test("with no other tab that has edits, a sign-out waits only for the answers", async () => {
  const channels = new FakeChannels();
  const b = tab(channels, "tab-b", false);
  const closing = closeOtherTabs(channels.port("nwiki.edits"), login, "c1", 2_000);

  await vi.advanceTimersByTimeAsync(closingAnswerWait - 1);
  expect(await settled(closing)).toBe(false);
  await vi.advanceTimersByTimeAsync(1);
  expect(await settled(closing)).toBe(true);
  expect(b.closes).toEqual([]);
});

test("a sign-out waits for each tab that has edits until it closed them", async () => {
  const channels = new FakeChannels();
  const [b, c] = [tab(channels, "tab-b"), tab(channels, "tab-c")];
  const closing = closeOtherTabs(channels.port("nwiki.edits"), login, "c1", 2_000);

  await vi.advanceTimersByTimeAsync(closingAnswerWait);
  expect([...b.closes, ...c.closes]).toEqual(["tab-b", "tab-c"]);
  b.close();
  await vi.advanceTimersByTimeAsync(500);
  expect(await settled(closing)).toBe(false);
  c.close();
  expect(await settled(closing)).toBe(true);
});

test("a tab that closes its edits before the answers are in is waited for no more", async () => {
  const channels = new FakeChannels();
  const b = tab(channels, "tab-b");
  const closing = closeOtherTabs(channels.port("nwiki.edits"), login, "c1", 2_000);

  await vi.advanceTimersByTimeAsync(0);
  b.close();
  await vi.advanceTimersByTimeAsync(closingAnswerWait);
  expect(await settled(closing)).toBe(true);
});

test("a tab that does not close its edits holds the sign-out as long as within at most", async () => {
  const channels = new FakeChannels();
  tab(channels, "tab-b");
  const closing = closeOtherTabs(channels.port("nwiki.edits"), login, "c1", 2_000);

  await vi.advanceTimersByTimeAsync(1_999);
  expect(await settled(closing)).toBe(false);
  await vi.advanceTimersByTimeAsync(1);
  expect(await settled(closing)).toBe(true);
});

test("a tab answers its own login's other tabs only: not itself, nor another login's", async () => {
  const channels = new FakeChannels();
  const a = tab(channels, "tab-a");
  const elsewhere = tab(channels, "tab-d");
  const otherLogin: string[] = [];
  const unsubscribe = answerClosings(
    channels.port("nwiki.edits"),
    { loginId: "login-1", tabId: "tab-e" },
    () => true,
    () => {
      otherLogin.push("tab-e");
      return new Promise<void>(() => undefined);
    }
  );
  const closing = closeOtherTabs(channels.port("nwiki.edits"), login, "c1", 2_000);

  await vi.advanceTimersByTimeAsync(closingAnswerWait);
  elsewhere.close();
  expect(await settled(closing)).toBe(true);
  expect([a.closes, elsewhere.closes, otherLogin]).toEqual([[], ["tab-d"], []]);
  unsubscribe();
});

test("a tab unsubscribed answers no more", async () => {
  const channels = new FakeChannels();
  const b = tab(channels, "tab-b");
  b.unsubscribe();
  const closing = closeOtherTabs(channels.port("nwiki.edits"), login, "c1", 2_000);

  await vi.advanceTimersByTimeAsync(closingAnswerWait);
  expect(await settled(closing)).toBe(true);
  expect(b.closes).toEqual([]);
});
