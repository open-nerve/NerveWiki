import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, test, vi } from "vitest";

import { eventHandlers } from "../../events/handlers";
import { FakePage } from "../../events/testing/fake-page";
import { readingEnhancements, type Enhancement } from "../../reading/enhancement";
import { eventServer, withEvents } from "../../test/event-server";
import { json, notebookJSON, problem, workspaceJSON } from "../../test/fakes";
import { bob, install, notes, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

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

  await screen.findByText("Bob is editing this page.");
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
