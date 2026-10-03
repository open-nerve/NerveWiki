import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { notebookJSON } from "../../test/fakes";
import { guide, notes, pageNode, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The quick switch (M4/P5 design 3.10). jsdom's platform is not macOS: Mod is Ctrl.

afterEach(() => vi.useRealTimers());

const home = `/lab/notebooks/${notebookJSON.id}`;

/** Presses Ctrl+O on the page; tells whether the browser's own action was let be. */
const ctrlO = () => fireEvent.keyDown(document.body, { key: "o", ctrlKey: true });

/** The options listed, each as its title and its ancestors. */
const listed = () =>
  within(screen.getByRole("listbox", { name: "Pages" }))
    .getAllByRole("option")
    .map((option) => [...option.querySelectorAll("span")].map((span) => span.textContent));

async function opened() {
  await screen.findByRole("heading", { level: 1, name: "Plans" });
  expect(ctrlO()).toBe(false);
  return screen.findByRole("dialog", { name: "Go to a page" });
}

test("Ctrl+O opens it instead of the browser's Open File; it finds by title, Down and Enter go to the page, arrived at", async () => {
  const user = userEvent.setup();
  renderApp(home, pageServer().app);
  // Opened from the tree: going to a page leaves the focus on its heading, not there.
  const nav = await screen.findByRole("navigation", { name: "Pages of Plans" });
  (await within(nav).findByRole("link", { name: "Notes" })).focus();
  const dialog = await opened();
  const field = within(dialog).getByRole("combobox", { name: "Page title" });
  expect(document.activeElement).toBe(field);
  expect(listed()).toEqual([
    ["Guide", ""],
    ["Install", "Guide"],
    ["Linux", "Guide / Install"],
    ["Notes", ""],
  ]);

  // What is typed starts the list anew, its first option active.
  await user.keyboard("{ArrowDown}{ArrowDown}");
  await user.type(field, "IN");
  expect(listed()).toEqual([
    ["Install", "Guide"],
    ["Linux", "Guide / Install"],
  ]);
  const [install, linux] = within(dialog).getAllByRole("option");
  expect(field.getAttribute("aria-activedescendant")).toBe(install?.id);
  await user.keyboard("{ArrowUp}");
  expect(field.getAttribute("aria-activedescendant")).toBe(install?.id);
  await user.keyboard("{ArrowDown}{ArrowDown}");
  expect(field.getAttribute("aria-activedescendant")).toBe(linux?.id);
  expect(linux?.getAttribute("aria-selected")).toBe("true");
  await user.keyboard("{Enter}");

  const heading = await screen.findByRole("heading", { level: 1, name: "Linux" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
  expect(screen.queryByRole("dialog")).toBeNull();
});

test("a page is chosen by a click too; Esc closes without going, the focus back where it was", async () => {
  const user = userEvent.setup();
  renderApp(pagePath(guide.id), pageServer().app);
  await screen.findByRole("heading", { level: 1, name: "Guide" });
  const link = await within(screen.getByRole("navigation", { name: "Pages of Plans" })).findByRole("link", {
    name: "Notes",
  });
  link.focus();

  expect(ctrlO()).toBe(false);
  await screen.findByRole("dialog", { name: "Go to a page" });
  await user.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(screen.getByRole("heading", { level: 1, name: "Guide" })).toBeTruthy();
  await waitFor(() => expect(document.activeElement).toBe(link));

  ctrlO();
  const dialog = await screen.findByRole("dialog", { name: "Go to a page" });
  await user.click(within(dialog).getByRole("option", { name: /^Notes/ }));
  expect(await screen.findByRole("heading", { level: 1, name: "Notes" })).toBeTruthy();
});

test("no page found says so; past 50 the first are listed, and it says so", async () => {
  const user = userEvent.setup();
  const many = Array.from({ length: 55 }, (_, i) => pageNode(i + 1, `Page ${i + 1}`));
  renderApp(home, pageServer({ nodes: many }).app);
  const dialog = await opened();

  expect(within(dialog).getAllByRole("option")).toHaveLength(50);
  expect(within(dialog).getByText("The first 50 are shown: type more to find the others.")).toBeTruthy();
  await user.type(within(dialog).getByRole("combobox"), "page 5");
  expect(listed().map(([title]) => title)).toEqual([
    "Page 5",
    "Page 50",
    "Page 51",
    "Page 52",
    "Page 53",
    "Page 54",
    "Page 55",
  ]);
  expect(within(dialog).queryByText(/^The first/)).toBeNull();

  await user.type(within(dialog).getByRole("combobox"), "x");
  expect(within(dialog).getByRole("status").textContent).toBe("No page's title has that in it.");
  expect(within(dialog).queryByRole("listbox")).toBeNull();
  expect(within(dialog).getByRole("combobox").hasAttribute("aria-controls")).toBe(false);
});

test("until the tree is read it says why, and Try again reads it", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  server.nodesDown = true;
  renderApp(home, server.app);

  const dialog = await opened();
  expect(await within(dialog).findByRole("alert")).toBeTruthy();
  server.nodesDown = false;
  await user.click(within(dialog).getByRole("button", { name: "Try again" }));

  expect(await within(dialog).findByRole("listbox", { name: "Pages" })).toBeTruthy();
});

test("Mod with another modifier, or Cmd off macOS, opens nothing", async () => {
  renderApp(home, pageServer().app);
  await screen.findByRole("heading", { level: 1, name: "Plans" });

  expect(fireEvent.keyDown(document.body, { key: "o", ctrlKey: true, shiftKey: true })).toBe(true);
  expect(fireEvent.keyDown(document.body, { key: "o", metaKey: true })).toBe(true);
  expect(screen.queryByRole("dialog")).toBeNull();
});

test("Ctrl+O outside a notebook is the browser's", async () => {
  renderApp("/lab", pageServer().app);
  await screen.findByRole("heading", { level: 1 });

  expect(ctrlO()).toBe(true);
  expect(screen.queryByRole("dialog")).toBeNull();
});

test("Ctrl+O does not open it over another dialog", async () => {
  const user = userEvent.setup();
  renderApp(home, pageServer().app);
  const nav = await screen.findByRole("navigation", { name: "Pages of Plans" });
  await user.click(await within(nav).findByRole("button", { name: "Actions for Notes" }));
  await user.click(await screen.findByRole("menuitem", { name: "Rename" }));
  await screen.findByRole("dialog", { name: "Rename Notes" });

  expect(ctrlO()).toBe(false);
  expect(screen.queryByRole("dialog", { name: "Go to a page" })).toBeNull();
});

test("the tree read again while it is open, Enter goes to the option it can still name", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
  const server = pageServer();
  renderApp(home, server.app);
  await opened();
  await user.keyboard("{ArrowDown}{ArrowDown}{ArrowDown}");
  expect(listed()).toHaveLength(4);

  server.nodes = [guide, notes];
  await act(() => vi.advanceTimersByTimeAsync(6_000));
  act(() => void window.dispatchEvent(new Event("focus")));
  await waitFor(() =>
    expect(listed()).toEqual([
      ["Guide", ""],
      ["Notes", ""],
    ])
  );
  await user.keyboard("{Enter}");

  expect(await screen.findByRole("heading", { level: 1, name: "Notes" })).toBeTruthy();
  expect(screen.queryByRole("dialog")).toBeNull();
});
