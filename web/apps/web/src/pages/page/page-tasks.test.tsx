import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, onTestFinished, test, vi } from "vitest";

import { eventHandlers } from "../../events/handlers";
import { FakePage } from "../../events/testing/fake-page";
import { readingEnhancements, type Enhancement } from "../../reading/enhancement";
import { eventServer, withEvents } from "../../test/event-server";
import { json, notebookJSON, problem, upgradeOf, workspaceJSON } from "../../test/fakes";
import { bob, install, notes, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";
import { lockNote } from "../../test/lock-note";

// Ticking a task item in the reading view (M5/P6 design 3.5), through the
// app's enhancements as the composition root gives them.

beforeEach(() => {
  // The view's scrolling enhancement watches widths, which jsdom does not have.
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    }
  );
});
afterEach(() => vi.unstubAllGlobals());

const tasks = "- [ ] a\n- [x] b\n";

/**
 * focusFixup applies HTML's focus fixup rule at once, as an engine may and
 * jsdom does not: a focused element disabled loses the focus (Chromium
 * applies it at the next rendering, after the new HTML is in). First, it
 * is undone last, after the checkboxes are disabled again. jsdom blurs
 * only what can have the focus.
 */
const focusFixup: Enhancement = () => () => {
  const focused = document.activeElement;
  if (focused instanceof HTMLInputElement && focused.disabled) {
    focused.disabled = false;
    focused.blur();
    focused.disabled = true;
  }
};

/** boxes are the reading view's checkboxes, in order. */
const boxes = () => screen.getAllByRole<HTMLInputElement>("checkbox");

/** opened renders Install with content's task items as server, else role's, shows them, through the app's enhancements. */
async function opened(
  role: "editor" | "reader" = "editor",
  content = tasks,
  server: ReturnType<typeof pageServer> = pageServer({ role })
) {
  server.withTasks(install.id, content);
  const { router } = renderApp(pagePath(install.id), server.app, {
    enhancements: [focusFixup, ...readingEnhancements],
  });
  await waitFor(() => expect(boxes()).toHaveLength(content.split("\n").length - 1));
  return { server, router };
}

test("a writer ticks an item, named by its text: the toggle on the view's revision, the view read again, the focus on the same checkbox, without a scroll", async () => {
  const { server } = await opened();
  const user = userEvent.setup();
  const focus = vi.spyOn(HTMLElement.prototype, "focus");

  await user.click(screen.getByRole("checkbox", { name: "a" }));

  await waitFor(() => expect(boxes()[0]?.checked).toBe(true));
  expect(server.sent.filter((line) => line.startsWith("TOGGLE") || line === "GET view Install")).toEqual([
    "GET view Install",
    "TOGGLE Install 3 true on 1",
    "GET view Install",
  ]);
  expect(server.contents.get(install.id)).toEqual({ content: "- [x] a\n- [x] b\n", revision: 2 });
  expect(document.activeElement).toBe(boxes()[0]);
  expect(focus).toHaveBeenLastCalledWith({ preventScroll: true });
  focus.mockRestore();

  await user.keyboard(" ");

  await waitFor(() => expect(boxes()[0]?.checked).toBe(false));
  expect(server.contents.get(install.id)).toEqual({ content: tasks, revision: 3 });
  expect(document.activeElement).toBe(boxes()[0]);
});

/**
 * folded is a view at revision whose one task item, of text, is in a folded callout titled by the revision, before
 * another folded callout.
 */
function folded(revision: number, text = "a") {
  return {
    html: `<details class="nw-callout"><summary>Later ${revision.toString()}</summary><ul>\n<li><input disabled="" type="checkbox" data-task="3"> ${text}</li>\n</ul></details>\n<details class="nw-callout"><summary>Other</summary><p>x</p></details>\n`,
    revision,
    assets_expire_at: null,
  };
}

/** callouts are whether the reading view's callouts are open, in order. */
const callouts = () => [...screen.getByRole("article").querySelectorAll("details")].map((each) => each.open);

test("an item of a folded callout the reader opened has the focus back in the view read again, its callout opened first; another item, not", async () => {
  const server = pageServer();
  server.views.set(install.id, folded(1));
  const reloads: (() => void)[] = [];
  renderApp(pagePath(install.id), server.app, {
    enhancements: [
      (_container, context) => {
        reloads.push(context.reload);
        return undefined;
      },
      focusFixup,
      ...readingEnhancements,
    ],
  });
  await waitFor(() => expect(boxes()[0]?.disabled).toBe(false));
  const first = screen.getByRole("article").querySelector("details") as HTMLDetailsElement;
  first.open = true;
  boxes()[0]?.focus();
  // Whether its callout is open as each element takes the focus: an engine gives none to one in a closed details.
  const openAsFocused: (boolean | undefined)[] = [];
  const focus = HTMLElement.prototype.focus;
  const focusing = vi.spyOn(HTMLElement.prototype, "focus").mockImplementation(function (this: HTMLElement, options) {
    openAsFocused.push(this.closest("details")?.open);
    focus.call(this, options);
  });
  onTestFinished(() => focusing.mockRestore());

  server.views.set(install.id, folded(2));
  act(() => reloads.at(-1)?.());
  await screen.findByText("Later 2");
  expect(callouts()).toEqual([true, false]);
  expect(document.activeElement).toBe(boxes()[0]);
  expect(openAsFocused).toContain(true);
  expect(openAsFocused).not.toContain(false);

  // The item at its position is another: its callout stays as the view has it.
  server.views.set(install.id, folded(3, "b"));
  act(() => reloads.at(-1)?.());
  await screen.findByText("Later 3");
  expect(callouts()).toEqual([false, false]);
  expect(document.activeElement).toBe(document.body);
});

test("a toggle on a revision passed reads the view again and says the page changed; the focus does not go to the item that moved to its place; the next toggle clears the refusal", async () => {
  const { server } = await opened("editor", "- [ ] a\n- [ ] b\n");
  server.withTasks(install.id, "- [ ] z\n- [ ] a\n- [ ] b\n", 2);

  await userEvent.click(screen.getByRole("checkbox", { name: "b" }));

  expect((await screen.findByRole("alert")).textContent).toBe("This page has changed since you read it.");
  await waitFor(() => expect(boxes()).toHaveLength(3));
  expect(document.activeElement).toBe(document.body);
  expect(server.contents.get(install.id)).toEqual({ content: "- [ ] z\n- [ ] a\n- [ ] b\n", revision: 2 });

  await userEvent.click(screen.getByRole("checkbox", { name: "b" }));

  await waitFor(() => expect(screen.getByRole<HTMLInputElement>("checkbox", { name: "b" }).checked).toBe(true));
  expect(screen.queryByRole("alert")).toBeNull();
});

test("a toggle while someone edits the page is refused as Edit is: the note reads who and takes the focus, no alert, the checkbox as it was", async () => {
  const { server } = await opened();
  server.hold(install.id, bob);

  await userEvent.click(boxes()[0] as HTMLElement);

  await waitFor(() => expect(document.activeElement?.textContent).toContain("Bob is editing this page."));
  expect(screen.queryByRole("alert")).toBeNull();
  expect(boxes()[0]?.checked).toBe(false);
  expect(server.contents.get(install.id)).toEqual({ content: tasks, revision: 1 });
});

test("one toggle of the page is out at a time, though the page is left and opened again meanwhile", async () => {
  let release: (() => void) | undefined;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  const posted: unknown[] = [];
  const server = pageServer({
    role: "editor",
    answers: {
      "POST /api/v0/pages/*/toggle-task": async (request) => {
        posted.push(await request.clone().json());
        await held;
        return problem(409, "page.revision_mismatch");
      },
    },
  });
  const { router } = await opened("editor", tasks, server);

  await userEvent.click(boxes()[0] as HTMLElement);
  await waitFor(() => expect(posted).toHaveLength(1));
  await act(() => router.navigate(pagePath(notes.id)));
  await act(() => router.navigate(pagePath(install.id)));
  await waitFor(() => expect(boxes()).toHaveLength(2));
  await userEvent.click(boxes()[1] as HTMLElement);
  expect(posted).toHaveLength(1);

  release?.();
  await waitFor(async () => {
    await userEvent.click(boxes()[1] as HTMLElement);
    expect(posted).toHaveLength(2);
  });
});

test.each([
  ["en", "This task cannot be ticked or cleared here: its line would no longer be a task. Edit the page to change it."],
  ["zh-CN", "这一项不能在这里勾选或取消勾选：改了之后它那一行就不再是任务项。请编辑页面来修改。"],
] as const)(
  "a tick that would leave no task item says so above the view, in %s; the checkbox as it was, nothing written",
  async (locale, said) => {
    const server = pageServer({ role: "editor" });
    server.app.preferences.setLocale(locale);
    await opened("editor", "- [ ]: /u\n- [ ] b\n", server);
    const user = userEvent.setup();

    await user.click(boxes()[0] as HTMLInputElement);

    expect((await screen.findByRole("alert")).textContent).toBe(said);
    expect(boxes()[0]?.checked).toBe(false);
    expect(server.contents.get(install.id)).toEqual({ content: "- [ ]: /u\n- [ ] b\n", revision: 1 });
    expect(server.sent.filter((line) => line.startsWith("TOGGLE"))).toEqual(["TOGGLE Install 3 true on 1"]);
  }
);

/** demoted is Install's server for an editor whose role the notebook's admin took away: its write answers 403. */
function demoted(write: string) {
  let notebookReads = 0;
  const server: ReturnType<typeof pageServer> = pageServer({
    role: "editor",
    answers: {
      "GET /api/v0/workspaces/lab/notebooks": () => {
        notebookReads += 1;
        return json({ data: [server.notebook] });
      },
      [write]: () => {
        server.notebook = { ...server.notebook, role: "reader" };
        return problem(403, "forbidden");
      },
    },
  });
  return { server, notebookReads: () => notebookReads };
}

// M12/P2/S4, 6.2: a role taken away elsewhere shows as a toggle is refused: the notebooks are read again, and the
// checkboxes follow the role, the view not written again (U7).
test("a toggle refused as forbidden reads the notebooks again, and with the role gone the checkboxes are disabled, the view not written again", async () => {
  const { server, notebookReads } = demoted("POST /api/v0/pages/*/toggle-task");
  await opened("editor", tasks, server);
  const article = screen.getByRole("article", { name: "Install" });
  const first = article.firstChild;
  const [a, b] = boxes();
  const reads = notebookReads();

  await userEvent.click(a as HTMLElement);

  expect((await screen.findByRole("alert")).textContent).toBe("You do not have permission to do this.");
  await waitFor(() => expect(boxes().map((box) => box.disabled)).toEqual([true, true]));
  expect(notebookReads()).toBeGreaterThan(reads);
  expect(article.firstChild).toBe(first);
  const [aNow, bNow] = boxes();
  expect(aNow).toBe(a);
  expect(bNow).toBe(b);
  // The checkbox the reader was at is disabled: the page's title has the focus (review B's M1).
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Install" })));
});

// M12/P2/S4, 6.2 (P2-23): Edit's refusal is the same root, the same remedy.
test("Edit refused as forbidden reads the notebooks again: with the role gone, Edit goes", async () => {
  const { server, notebookReads } = demoted("POST /api/v0/pages/*/edit-sessions");
  await opened("editor", tasks, server);
  const reads = notebookReads();

  await userEvent.click(screen.getByRole("button", { name: "Edit" }));

  expect((await screen.findByRole("alert")).textContent).toBe("You do not have permission to do this.");
  await waitFor(() => expect(screen.queryByRole("button", { name: "Edit" })).toBeNull());
  expect(notebookReads()).toBeGreaterThan(reads);
  expect(boxes().map((box) => box.disabled)).toEqual([true, true]);
  // Edit, which had the focus, is gone: the page's title has it (review B's M1).
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Install" })));
});

/** streamed is Install's task items open as an editor's, the stream connected; a toggle answers as toggled does. */
async function streamed(toggled: () => Response | Promise<Response>) {
  const events = eventServer();
  let notebookReads = 0;
  const server: ReturnType<typeof pageServer> = pageServer({
    role: "editor",
    answers: {
      "GET /api/v0/events": events.answer,
      "GET /api/v0/workspaces/lab/notebooks": () => {
        notebookReads += 1;
        return json({ data: [server.notebook] });
      },
      "POST /api/v0/pages/*/toggle-task": toggled,
    },
  });
  server.withTasks(install.id, tasks);
  renderApp(pagePath(install.id), withEvents(server.app), {
    enhancements: [focusFixup, ...readingEnhancements],
    eventHandlers,
  });
  await waitFor(() => expect(boxes()).toHaveLength(2));
  await waitFor(() => expect(boxes()[0]?.disabled).toBe(false));
  await waitFor(() => expect(events.streams).toHaveLength(1));
  const stream = events.last();
  act(() => stream.hello());
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  // A mode event reads the notebooks again, the role and the upgrade with them.
  const modeChanged = () =>
    act(() => stream.send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id }));
  return { server, modeChanged, notebookReads: () => notebookReads };
}

// M12/P2/S4, 6.2 (review B's C1-M1): the notebooks read again keep the role (or the read fails): the wait ends with
// the answer, and a later change of whether the page can be written, an upgrade starting, moves no focus.
test("a toggle refused as forbidden whose role the notebooks read again keep leaves the focus as an upgrade starts later", async () => {
  const { server, modeChanged, notebookReads } = await streamed(() => problem(403, "forbidden"));
  const reads = notebookReads();
  await userEvent.click(boxes()[0] as HTMLElement);
  expect((await screen.findByRole("alert")).textContent).toBe("You do not have permission to do this.");
  await waitFor(() => expect(notebookReads()).toBeGreaterThan(reads));
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  expect(boxes()[0]?.disabled).toBe(false);
  // The reader reads on: the focus is nowhere, as after a click on the text.
  (document.activeElement as HTMLElement | null)?.blur();

  server.notebook = { ...server.notebook, upgrade: upgradeOf() };
  modeChanged();
  await waitFor(() => expect(boxes()[0]?.disabled).toBe(true));
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));

  expect(document.activeElement).toBe(document.body);
});

// M12/P2/S4, 6.2: the role taken by an event first, the checkbox disabled as its toggle was under way, no render is
// to come of the read the refusal brings: the focus the disabled checkbox has goes to the page's title at its answer.
test("a toggle refused as forbidden after an event took the role gives the page's title the focus the disabled checkbox had", async () => {
  let answer: (() => void) | undefined;
  const held = new Promise<void>((resolve) => {
    answer = resolve;
  });
  const { server, modeChanged } = await streamed(async () => {
    await held;
    return problem(403, "forbidden");
  });
  const box = boxes()[0] as HTMLElement;
  await userEvent.click(box);
  server.notebook = { ...server.notebook, role: "reader" };
  modeChanged();
  await waitFor(() => expect(boxes()[0]?.disabled).toBe(true));
  expect(document.activeElement).toBe(box);

  answer?.();

  expect((await screen.findByRole("alert")).textContent).toBe("You do not have permission to do this.");
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Install" })));
});

test("a reader's checkboxes stay disabled", async () => {
  const { server } = await opened("reader");

  await userEvent.click(boxes()[0] as HTMLElement);

  expect(boxes().map((box) => box.disabled)).toEqual([true, true]);
  expect(server.sent.filter((line) => line.startsWith("TOGGLE"))).toEqual([]);
});

test("the focus does not go to another item of the same text that a write moved to its place", async () => {
  const { server } = await opened("editor", "- [ ] a\n- [x] a\n");
  server.withTasks(install.id, "- [ ] a\n- [ ] a\n- [x] a\n", 2);
  (boxes()[1] as HTMLElement).focus();

  await userEvent.keyboard(" ");

  await screen.findByRole("alert");
  await waitFor(() => expect(boxes()).toHaveLength(3));
  expect(document.activeElement).toBe(document.body);
});

test("a lock refusal does not take the focus from where the user went meanwhile", async () => {
  let reads = 0;
  let release: (() => void) | undefined;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  const server = pageServer({
    role: "editor",
    answers: {
      "GET /api/v0/pages/*/edit-lock": async () => {
        reads += 1;
        if (reads > 1) {
          await held;
        }
        return json(server.lockOf(install.id));
      },
    },
  });
  await opened("editor", tasks, server);
  server.hold(install.id, bob);

  await userEvent.click(boxes()[0] as HTMLElement);
  await waitFor(() => expect(reads).toBe(2));
  const edit = screen.getByRole("button", { name: "Edit" });
  edit.focus();
  release?.();

  await screen.findByText(lockNote("Bob is editing this page."));
  expect(document.activeElement).toBe(edit);
});

test("a toggle's refusal that comes while the page is edited is not shown once the edit is done", async () => {
  let release: (() => void) | undefined;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  const server = pageServer({
    role: "editor",
    answers: {
      "POST /api/v0/pages/*/toggle-task": async () => {
        await held;
        return problem(409, "page.revision_mismatch");
      },
    },
  });
  await opened("editor", tasks, server);
  const user = userEvent.setup();

  await user.click(boxes()[0] as HTMLElement);
  await user.click(screen.getByRole("button", { name: "Edit" }));
  const done = await screen.findByRole("button", { name: "Done" });
  // The refusal reaches the page while it edits: the view is gone, nothing is read again.
  release?.();
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  await user.click(done);

  await screen.findByRole("button", { name: "Edit" });
  expect(screen.queryByRole("alert")).toBeNull();
});

test("the focus stays on the item ticked when the event of the toggle reads the view again before the toggle's answer comes", async () => {
  const events = eventServer();
  let release: (() => void) | undefined;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  let posted = false;
  const server = pageServer({
    role: "editor",
    answers: {
      "GET /api/v0/events": events.answer,
      // The toggle is written, its event sent, and its answer comes late.
      "POST /api/v0/pages/*/toggle-task": async () => {
        server.withTasks(install.id, "- [x] a\n- [x] b\n", 2);
        posted = true;
        await held;
        return json({
          ...install,
          ancestors: [],
          revision: 2,
          byte_size: 0,
          content_updated_at: install.updated_at,
          content_updated_by: "",
        });
      },
    },
  });
  server.withTasks(install.id, tasks);
  renderApp(pagePath(install.id), withEvents(server.app, new FakePage()), {
    enhancements: [focusFixup, ...readingEnhancements],
    eventHandlers,
  });
  await waitFor(() => expect(boxes()).toHaveLength(2));
  await waitFor(() => expect(events.streams).toHaveLength(1));
  act(() => events.last().hello());
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  (boxes()[0] as HTMLElement).focus();

  await userEvent.keyboard(" ");
  await waitFor(() => expect(posted).toBe(true));
  act(() =>
    events.last().send("pages", {
      workspace_id: workspaceJSON.id,
      notebook_id: notebookJSON.id,
      tree: false,
      pages: [{ id: install.id, revision: 2 }],
    })
  );

  await waitFor(() => expect(boxes()[0]?.checked).toBe(true));
  expect(document.activeElement).toBe(boxes()[0]);
  release?.();
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  expect(document.activeElement).toBe(boxes()[0]);
});

test("the focus stays on the item ticked when another's write, read again for its event, comes before the toggle's 409", async () => {
  const events = eventServer();
  let release: (() => void) | undefined;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  let posted = false;
  const server = pageServer({
    role: "editor",
    answers: {
      "GET /api/v0/events": events.answer,
      // Another writes below the items meanwhile: the toggle, on revision 1, is refused late.
      "POST /api/v0/pages/*/toggle-task": async () => {
        server.withTasks(install.id, `${tasks}- [ ] c\n`, 2);
        posted = true;
        await held;
        return problem(409, "page.revision_mismatch");
      },
    },
  });
  server.withTasks(install.id, tasks);
  renderApp(pagePath(install.id), withEvents(server.app, new FakePage()), {
    enhancements: [focusFixup, ...readingEnhancements],
    eventHandlers,
  });
  await waitFor(() => expect(boxes()).toHaveLength(2));
  await waitFor(() => expect(events.streams).toHaveLength(1));
  act(() => events.last().hello());
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  (boxes()[0] as HTMLElement).focus();

  await userEvent.keyboard(" ");
  await waitFor(() => expect(posted).toBe(true));
  act(() =>
    events.last().send("pages", {
      workspace_id: workspaceJSON.id,
      notebook_id: notebookJSON.id,
      tree: false,
      pages: [{ id: install.id, revision: 2 }],
    })
  );

  await waitFor(() => expect(boxes()).toHaveLength(3));
  expect(document.activeElement).toBe(boxes()[0]);
  release?.();
  expect((await screen.findByRole("alert")).textContent).toBe("This page has changed since you read it.");
  expect(boxes()[0]?.checked).toBe(false);
  expect(document.activeElement).toBe(boxes()[0]);
});
