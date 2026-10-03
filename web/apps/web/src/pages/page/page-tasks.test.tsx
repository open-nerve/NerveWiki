import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, test, vi } from "vitest";

import { readingEnhancements, type Enhancement } from "../../reading/enhancement";
import { bob, install, pagePath, pageServer } from "../../test/page-server";
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

/** opened renders Install with the task items as role sees it, through the app's enhancements. */
async function opened(role: "editor" | "reader" = "editor") {
  const server = pageServer({ role });
  server.withTasks(install.id, tasks);
  renderApp(pagePath(install.id), server.app, { enhancements: [focusFixup, ...readingEnhancements] });
  await waitFor(() => expect(boxes()).toHaveLength(2));
  return server;
}

test("a writer ticks an item: the toggle on the view's revision, the view read again, the focus on the same checkbox", async () => {
  const server = await opened();
  const user = userEvent.setup();

  await user.click(boxes()[0] as HTMLElement);

  await waitFor(() => expect(boxes()[0]?.checked).toBe(true));
  expect(server.sent.filter((line) => line.startsWith("TOGGLE") || line === "GET view Install")).toEqual([
    "GET view Install",
    "TOGGLE Install 3 true on 1",
    "GET view Install",
  ]);
  expect(server.contents.get(install.id)).toEqual({ content: "- [x] a\n- [x] b\n", revision: 2 });
  expect(document.activeElement).toBe(boxes()[0]);

  await user.keyboard(" ");

  await waitFor(() => expect(boxes()[0]?.checked).toBe(false));
  expect(server.contents.get(install.id)).toEqual({ content: tasks, revision: 3 });
  expect(document.activeElement).toBe(boxes()[0]);
});

test("a toggle on a revision passed reads the view again and says the page changed", async () => {
  const server = await opened();
  server.withTasks(install.id, "- [ ] a\n- [ ] b\n", 2);

  await userEvent.click(boxes()[1] as HTMLElement);

  expect((await screen.findByRole("alert")).textContent).toBe("This page has changed since you read it.");
  await waitFor(() => expect(boxes()[1]?.checked).toBe(false));
  expect(server.contents.get(install.id)).toEqual({ content: "- [ ] a\n- [ ] b\n", revision: 2 });
});

test("a toggle while someone edits the page is refused: the note reads who, the checkbox stays; the next toggle clears the refusal", async () => {
  const server = await opened();
  const session = server.hold(install.id, bob);

  await userEvent.click(boxes()[0] as HTMLElement);

  expect((await screen.findByRole("alert")).textContent).toBe("This page is being edited in another session.");
  await screen.findByText("Bob is editing this page.");
  expect(boxes()[0]?.checked).toBe(false);
  expect(server.contents.get(install.id)).toEqual({ content: tasks, revision: 1 });

  server.sessions.delete(session);
  await userEvent.click(boxes()[0] as HTMLElement);

  await waitFor(() => expect(boxes()[0]?.checked).toBe(true));
  expect(screen.queryByRole("alert")).toBeNull();
});

test("a reader's checkboxes stay disabled", async () => {
  const server = await opened("reader");

  await userEvent.click(boxes()[0] as HTMLElement);

  expect(boxes().map((box) => box.disabled)).toEqual([true, true]);
  expect(server.sent.filter((line) => line.startsWith("TOGGLE"))).toEqual([]);
});
