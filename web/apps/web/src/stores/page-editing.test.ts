import { afterEach, expect, test, vi } from "vitest";

import { ApiError } from "../services/api";
import type { PageContent } from "../services/page.service";
import { editSessionHeartbeat, PageEditing, type EditingService } from "./page-editing";

// One edit of a page (M4/P6 design 3.6): the content it begins on, its
// session and the session's heartbeat, its saves.

afterEach(() => vi.useRealTimers());

const refusal = (status: number, code: string, retryAfter?: number) =>
  new ApiError(status, { status, code, title: code }, retryAfter);

type Answer<T> = () => T | Promise<T>;

/** A service whose answers the test sets; what went out is in sent. */
function fakeService() {
  const sent: string[] = [];
  let opened = 0;
  const answers = {
    content: (): PageContent | Promise<PageContent> => ({ content: "# Notes\n", revision: 3, content_hash: "" }),
    put: undefined as Answer<{ revision: number }> | undefined,
    open: undefined as Answer<{ id: string }> | undefined,
    beat: undefined as Answer<unknown> | undefined,
  };
  let revision = 3;
  const service = {
    getPageContent: vi.fn(async () => answers.content()),
    putPageContent: vi.fn(
      async (_: string, write: { content: string; base_revision: number; edit_session_id?: string }) => {
        sent.push(`PUT ${JSON.stringify(write.content)} on ${write.base_revision} in ${write.edit_session_id}`);
        const page = answers.put === undefined ? { revision: ++revision } : await answers.put();
        return { ...page } as never;
      }
    ),
    openEditSession: vi.fn(async () => {
      sent.push("OPEN");
      return answers.open === undefined ? { id: `s${++opened}` } : await answers.open();
    }) as never,
    heartbeatEditSession: vi.fn(async (id: string) => {
      sent.push(`BEAT ${id}`);
      return answers.beat === undefined ? {} : await answers.beat();
    }) as never,
    endEditSession: vi.fn(async (id: string) => {
      sent.push(`END ${id}`);
    }),
  } satisfies EditingService;
  return { service, sent, answers };
}

/** An edit of p1 begun, its content read. */
async function begun() {
  const fake = fakeService();
  const editing = new PageEditing(fake.service, "p1");
  editing.start();
  await vi.waitFor(() => expect(editing.content).toBeDefined());
  return { ...fake, editing };
}

test("the heartbeat's interval is 20 seconds, the server's lease a third of 60 (domain/session.go)", () => {
  expect(editSessionHeartbeat).toBe(20_000);
});

test("begins by reading the content and opening the session together; a read that fails is read again", async () => {
  const { service, sent, answers } = fakeService();
  answers.content = () => Promise.reject(new TypeError("offline"));
  const editing = new PageEditing(service, "p1");

  editing.start();
  await vi.waitFor(() => expect(editing.readFailure).toBeInstanceOf(TypeError));
  expect(sent).toEqual(["OPEN"]);
  answers.content = () => ({ content: "x", revision: 7, content_hash: "" });
  await editing.read();
  expect(editing.content?.revision).toBe(7);
  expect(editing.readFailure).toBeUndefined();
  editing.end();
});

test("a save goes on the revision read, in the session; the next goes on the one the first answered", async () => {
  const { editing, sent } = await begun();

  editing.changed(1);
  expect(editing.unsaved).toBe(true);
  expect(await editing.save("one", 1)).toBe(true);
  editing.changed(2);
  expect(await editing.save("two", 2)).toBe(true);
  expect(sent).toEqual(["OPEN", 'PUT "one" on 3 in s1', 'PUT "two" on 4 in s1']);
  expect(editing.unsaved).toBe(false);
  expect(editing.saved).toBe(true);
  editing.end();
});

test("nothing unsaved, a save sends nothing", async () => {
  const { editing, sent } = await begun();

  expect(await editing.save("# Notes\n", 0)).toBe(true);
  expect(sent).toEqual(["OPEN"]);
  editing.end();
});

test("one save is out at a time; those asked for meanwhile go once, with the latest text", async () => {
  const { editing, sent, answers } = await begun();
  let answer: ((page: { revision: number }) => void) | undefined;
  answers.put = () => new Promise((resolve) => (answer = resolve));

  const first = editing.save("one", 1);
  await vi.waitFor(() => expect(sent).toHaveLength(2));
  const second = editing.save("two", 2);
  const third = editing.save("three", 3);
  answers.put = undefined;
  answer?.({ revision: 4 });
  expect(await Promise.all([first, second, third])).toEqual([true, true, true]);
  expect(sent).toEqual(["OPEN", 'PUT "one" on 3 in s1', 'PUT "three" on 4 in s1']);
  expect(editing.savedVersion).toBe(3);
  editing.end();
});

test("a save whose session ended opens another and goes once more", async () => {
  const { editing, sent, answers } = await begun();
  answers.put = () => {
    answers.put = undefined;
    throw refusal(409, "page.edit_session_ended");
  };

  expect(await editing.save("text", 1)).toBe(true);
  expect(sent).toEqual(["OPEN", 'PUT "text" on 3 in s1', "OPEN", 'PUT "text" on 3 in s2']);
  editing.end();
});

test("a session that ends again is a failure, not a loop", async () => {
  const { editing, sent, answers } = await begun();
  answers.put = () => {
    throw refusal(409, "page.edit_session_ended");
  };

  editing.changed(1);
  expect(await editing.save("text", 1)).toBe(false);
  expect(sent.filter((line) => line.startsWith("PUT"))).toHaveLength(2);
  expect(editing.failure).toBeInstanceOf(ApiError);
  expect(editing.unsaved).toBe(true);
  editing.end();
});

test("a 503 server_busy is waited out by its Retry-After, three times at most", async () => {
  const { editing, sent, answers } = await begun();
  vi.useFakeTimers();
  answers.put = () => {
    throw refusal(503, "server_busy", 2);
  };

  const saving = editing.save("text", 1);
  const puts = () => sent.filter((line) => line.startsWith("PUT")).length;
  // vi.waitFor would move the fake clock on: the answer is in once the promises settle.
  await vi.advanceTimersByTimeAsync(0);
  expect(editing.busy).toBe(true);
  // Not sent again before its Retry-After, 2 seconds, is up.
  await vi.advanceTimersByTimeAsync(1999);
  expect(puts()).toBe(1);
  await vi.advanceTimersByTimeAsync(1);
  expect(puts()).toBe(2);
  await vi.advanceTimersByTimeAsync(2 * 2000);
  expect(await saving).toBe(false);
  expect(puts()).toBe(4);
  expect(editing.busy).toBe(false);
  expect(editing.failure).toBeInstanceOf(ApiError);
  editing.end();
});

test("a 503 waited out saves when the server can", async () => {
  const { editing, answers } = await begun();
  vi.useFakeTimers();
  answers.put = () => {
    answers.put = undefined;
    throw refusal(503, "server_busy", 1);
  };

  const saving = editing.save("text", 1);
  await vi.advanceTimersByTimeAsync(1000);
  expect(await saving).toBe(true);
  expect(editing.failure).toBeUndefined();
  editing.end();
});

test("another refusal is the save's failure; the next save clears it", async () => {
  const { editing, answers } = await begun();
  answers.put = () => {
    throw refusal(400, "bad_request");
  };

  expect(await editing.save("text", 1)).toBe(false);
  expect((editing.failure as ApiError).code).toBe("bad_request");
  answers.put = undefined;
  expect(await editing.save("text", 1)).toBe(true);
  expect(editing.failure).toBeUndefined();
  editing.end();
});

test("the session beats every 20 seconds; one not found is opened anew; forbidden, the beats stop", async () => {
  vi.useFakeTimers();
  const { service, sent, answers } = fakeService();
  const editing = new PageEditing(service, "p1");
  editing.start();
  await vi.advanceTimersByTimeAsync(0);

  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  expect(sent).toEqual(["OPEN", "BEAT s1"]);
  answers.beat = () => {
    throw refusal(404, "page.edit_session_not_found");
  };
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  expect(sent).toEqual(["OPEN", "BEAT s1", "BEAT s1", "OPEN"]);
  answers.beat = () => {
    throw refusal(403, "forbidden");
  };
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  expect(editing.lostAccess).toBe(true);
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat * 3);
  expect(sent).toEqual(["OPEN", "BEAT s1", "BEAT s1", "OPEN", "BEAT s2"]);
  editing.end();
});

test("the tab shown again beats at once", async () => {
  const { editing, sent } = await begun();

  document.dispatchEvent(new Event("visibilitychange"));
  await vi.waitFor(() => expect(sent).toEqual(["OPEN", "BEAT s1"]));
  editing.end();
});

test("the end ends the session unanswered and stops the beats; a session opening as it ends is ended too", async () => {
  vi.useFakeTimers();
  const { editing, sent } = await begun();
  editing.end();
  expect(vi.getTimerCount()).toBe(0);
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat * 2);
  document.dispatchEvent(new Event("visibilitychange"));
  expect(sent).toEqual(["OPEN", "END s1"]);

  const late = fakeService();
  let open: ((session: { id: string }) => void) | undefined;
  late.answers.open = () => new Promise((resolve) => (open = resolve));
  const leaving = new PageEditing(late.service, "p1");
  leaving.start();
  leaving.end();
  open?.({ id: "s9" });
  await vi.advanceTimersByTimeAsync(0);
  expect(late.sent).toEqual(["OPEN", "END s9"]);
});

test("an edit ended and begun again, as StrictMode runs an effect, reads its content once and keeps its one session", async () => {
  vi.useFakeTimers();
  const { service, sent } = fakeService();
  const editing = new PageEditing(service, "p1");
  editing.start();
  editing.end();
  editing.start();
  await vi.advanceTimersByTimeAsync(0);

  expect(editing.content?.revision).toBe(3);
  expect(await editing.save("text", 1)).toBe(true);
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  expect(sent).toEqual(["OPEN", 'PUT "text" on 3 in s1', "BEAT s1"]);
  editing.end();
  editing.start();
  await vi.advanceTimersByTimeAsync(0);
  expect(service.getPageContent).toHaveBeenCalledTimes(1);
  expect(sent.slice(3)).toEqual(["END s1", "OPEN"]);
  editing.end();
});

test("once the edit ends nothing more is sent: a save asked for then, one waiting for its turn, one waiting out a 503", async () => {
  vi.useFakeTimers();
  const { editing, sent, answers } = await begun();
  let answer: (() => void) | undefined;
  answers.put = () => new Promise((resolve) => (answer = () => resolve({ revision: 4 })));
  const first = editing.save("one", 1);
  const second = editing.save("one two", 2);
  await vi.advanceTimersByTimeAsync(0);
  editing.end();
  answer?.();
  expect(await first).toBe(true);
  expect(await second).toBe(false);
  expect(await editing.save("one two three", 3)).toBe(false);
  expect(sent).toEqual(["OPEN", 'PUT "one" on 3 in s1', "END s1"]);

  const busy = await begun();
  busy.answers.put = () => {
    throw refusal(503, "server_busy", 30);
  };
  const waiting = busy.editing.save("text", 1);
  await vi.waitFor(() => expect(busy.editing.busy).toBe(true));
  busy.editing.end();
  expect(vi.getTimerCount()).toBe(0);
  expect(await waiting).toBe(false);
  expect(busy.editing.saving).toBe(false);
  expect(busy.sent).toEqual(["OPEN", 'PUT "text" on 3 in s1', "END s1"]);

  const answered = await begun();
  let refuse: (() => void) | undefined;
  answered.answers.put = () => new Promise((_, reject) => (refuse = () => reject(refusal(503, "server_busy", 30))));
  const out = answered.editing.save("text", 1);
  await vi.advanceTimersByTimeAsync(0);
  answered.editing.end();
  refuse?.();
  await vi.advanceTimersByTimeAsync(0);
  expect(vi.getTimerCount()).toBe(0);
  expect(await out).toBe(false);
  expect(answered.editing.busy).toBe(false);
});

test("a save waiting for the session to open is not sent once the edit ends; the session is ended as it opens", async () => {
  const { service, sent, answers } = fakeService();
  let open: ((session: { id: string }) => void) | undefined;
  answers.open = () => new Promise((resolve) => (open = resolve));
  const editing = new PageEditing(service, "p1");
  editing.start();
  await vi.waitFor(() => expect(editing.content).toBeDefined());

  const saving = editing.save("text", 1);
  editing.end();
  open?.({ id: "s9" });
  expect(await saving).toBe(false);
  expect(sent).toEqual(["OPEN", "END s9"]);
});

test("a save whose session ended after the heartbeat opened another goes in that one", async () => {
  vi.useFakeTimers();
  const { editing, sent, answers } = await begun();
  let answer: (() => void) | undefined;
  answers.put = () => new Promise((_, reject) => (answer = () => reject(refusal(409, "page.edit_session_ended"))));
  const saving = editing.save("text", 1);
  await vi.advanceTimersByTimeAsync(0);
  answers.beat = () => {
    throw refusal(404, "page.edit_session_not_found");
  };
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  answers.put = undefined;
  answer?.();

  expect(await saving).toBe(true);
  expect(sent).toEqual(["OPEN", 'PUT "text" on 3 in s1', "BEAT s1", "OPEN", 'PUT "text" on 3 in s2']);
  editing.end();
});

test("access lost, a save that goes through gets it back: the session beats again", async () => {
  vi.useFakeTimers();
  const { editing, sent, answers } = await begun();
  answers.beat = () => {
    throw refusal(403, "forbidden");
  };
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  expect(editing.lostAccess).toBe(true);
  answers.beat = undefined;

  expect(await editing.save("text", 1)).toBe(true);
  expect(editing.lostAccess).toBe(false);
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  expect(sent).toEqual(["OPEN", "BEAT s1", 'PUT "text" on 3 in s1', "BEAT s1"]);
  editing.end();
  expect(vi.getTimerCount()).toBe(0);
});

/** An edit begun on revision 3 whose next save is refused because the page changed to revision 5, "theirs". */
async function inConflict() {
  const fake = await begun();
  fake.answers.put = () => {
    fake.answers.put = undefined;
    throw refusal(409, "page.revision_mismatch");
  };
  fake.answers.content = () => ({ content: "theirs", revision: 5, content_hash: "" });
  fake.editing.changed(1);
  expect(await fake.editing.save("mine", 1)).toBe(false);
  return fake;
}

test("a save refused because the page changed reads the page as it is now; until kept or discarded, saves send nothing", async () => {
  const { editing, sent } = await inConflict();

  expect(editing.conflict).toEqual({ theirs: "theirs", revision: 5, mine: "mine" });
  expect(editing.saving).toBe(false);
  expect(editing.failure).toBeUndefined();
  editing.changed(2);
  expect(await editing.save("mine, more", 2)).toBe(false);
  expect(sent.filter((line) => line.startsWith("PUT"))).toEqual(['PUT "mine" on 3 in s1']);
  editing.end();
});

test("keep mine saves on the revision the conflict read; a write since is a conflict again", async () => {
  const { editing, sent, answers } = await inConflict();

  expect(await editing.keepMine("mine", 1)).toBe(true);
  expect(sent.at(-1)).toBe('PUT "mine" on 5 in s1');
  expect(editing.conflict).toBeUndefined();
  expect(editing.unsaved).toBe(false);

  answers.put = () => {
    answers.put = undefined;
    throw refusal(409, "page.revision_mismatch");
  };
  answers.content = () => ({ content: "theirs again", revision: 8, content_hash: "" });
  editing.changed(2);
  expect(await editing.save("mine again", 2)).toBe(false);
  expect(await editing.keepMine("mine again", 2)).toBe(true);
  expect(sent.at(-1)).toBe('PUT "mine again" on 8 in s1');
  editing.end();
});

test("keep mine does not read the page again: a write after the conflict read it is a conflict again, not overwritten", async () => {
  const { editing, sent, answers } = await inConflict();
  // Another write lands between the conflict's read (revision 5) and Keep mine.
  answers.content = () => ({ content: "theirs, later", revision: 6, content_hash: "" });
  answers.put = () => {
    answers.put = undefined;
    throw refusal(409, "page.revision_mismatch");
  };

  expect(await editing.keepMine("mine", 1)).toBe(false);
  expect(sent.findLast((line) => line.startsWith("PUT"))).toBe('PUT "mine" on 5 in s1');
  expect(editing.conflict).toEqual({ theirs: "theirs, later", revision: 6, mine: "mine" });
  editing.end();
});

test("discard mine gives the page as it is now, nothing unsaved; the next save goes on its revision", async () => {
  const { editing, sent } = await inConflict();

  expect(editing.discardMine()).toBe("theirs");
  expect(editing.conflict).toBeUndefined();
  expect(editing.unsaved).toBe(false);
  expect(editing.discardMine()).toBeUndefined();
  editing.changed(2);
  expect(await editing.save("theirs, edited", 2)).toBe(true);
  expect(sent.at(-1)).toBe('PUT "theirs, edited" on 5 in s1');
  editing.end();
});

test("a conflict whose page cannot be read is the save's failure", async () => {
  const { editing, answers } = await begun();
  answers.put = () => {
    throw refusal(409, "page.revision_mismatch");
  };
  answers.content = () => Promise.reject(new TypeError("offline"));

  editing.changed(1);
  expect(await editing.save("mine", 1)).toBe(false);
  expect(editing.conflict).toBeUndefined();
  expect((editing.failure as ApiError).code).toBe("page.revision_mismatch");
  editing.end();
});

test("ended while a save is out, the save that goes through after it starts no heartbeat; begun twice, one beats", async () => {
  vi.useFakeTimers();
  const { editing, answers } = await begun();
  answers.beat = () => {
    throw refusal(403, "forbidden");
  };
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  let answer: (() => void) | undefined;
  answers.put = () => new Promise((resolve) => (answer = () => resolve({ revision: 4 })));
  const saving = editing.save("text", 1);
  await vi.advanceTimersByTimeAsync(0);

  editing.end();
  answer?.();
  expect(await saving).toBe(true);
  expect(vi.getTimerCount()).toBe(0);

  const twice = fakeService();
  const begunTwice = new PageEditing(twice.service, "p1");
  begunTwice.start();
  begunTwice.start();
  begunTwice.end();
  expect(vi.getTimerCount()).toBe(0);
});

test("a save waiting out a 503 when the edit ends and begins again is not sent: it waited for the edit that ended", async () => {
  vi.useFakeTimers();
  const { editing, sent, answers } = await begun();
  answers.put = () => {
    throw refusal(503, "server_busy", 30);
  };
  const waiting = editing.save("text", 1);
  await vi.waitFor(() => expect(editing.busy).toBe(true));

  editing.end();
  editing.start();
  expect(await waiting).toBe(false);
  expect(sent.filter((line) => line.startsWith("PUT"))).toHaveLength(1);
  editing.end();
});
