import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { arrived } from "../../app/arrival";
import { instanceJSON, json, signedInApp, workspaceJSON } from "../../test/fakes";
import { guide, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The shell of a workspace's pages (M2/P5 design 3.2).

const acme = { ...workspaceJSON, id: "0199a2b4-0000-7000-8000-0000000000a1", slug: "acme", name: "Acme" };
const nameField = () => screen.findByLabelText<HTMLInputElement>("Name");
const withWorkspaces = (routes = {}) =>
  signedInApp({ "GET /api/v0/workspaces": () => json({ data: [acme, workspaceJSON] }), ...routes });

test("the switcher lists the account's workspaces, checks the one shown, and goes to another", async () => {
  const user = userEvent.setup();
  const { router } = renderApp("/lab", withWorkspaces());

  await user.click(await screen.findByRole("button", { name: "Lab", description: "Switch workspace" }));
  const items = await screen.findAllByRole("menuitemradio");
  expect(items.map((item) => [item.textContent, item.getAttribute("aria-checked")])).toEqual([
    ["Acme", "false"],
    ["Lab", "true"],
  ]);
  await user.click(screen.getByRole("menuitemradio", { name: "Acme" }));

  expect(await screen.findByRole("heading", { name: "Acme" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/acme");
});

test.each([
  [true, ["Create workspace"]],
  [false, []],
])("with creation %s, the switcher offers to create a workspace: %j", async (enabled, offered) => {
  const user = userEvent.setup();
  renderApp(
    "/lab",
    withWorkspaces({ "GET /api/v0/instance": () => json({ ...instanceJSON, workspace_creation_enabled: enabled }) })
  );

  await user.click(await screen.findByRole("button", { name: "Lab" }));
  await screen.findAllByRole("menuitemradio");

  const items = screen.queryAllByRole("menuitem");
  expect(items.map((item) => item.textContent)).toEqual(offered);
  expect(items.map((item) => item.getAttribute("href"))).toEqual(offered.map(() => "/create-workspace"));
});

test("Create workspace in the switcher leaves the workspace's shell, arrived at: the page's title takes the focus", async () => {
  const user = userEvent.setup();
  const { router } = renderApp("/lab", withWorkspaces());

  await user.click(await screen.findByRole("button", { name: "Lab", description: "Switch workspace" }));
  await user.click(await screen.findByRole("menuitem", { name: "Create workspace" }));

  // The switcher leaves with the shell: its menu has no button to give the focus back to (M13/P2/S1, decision 771).
  const title = await screen.findByRole("heading", { level: 1, name: "Create a workspace" });
  expect(router.state.location.pathname).toBe("/create-workspace");
  await waitFor(() => expect(document.activeElement).toBe(title));
});

test("a workspace shown is the one the device lands on next", async () => {
  const app = withWorkspaces();
  renderApp("/acme", app);

  expect(await screen.findByRole("heading", { name: "Acme" })).toBeTruthy();
  expect(app.preferences.lastWorkspace()).toBe("acme");
});

// Only an arrival moves the focus (M3/P4 design 3.5): a page opened, or
// gone to from the switcher, leaves it where the browser puts it.
test("a workspace's home opened, or chosen in the switcher, does not take the focus", async () => {
  const user = userEvent.setup();
  renderApp("/lab", withWorkspaces());

  const lab = await screen.findByRole("heading", { name: "Lab" });
  expect(document.activeElement).not.toBe(lab);
  await user.click(screen.getByRole("button", { name: "Lab", description: "Switch workspace" }));
  await user.click(await screen.findByRole("menuitemradio", { name: "Acme" }));

  const chosen = await screen.findByRole("heading", { name: "Acme" });
  expect(document.activeElement).not.toBe(chosen);
});

test("a slug the account has no workspace of is not found, nor remembered", async () => {
  const app = withWorkspaces();
  renderApp("/zeta", app);

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
  expect(screen.queryByRole("navigation", { name: "Zeta" })).toBeNull();
  expect(screen.queryByRole("button", { description: "Switch workspace" })).toBeNull();
  expect(app.preferences.lastWorkspace()).toBeUndefined();
});

// The left column is no landmark of its own: the workspace's navigation is
// the one it holds (M3/P4 design 3.3). Its switcher is the column's top, above
// the search, outside the navigation (M13/P2/S1, decision 775; 00-M13 4.2.1).
test("the switcher heads the column, before the search, outside the navigation, which leads to the workspace's pages, marking the one shown", async () => {
  renderApp("/acme", withWorkspaces());

  const nav = await screen.findByRole("navigation", { name: "Acme" });
  expect(screen.queryByRole("complementary")).toBeNull();
  const column = nav.closest<HTMLElement>("[data-shell]");
  if (column === null) throw new Error("the navigation is in no column");
  const switcher = screen.getByRole("button", { name: "Acme", description: "Switch workspace" });
  expect(within(column).getAllByRole("button")[0]).toBe(switcher);
  expect(nav.contains(switcher)).toBe(false);
  const search = within(column).getByRole("search");
  expect(switcher.compareDocumentPosition(search) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(search.compareDocumentPosition(nav) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  const links = within(nav).getAllByRole("link");
  expect(links.map((link) => [link.textContent, link.getAttribute("href"), link.getAttribute("aria-current")])).toEqual(
    [
      ["Home", "/acme", "page"],
      // Straight to the first page: no index's redirect (M13/P3/S1, decision 816).
      ["Settings", "/acme/settings/general", null],
    ]
  );
});

// The settings' mode (M13/P3/S1, decisions 810, 816, 819): the column turns their directory, the same element, its
// drawer and strip kept; the search box goes, the keys left to the browser.

test("the navigation's Settings is arrived at: the column, the same element, turns the settings' directory, without the search", async () => {
  const user = userEvent.setup();
  const { router } = renderApp("/acme", withWorkspaces());
  await screen.findByRole("heading", { name: "Acme" });
  const column = document.querySelector("[data-shell]");
  expect(screen.getByRole("searchbox")).toBeTruthy();

  await user.click(within(screen.getByRole("navigation", { name: "Acme" })).getByRole("link", { name: "Settings" }));

  const title = await screen.findByRole("heading", { level: 1, name: "Workspace settings" });
  expect(router.state.location.pathname).toBe("/acme/settings/general");
  await waitFor(() => expect(document.activeElement).toBe(title));
  expect(document.querySelector("[data-shell]")).toBe(column);
  expect(screen.getByRole("navigation", { name: "Workspace settings" })).toBeTruthy();
  expect(screen.queryByRole("navigation", { name: "Acme" })).toBeNull();
  expect(screen.queryByRole("searchbox")).toBeNull();
});

test("in the settings' mode Mod+K and Mod+Shift+F are the browser's: no box to give the focus to; back, the shell's again", async () => {
  const user = userEvent.setup();
  const { router } = renderApp("/acme", withWorkspaces());
  await screen.findByRole("heading", { name: "Acme" });
  // The box there, the shell keeps the key from the browser.
  expect(fireEvent.keyDown(document.body, { key: "k", code: "KeyK", ctrlKey: true })).toBe(false);
  expect(document.activeElement).toBe(screen.getByRole("searchbox"));

  await act(() => router.navigate("/acme/settings/members", { state: arrived }));
  await screen.findByRole("heading", { level: 1, name: "Workspace settings" });

  expect(fireEvent.keyDown(document.body, { key: "k", code: "KeyK", ctrlKey: true })).toBe(true);
  expect(fireEvent.keyDown(document.body, { key: "F", code: "KeyF", ctrlKey: true, shiftKey: true })).toBe(true);

  // Back, the box mounted anew: the shell keeps the key again and gives it the focus (review C's N1).
  await user.click(screen.getByRole("link", { name: "Back to Acme" }));
  await screen.findByRole("heading", { level: 1, name: "Acme" });
  expect(fireEvent.keyDown(document.body, { key: "k", code: "KeyK", ctrlKey: true })).toBe(false);
  expect(document.activeElement).toBe(screen.getByRole("searchbox"));
});

/** A rectangle from top to bottom, 100px wide. */
const box = (top: number, bottom: number) => new DOMRect(0, top, 100, bottom - top);

// jsdom lays nothing out: the column's scrolling area and the row shown stand where the stubs put them (decision
// 814; review A's M3). As a browser does, the area clamps what it is scrolled to: without the tree in sight it has
// nothing to scroll (a tree taller than the column, as in the implementer's probe; review A's unverified suspicion), so
// the scroll is put back only once the tree's container is held in the column again, ScrollKept after it. And, its
// scrollTop changed, the area fires scroll afterwards, as a browser does: a listener left on in the settings' mode
// would keep their 0 (review C's fix check, M1).
test("the column's scroll is kept through the settings: their directory at its top; back, the column where it was, then the page's row brought into view", async () => {
  const sets: number[] = [];
  const values = new WeakMap<Element, number>();
  vi.spyOn(Element.prototype, "scrollTop", "get").mockImplementation(function (this: Element) {
    return values.get(this) ?? 0;
  });
  vi.spyOn(Element.prototype, "scrollTop", "set").mockImplementation(function (this: Element, value: number) {
    if (!this.hasAttribute("data-sidebar-scroll")) {
      values.set(this, value);
      return;
    }
    sets.push(value);
    const tree = this.querySelector('nav[aria-label="Pages of Plans"]');
    const before = values.get(this) ?? 0;
    const after = tree !== null && tree.closest("[hidden]") === null ? value : 0;
    values.set(this, after);
    if (after !== before) {
      setTimeout(() => this.dispatchEvent(new Event("scroll")), 0);
    }
  });
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (this: Element) {
    if (this.hasAttribute("data-sidebar-scroll")) {
      return box(0, 100);
    }
    if (this.closest("[data-sidebar-scroll]") !== null && this.getAttribute("aria-current") === "page") {
      return box(500, 530);
    }
    return box(0, 0);
  });
  const user = userEvent.setup();
  const { router } = renderApp(pagePath(guide.id), pageServer().app);
  await screen.findByRole("heading", { level: 1, name: "Guide" });
  const area = document.querySelector("[data-sidebar-scroll]") as HTMLElement;
  await act(() => new Promise((resolve) => setTimeout(resolve, 20)));
  area.scrollTop = 300;
  fireEvent.scroll(area);
  sets.length = 0;

  await user.click(screen.getByRole("link", { name: "Settings of Plans" }));
  await screen.findByRole("heading", { level: 1, name: "Plans settings" });
  expect(area.scrollTop).toBe(0);

  sets.length = 0;
  await user.click(screen.getByRole("link", { name: "Back to Plans" }));
  await waitFor(() => expect(router.state.location.pathname).toBe(pagePath(guide.id)));
  await act(() => new Promise((resolve) => setTimeout(resolve, 20)));
  // Put back first, then the row shown brought in, 8px to spare: 300 + (530 - 100) + 8.
  expect(sets[0]).toBe(300);
  expect(area.scrollTop).toBe(738);
});

test("a page of one workspace starts anew in another: a name typed for one is not offered to another", async () => {
  const user = userEvent.setup();
  const { router } = renderApp("/acme/settings/general", withWorkspaces());
  await user.clear(await nameField());
  await user.type(await nameField(), "Acme Two");

  await act(() => router.navigate("/lab/settings/general"));

  expect((await nameField()).value).toBe("Lab");
});

afterEach(() => vi.useRealTimers());

test("a workspace a later read no longer has is not found", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  let list = [acme, workspaceJSON];
  const { router } = renderApp("/acme", signedInApp({ "GET /api/v0/workspaces": () => json({ data: list }) }));
  expect(await screen.findByRole("heading", { name: "Acme" })).toBeTruthy();

  // Removed elsewhere; coming back to it reads the list again, once the read before is no longer recent.
  list = [workspaceJSON];
  await act(() => router.navigate("/create-workspace"));
  await act(() => vi.advanceTimersByTimeAsync(2_000));
  await act(() => router.navigate("/acme"));

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
});

test("the shell says so when the workspaces cannot be loaded; Try again loads them", async () => {
  const user = userEvent.setup();
  let down = true;
  renderApp(
    "/acme",
    signedInApp({
      "GET /api/v0/workspaces": () => (down ? Promise.reject(new TypeError("offline")) : json({ data: [acme] })),
    })
  );
  expect((await screen.findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );

  down = false;
  await user.click(screen.getByRole("button", { name: "Try again" }));

  expect(await screen.findByRole("heading", { name: "Acme" })).toBeTruthy();
});
