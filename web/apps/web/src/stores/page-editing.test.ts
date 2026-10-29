import { EditorView } from "@codemirror/view";
import { act, configure, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, test, vi } from "vitest";

import { autosavePause } from "../editor/autosave";
import { editorExtensions, type EditorControls, type EditorExtension } from "../editor/registry";
import { ApiError } from "../services/api";
import type { PageContent } from "../services/page.service";
import { problem, type Answer as HttpAnswer } from "../test/fakes";
import { pageEditor } from "../test/page-editor";
import { ada, guide, notes, pagePath, pageServer } from "../test/page-server";
import { renderApp } from "../test/render";
import { editSessionHeartbeat } from "./edit-session";
import { PageEditing, type EditingService } from "./page-editing";
import { RootStore } from "./root.store";
import { fakeSession, lockedBy, refusal } from "./testing/fake-edit-sessions";

// One edit of a page (M4/P6 design 3.6; M5/P4 design 3.4): its session,
// opened first; the content it begins on, read once the lock is the
// edit's; its saves.

beforeAll(() => configure({ reactStrictMode: true }));
afterAll(() => configure({ reactStrictMode: false }));
afterEach(() => vi.useRealTimers());

type Answer<T> = () => T | Promise<T>;

/** A service and session whose answers the test sets; what went out is in sent; record has the edits whose session is open. */
function fakeService() {
  const sent: string[] = [];
  const fake = fakeSession(sent);
  // One set of answers: the content's, and the session's.
  const answers = Object.assign(fake.answers, {
    content: (): PageContent | Promise<PageContent> => ({ content: "# Notes\n", revision: 3, content_hash: "" }),
    put: undefined as Answer<{ revision: number }> | undefined,
  });
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
  } satisfies EditingService;
  const record = new Set<PageEditing>();
  const editing = new PageEditing(service, fake.session, { id: "n1", workspace_id: "w1" }, record);
  return { ...fake, service, sent, answers, record, editing };
}

/** An edit of p1 begun, its content read. */
async function begun() {
  const fake = fakeService();
  expect(await fake.editing.begin(false)).toEqual({ opened: true });
  await vi.waitFor(() => expect(fake.editing.content).toBeDefined());
  return fake;
}

test("begins by opening the session, then reads the content; a read that fails is read again", async () => {
  const { editing, service, answers } = fakeService();
  answers.content = () => Promise.reject(new TypeError("offline"));
  let open: ((session: { id: string }) => void) | undefined;
  answers.open = () => new Promise((resolve) => (open = resolve));

  const beginning = editing.begin(false);
  await vi.waitFor(() => expect(open).toBeDefined());
  expect(service.getPageContent).not.toHaveBeenCalled();
  open?.({ id: "s1" });
  await beginning;
  await vi.waitFor(() => expect(editing.readFailure).toBeInstanceOf(TypeError));
  answers.content = () => ({ content: "x", revision: 7, content_hash: "" });
  await editing.read();
  expect(editing.content?.revision).toBe(7);
  expect(editing.readFailure).toBeUndefined();
  await editing.end();
});

test("a page someone else holds is not read: begin answers who holds it, and the edit is not recorded", async () => {
  const { editing, sent, answers, record, service } = fakeService();
  answers.open = () => {
    throw lockedBy("Bob");
  };

  expect(await editing.begin(false)).toMatchObject({ opened: false, holder: { display_name: "Bob" } });
  expect(sent).toEqual(["OPEN"]);
  expect(service.getPageContent).not.toHaveBeenCalled();
  expect(record.size).toBe(0);
});

test("begin begins once, taking over or not; the edit is recorded while its session is open", async () => {
  const { editing, sent, record } = fakeService();

  const [first, second] = await Promise.all([editing.begin(true), editing.begin(false)]);
  expect(first).toBe(second);
  await vi.waitFor(() => expect(editing.content).toBeDefined());
  expect(sent).toEqual(["OPEN TAKE"]);
  expect([...record]).toEqual([editing]);
  await editing.end();
  expect(record.size).toBe(0);
  expect(sent.at(-1)).toBe("END s1");
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
  await editing.end();
});

test("nothing unsaved, a save sends nothing", async () => {
  const { editing, sent } = await begun();

  expect(await editing.save("# Notes\n", 0)).toBe(true);
  expect(sent).toEqual(["OPEN"]);
  await editing.end();
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
  await editing.end();
});

test("a save whose session ended opens another and goes once more", async () => {
  const { editing, sent, answers } = await begun();
  answers.put = () => {
    answers.put = undefined;
    throw refusal(409, "page.edit_session_ended");
  };

  expect(await editing.save("text", 1)).toBe(true);
  expect(sent).toEqual(["OPEN", 'PUT "text" on 3 in s1', "OPEN", 'PUT "text" on 3 in s2']);
  await editing.end();
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
  await editing.end();
});

test("a 503 server_busy is waited out by its Retry-After, three times at most", async () => {
  const { editing, sent, answers } = await begun();
  vi.useFakeTimers();
  answers.put = () => {
    throw refusal(503, "server_busy", {}, 2);
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
  await editing.end();
});

test("a 503 waited out saves when the server can", async () => {
  const { editing, answers } = await begun();
  vi.useFakeTimers();
  answers.put = () => {
    answers.put = undefined;
    throw refusal(503, "server_busy", {}, 1);
  };

  const saving = editing.save("text", 1);
  await vi.advanceTimersByTimeAsync(1000);
  expect(await saving).toBe(true);
  expect(editing.failure).toBeUndefined();
  await editing.end();
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
  await editing.end();
});

test("the end ends the session, resolving once it is answered; a session opening as it ends is ended too", async () => {
  vi.useFakeTimers();
  const { editing, sent, answers } = await begun();
  let answer: (() => void) | undefined;
  answers.end = () => new Promise((resolve) => (answer = resolve));
  let ended = false;
  void editing.end().then(() => (ended = true));
  await vi.advanceTimersByTimeAsync(0);
  expect(ended).toBe(false);
  answer?.();
  await vi.advanceTimersByTimeAsync(0);
  expect(ended).toBe(true);
  expect(vi.getTimerCount()).toBe(0);
  expect(sent).toEqual(["OPEN", "END s1"]);

  const late = fakeService();
  let open: ((session: { id: string }) => void) | undefined;
  late.answers.open = () => new Promise((resolve) => (open = resolve));
  const beginning = late.editing.begin(false);
  await vi.advanceTimersByTimeAsync(0);
  expect(late.record.size).toBe(1);
  const ending = late.editing.end();
  open?.({ id: "s9" });
  await ending;
  await expect(beginning).rejects.toThrow("The edit has ended.");
  expect(late.sent).toEqual(["OPEN", "END s9"]);
  expect(late.record.size).toBe(0);
  expect(late.service.getPageContent).not.toHaveBeenCalled();
});

test("an edit let go and kept again in the same task, as StrictMode unmounts and mounts its view, keeps its session", async () => {
  vi.useFakeTimers();
  const { editing, sent, record } = await begun();

  editing.keep();
  editing.letGo();
  editing.keep();
  await vi.advanceTimersByTimeAsync(editSessionHeartbeat);
  expect(sent).toEqual(["OPEN", "BEAT s1"]);
  expect(await editing.save("text", 1)).toBe(true);

  editing.letGo();
  expect(sent.at(-1)).toBe('PUT "text" on 3 in s1');
  await vi.advanceTimersByTimeAsync(0);
  expect(sent.at(-1)).toBe("END s1");
  expect(record.size).toBe(0);
  expect(vi.getTimerCount()).toBe(0);
});

test("once the edit ends nothing more is sent: a save asked for then, one waiting for its turn, one waiting out a 503", async () => {
  vi.useFakeTimers();
  const { editing, sent, answers } = await begun();
  let answer: (() => void) | undefined;
  answers.put = () => new Promise((resolve) => (answer = () => resolve({ revision: 4 })));
  const first = editing.save("one", 1);
  const second = editing.save("one two", 2);
  await vi.advanceTimersByTimeAsync(0);
  await editing.end();
  answer?.();
  expect(await first).toBe(true);
  expect(await second).toBe(false);
  expect(await editing.save("one two three", 3)).toBe(false);
  expect(sent).toEqual(["OPEN", 'PUT "one" on 3 in s1', "END s1"]);

  const busy = await begun();
  busy.answers.put = () => {
    throw refusal(503, "server_busy", {}, 30);
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
  answered.answers.put = () => new Promise((_, reject) => (refuse = () => reject(refusal(503, "server_busy", {}, 30))));
  const out = answered.editing.save("text", 1);
  await vi.advanceTimersByTimeAsync(0);
  answered.editing.end();
  refuse?.();
  await vi.advanceTimersByTimeAsync(0);
  expect(vi.getTimerCount()).toBe(0);
  expect(await out).toBe(false);
  expect(answered.editing.busy).toBe(false);
});

test("a save waiting for its lapsed session to open again is not sent once the edit ends; the session is ended as it opens", async () => {
  vi.useFakeTimers();
  const { editing, sent, answers, session } = await begun();
  let open: ((session: { id: string }) => void) | undefined;
  answers.open = () => new Promise((resolve) => (open = resolve));
  session.forget("s1");

  const saving = editing.save("text", 1);
  await vi.advanceTimersByTimeAsync(0);
  const ending = editing.end();
  open?.({ id: "s9" });
  await ending;
  expect(await saving).toBe(false);
  expect(sent).toEqual(["OPEN", "OPEN", "END s9"]);
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
  await editing.end();
});

test("a save that loses the session shows no failure: the edit says why, and saves nothing more", async () => {
  const { editing, sent, answers, session } = await begun();
  answers.put = () => {
    throw refusal(409, "page.edit_session_taken_over");
  };

  editing.changed(1);
  expect(await editing.save("text", 1)).toBe(false);
  expect(session.lost).toEqual({ reason: "taken_over" });
  expect(editing.failure).toBeUndefined();
  expect(editing.saving).toBe(false);
  editing.changed(2);
  expect(await editing.save("text, more", 2)).toBe(false);
  expect(sent.filter((line) => line.startsWith("PUT"))).toHaveLength(1);
  expect(editing.unsaved).toBe(true);
  await editing.end();
  expect(sent.at(-1)).toBe('PUT "text" on 3 in s1');
});

test("a save whose session ended, opened again onto the lock someone took meanwhile, loses the edit to them", async () => {
  const { editing, sent, answers, session } = await begun();
  answers.put = () => {
    throw refusal(409, "page.edit_session_ended");
  };
  answers.open = () => {
    throw lockedBy("Bob");
  };

  expect(await editing.save("text", 1)).toBe(false);
  expect(session.lost).toMatchObject({ reason: "taken", holder: { display_name: "Bob" } });
  expect(editing.failure).toBeUndefined();
  expect(sent).toEqual(["OPEN", 'PUT "text" on 3 in s1', "OPEN"]);
});

/** A response held until the test reaches the other side of a save, end, or unmount. */
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => (resolve = yes));
  return { promise, resolve };
}

/** Notes starts outside the actual nominated source root, Guide. */
function sourceServer(answers: Record<string, HttpAnswer> = {}) {
  return pageServer({ role: "editor", mode: "llm_wiki", sourceRootId: guide.id, nodes: [guide, notes], answers });
}

/** Observe the app's real root and controls; its complete registry supplies the behavior under test. */
async function mountedSourceEdit(server = sourceServer()) {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  const made = vi.spyOn(RootStore.prototype, "editPage");
  let capturedControls: EditorControls | undefined;
  const capture: EditorExtension = {
    name: "observe-source-loss",
    extension(_context, controls) {
      capturedControls = controls;
      return [];
    },
  };
  const rendered = renderApp(pagePath(notes.id), server.app, {
    editorExtensions: [capture, ...editorExtensions],
  });
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  const editor = await pageEditor();
  const root = made.mock.contexts.at(-1);
  if (!(root instanceof RootStore)) {
    throw new Error("the editor did not open through its actual root store");
  }
  const controls = capturedControls;
  const editing = [...root.edits][0];
  const pages = root.pagesOf(server.notebook);
  if (editing === undefined || controls === undefined || pages === undefined) {
    throw new Error("the mounted editor did not expose its actual edit and controls");
  }
  expect(editor.view.state.readOnly).toBe(false);
  expect(editor.content.getAttribute("contenteditable")).toBe("true");
  return { ...rendered, ...editor, user, server, editing, controls, pages };
}

/** Another client moves Notes into source; the existing session's real fake-HTTP PUT now refuses it. */
function enterSource(server: ReturnType<typeof pageServer>) {
  server.nodes = server.nodes.map((node) =>
    node.id === notes.id ? Object.assign({}, node, { parent_id: guide.id }) : node
  );
}

const sourceWhy = "This page is now a source. Its original content is read-only.";
const sourceDraft = "Notes\n my unsaved text";
const sessionWrites = (sent: readonly string[]) => sent.filter((line) => /^(OPEN|PUT|BEAT|END) /.test(line));

test("source loss locks the mounted editor and preserves unsaved text: real registry, copy and return", async () => {
  const { user, server, view, content, type, controls, editing, pages } = await mountedSourceEdit();
  const readonly = vi.spyOn(controls, "setReadOnly");
  act(() => type(" my unsaved text"));
  enterSource(server);

  await act(() => controls.save());

  const banner = await screen.findByRole("alert");
  expect(banner.textContent).toContain(sourceWhy);
  expect(banner.textContent).toContain("Your changes here are not saved: copy them before you leave.");
  expect(readonly).toHaveBeenCalledWith(true);
  expect(view.state.readOnly).toBe(true);
  expect(view.state.facet(EditorView.editable)).toBe(false);
  expect(content.getAttribute("contenteditable")).toBe("false");
  expect((await pageEditor()).view).toBe(view);
  expect(view.state.doc.toString()).toBe(sourceDraft);
  expect(editing.unsaved).toBe(true);
  expect(editing.failure).toBeUndefined();
  expect(editing.saving).toBe(false);
  await act(() => editing.session.end());
  // PageService sends GET edit-lock over pageServer's fake HTTP transport; this is not backend E2E evidence.
  expect(await pages.editLock(notes.id)).toEqual({ holder: null, expires_in: null });

  act(() => {
    view.focus();
    view.dispatch({ selection: { anchor: 0, head: view.state.doc.length } });
  });
  const copied = new Map<string, string>();
  fireEvent.copy(content, { clipboardData: { clearData: () => copied.clear(), setData: copied.set.bind(copied) } });
  expect(copied.get("text/plain")).toBe(sourceDraft);

  await user.click(screen.getByRole("button", { name: "Back to reading" }));
  const first = await screen.findByRole("alertdialog", { name: "Leave without saving?" });
  await user.click(within(first).getByRole("button", { name: "Stay" }));
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect((await pageEditor()).view).toBe(view);
  expect(view.state.doc.toString()).toBe(sourceDraft);
  await user.click(screen.getByRole("button", { name: "Back to reading" }));
  const second = await screen.findByRole("alertdialog", { name: "Leave without saving?" });
  await user.click(within(second).getByRole("button", { name: "Leave" }));
  expect(await screen.findByRole("article", { name: "Notes" })).toBeTruthy();
  expect(sessionWrites(server.sent)).toEqual([
    "OPEN Notes",
    'PUT Notes "Notes\\n my unsaved text" on 1 in session-1',
    "END session-1",
  ]);
});

test("source loss locks the mounted editor and preserves unsaved text: pending autosave stops before end answers", async () => {
  const endAnswer = deferred<void>();
  let server: ReturnType<typeof pageServer>;
  server = sourceServer({
    "DELETE /api/v0/edit-sessions/*": async (request) => {
      const id = new URL(request.url).pathname.split("/").at(-1) ?? "";
      server.sent.push(`END ${id}`);
      await endAnswer.promise;
      server.sessions.delete(id);
      return new Response(null, { status: 204 });
    },
  });
  const { view, type, controls, editing, pages, unmount } = await mountedSourceEdit(server);
  const save = vi.spyOn(controls, "save");
  act(() => type(" my unsaved text"));
  enterSource(server);

  await act(() => controls.save());

  expect(save).toHaveBeenCalledOnce();
  expect(view.state.readOnly).toBe(true);
  expect(editing.session.lost).toEqual({ reason: "source" });
  expect(editing.failure).toBeUndefined();
  let ended = false;
  const ending = editing.session.end().then(() => (ended = true));
  expect(await pages.editLock(notes.id)).toMatchObject({ holder: ada });
  expect(ended).toBe(false);
  const before = sessionWrites(server.sent);
  await act(() => vi.advanceTimersByTimeAsync(Math.max(autosavePause, editSessionHeartbeat) * 2));
  // A downstream no-PUT guard cannot satisfy this: the original controls.save must not be called by the timer.
  expect(save).toHaveBeenCalledOnce();
  expect(sessionWrites(server.sent)).toEqual(before);
  expect(view.state.doc.toString()).toBe(sourceDraft);
  expect(editing.unsaved).toBe(true);

  endAnswer.resolve();
  await act(() => ending);
  expect(ended).toBe(true);
  expect(await pages.editLock(notes.id)).toEqual({ holder: null, expires_in: null });
  act(() => unmount());
  await act(() => vi.advanceTimersByTimeAsync(0));
  await editing.end();
  expect(server.sent.filter((line) => line.startsWith("END "))).toEqual(["END session-1"]);
});

test("source loss locks the mounted editor and preserves unsaved text: failed end still locks immediately", async () => {
  let server: ReturnType<typeof pageServer>;
  server = sourceServer({
    "DELETE /api/v0/edit-sessions/*": (request) => {
      server.sent.push(`END ${new URL(request.url).pathname.split("/").at(-1) ?? ""}`);
      return problem(503, "server_busy");
    },
  });
  const { view, type, controls, editing, pages } = await mountedSourceEdit(server);
  const save = vi.spyOn(controls, "save");
  act(() => type(" my unsaved text"));
  enterSource(server);

  await act(() => controls.save());
  await act(() => editing.session.end());

  expect((await screen.findByRole("alert")).textContent).toContain(sourceWhy);
  expect(view.state.readOnly).toBe(true);
  expect(view.state.facet(EditorView.editable)).toBe(false);
  expect(view.state.doc.toString()).toBe(sourceDraft);
  expect(editing.unsaved).toBe(true);
  expect(editing.failure).toBeUndefined();
  expect(await pages.editLock(notes.id)).toMatchObject({ holder: ada });
  await act(() => vi.advanceTimersByTimeAsync(editSessionHeartbeat * 2));
  expect(save).toHaveBeenCalledOnce();
  await act(() => editing.end());
  expect(sessionWrites(server.sent)).toEqual([
    "OPEN Notes",
    'PUT Notes "Notes\\n my unsaved text" on 1 in session-1',
    "END session-1",
  ]);
});

test.each(["response first", "unmount first"] as const)(
  "source loss locks the mounted editor and preserves unsaved text: %s releases once",
  async (order) => {
    const requested = deferred<void>();
    const response = deferred<Response>();
    let server: ReturnType<typeof pageServer>;
    server = sourceServer({
      "PUT /api/v0/pages/*/content": async (request) => {
        const write = (await request.json()) as { content: string; edit_session_id: string };
        server.sent.push(`PUT Notes ${JSON.stringify(write.content)} in ${write.edit_session_id}`);
        requested.resolve();
        return response.promise;
      },
    });
    const { type, controls, editing, pages, unmount } = await mountedSourceEdit(server);
    act(() => type(" my unsaved text"));
    let saving: Promise<void> | undefined;
    await act(async () => {
      saving = controls.save();
      await requested.promise;
    });
    expect(await pages.editLock(notes.id)).toMatchObject({ holder: ada });
    enterSource(server);

    if (order === "response first") {
      response.resolve(problem(409, "page.source_immutable"));
      await act(async () => saving);
      expect(editing.session.lost).toEqual({ reason: "source" });
      expect(editing.unsaved).toBe(true);
      act(() => unmount());
      await act(() => vi.advanceTimersByTimeAsync(0));
    } else {
      act(() => unmount());
      await act(() => vi.advanceTimersByTimeAsync(0));
      response.resolve(problem(409, "page.source_immutable"));
      await act(async () => saving);
    }

    await editing.end();
    expect(await pages.editLock(notes.id)).toEqual({ holder: null, expires_in: null });
    await act(() => vi.advanceTimersByTimeAsync(editSessionHeartbeat * 2));
    expect(server.sent.filter((line) => line.startsWith("END "))).toEqual(["END session-1"]);
    expect(server.sent.filter((line) => line.startsWith("PUT "))).toHaveLength(1);
    expect(server.sent.filter((line) => line.startsWith("OPEN "))).toEqual(["OPEN Notes"]);
    expect(server.sent.filter((line) => line.startsWith("BEAT "))).toEqual([]);
  }
);

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
  await editing.end();
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
  await editing.end();
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
  await editing.end();
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
  await editing.end();
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
  await editing.end();
});
