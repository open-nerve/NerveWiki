import userEvent from "@testing-library/user-event";
import { afterEach, expect, test } from "vitest";

import type { NotebookRole } from "../services/notebook.service";
import type { ReadingContext } from "./enhancement";
import { taskToggle } from "./task-toggle";
import { translator } from "../i18n/i18n";

// The task items' enhancement (M5/P6 design 3.5).

afterEach(() => document.body.replaceChildren());

/** A deferred toggle: the test settles it. */
type Toggle = { offset: number; checked: boolean; settle: (error?: unknown) => void };

/** setUp puts a view of two task items, b done, in the page, and a context that records the toggles (a writer's) and reports. */
function setUp(role: NotebookRole = "editor") {
  const container = document.createElement("article");
  container.innerHTML =
    '<ul><li><input disabled="" type="checkbox" data-task="3"> a</li>' +
    '<li><input checked="" disabled="" type="checkbox" data-task="11"> b</li></ul><p><span>text</span></p>';
  document.body.append(container);
  const toggles: Toggle[] = [];
  const reports: unknown[] = [];
  const context: ReadingContext = {
    workspace: "lab",
    notebook: "n",
    page: "p",
    revision: 4,
    role,
    t: translator("en"),
    theme: "light",
    reload: () => undefined,
    navigate: () => undefined,
    toggleTask:
      role === "reader"
        ? undefined
        : (offset, checked) =>
            new Promise((resolve, reject) => {
              toggles.push({ offset, checked, settle: (error) => (error === undefined ? resolve() : reject(error)) });
            }),
    report: (error) => reports.push(error),
    unresolved: () => undefined,
  };
  const [a, b] = container.querySelectorAll<HTMLInputElement>("input");
  if (a === undefined || b === undefined) {
    throw new Error("no checkboxes");
  }
  return { container, context, toggles, reports, a, b };
}

/** settled waits for the promises a toggle settled to run on. */
const settled = () => new Promise((resolve) => setTimeout(resolve, 0));

test("a writer's checkboxes are enabled, each named by its item; a click asks for the other state and leaves the box as the server has it", async () => {
  const { container, context, toggles, a, b } = setUp();
  taskToggle(container, context);
  expect([a.disabled, b.disabled]).toEqual([false, false]);
  expect([a.getAttribute("aria-label"), b.getAttribute("aria-label")]).toEqual(["a", "b"]);

  await userEvent.click(a);
  await userEvent.click(b);

  expect(toggles.map(({ offset, checked }) => [offset, checked])).toEqual([
    [3, true],
    [11, false],
  ]);
  expect([a.checked, b.checked]).toEqual([false, true]);
});

test("an item's name is its own text, in a paragraph or a heading, up to its next block (a sublist, code, a quote); an item with no text has no name", () => {
  const container = document.createElement("article");
  container.innerHTML =
    '<ul><li><input disabled="" type="checkbox" data-task="3"> parent  <em>item</em>\n' +
    '<ul><li><input disabled="" type="checkbox" data-task="20"> child</li></ul></li>' +
    '<li><p><input disabled="" type="checkbox" data-task="40"> loose</p><p>more</p></li>' +
    '<li><h1><input disabled="" type="checkbox" data-task="60"> heading</h1></li>' +
    '<li><input disabled="" type="checkbox" data-task="80"> </li>' +
    '<li><input disabled="" type="checkbox" data-task="90"> step\n<pre><code>run it</code></pre></li>' +
    '<li><input disabled="" type="checkbox" data-task="99"> said\n<blockquote><p>quoted</p></blockquote></li></ul>';
  const { context } = setUp();

  taskToggle(container, context);

  expect([...container.querySelectorAll("input")].map((box) => box.getAttribute("aria-label"))).toEqual([
    "parent item",
    "child",
    "loose",
    "heading",
    null,
    "step",
    "said",
  ]);
});

test("the space key toggles the focused item", async () => {
  const { container, context, toggles, b } = setUp();
  taskToggle(container, context);

  b.focus();
  await userEvent.keyboard(" ");

  expect(toggles.map(({ offset, checked }) => [offset, checked])).toEqual([[11, false]]);
  expect(b.checked).toBe(true);
});

test("a toggle refused is reported, and the next click asks again", async () => {
  const { container, context, toggles, reports, a } = setUp();
  taskToggle(container, context);
  const refusal = new Error("refused");

  await userEvent.click(a);
  toggles[0]?.settle(refusal);
  await settled();

  expect(reports).toEqual([refusal]);
  await userEvent.click(a);
  expect(toggles).toHaveLength(2);
});

test("without a toggle, a reader's, the checkboxes stay disabled, and a click asks nothing", async () => {
  const { container, context, toggles, a, b } = setUp("reader");
  expect(taskToggle(container, context)).toBeUndefined();

  a.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true }));

  expect([a.disabled, b.disabled]).toEqual([true, true]);
  expect(toggles).toEqual([]);
});

test("a click elsewhere in the view is the view's", async () => {
  const { container, context, toggles } = setUp();
  taskToggle(container, context);
  const click = new MouseEvent("click", { bubbles: true, cancelable: true });

  container.querySelector("span")?.dispatchEvent(click);

  expect(click.defaultPrevented).toBe(false);
  expect(toggles).toEqual([]);
});

test("undone, the checkboxes are disabled again, without their names, and a click asks nothing", async () => {
  const { container, context, toggles, a, b } = setUp();
  const undo = taskToggle(container, context);

  undo?.();
  a.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true }));

  expect([a.disabled, b.disabled]).toEqual([true, true]);
  expect([a.hasAttribute("aria-label"), b.hasAttribute("aria-label")]).toEqual([false, false]);
  expect(toggles).toEqual([]);
});
