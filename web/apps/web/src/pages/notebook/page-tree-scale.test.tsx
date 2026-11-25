import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { spy } from "mobx";
import { useEffect } from "react";
import { Outlet } from "react-router";
import { expect, test, vi } from "vitest";

import { routes } from "../../app/routes";
import type { TreeNode } from "../../services/page.service";
import { useStore } from "../../stores/context";
import type { RootStore } from "../../stores/root.store";
import { notebookJSON } from "../../test/fakes";
import { pageNode, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The notebook's tree and its home's list at a tenth of the scale they are measured at (M12/P4/S4 design 4.1): a
// notebook of 1,000 pages at its root, an editor's, on its home, whose left column and main each list them all. What
// is guarded is what renders, not how long it takes (design 2.4 辛): each list draws its first rows at once and the
// rest a chunk at a time, another's change to the tree renders only the row it adds, a row holds no menu of its own
// nor a router's link, a dialog chosen from the menu renders once as it opens, and the lists all drawn render no more.
// The full scale is measured by e2e/perf (design 6.3).

/** Each time a row of the tree renders it calls useDrag once, an editor's row and a reader's alike: counted here. */
const rendered = vi.hoisted(() => ({ rows: 0 }));
vi.mock("./page-drag", async (importOriginal) => {
  const original = await importOriginal<typeof import("./page-drag")>();
  return {
    ...original,
    useDrag: (...args: Parameters<typeof original.useDrag>) => {
      rendered.rows += 1;
      return original.useDrag(...args);
    },
  };
});

/** Each time the "Move to…" dialog a row mounts renders: counted here (its form reads each page of the tree). */
const moving = vi.hoisted(() => ({ renders: 0 }));
vi.mock("./move-page-dialog", async (importOriginal) => {
  const original = await importOriginal<typeof import("./move-page-dialog")>();
  return {
    ...original,
    MovePageDialog: (props: Parameters<typeof original.MovePageDialog>[0]) => {
      moving.renders += 1;
      return <original.MovePageDialog {...props} />;
    },
  };
});

const home = `/lab/notebooks/${notebookJSON.id}`;
const many = 1_000;
// Ten thousand pages are measured; a tenth of them renders here within seconds.
const wait = { timeout: 20_000 };

/** manyPage is the page n of a large Plans at its root, titled pNNNN: its id as long as every other's (pageNode's stop at 899). */
function manyPage(n: number): TreeNode {
  return {
    ...pageNode(1, `p${String(n).padStart(4, "0")}`),
    id: `0199a2b4-0000-7000-8001-${String(n).padStart(12, "0")}`,
  };
}

/** flat is count pages at the root, from the page from on. */
const flat = (count: number, from = 1) => Array.from({ length: count }, (_, i) => manyPage(from + i));

/** The rows of the tree in the left column, and of the home's list of the root pages. */
const treeRows = () => document.querySelectorAll('nav[aria-label="Pages of Plans"] li').length;
const homeRows = () => document.querySelectorAll('ul[aria-label="Pages"] > li').length;

/** ofRadix counts the elements of root's subtree that Radix's triggers and menus mark: either of these. */
const ofRadix = (root: Element) => root.querySelectorAll('[data-state], [id^="radix-"]').length;

/** discovering counts the links of root that say they discover their route, as React Router's Link and NavLink do. */
const discovering = (root: Element) => root.querySelectorAll("a[data-discover]").length;

/** The title of the page shown. */
const shownTitle = () => document.querySelector("main h1")?.textContent;

/** How a list's rows came, one change of the document at a time: the first it had, its last, the most added at once. */
type Rows = { first?: number; last: number; most: number };

/**
 * watchRows watches the tree's rows and the home's, from now on, at each change of the document: a commit of React's
 * is one, its observer called in a microtask after it; the next chunk is drawn in a task of its own, another change.
 */
function watchRows() {
  const tree: Rows = { last: treeRows(), most: 0 };
  const main: Rows = { last: homeRows(), most: 0 };
  const lists: [Rows, () => number][] = [
    [tree, treeRows],
    [main, homeRows],
  ];
  const observer = new MutationObserver(() => {
    for (const [rows, count] of lists) {
      const now = count();
      if (rows.first === undefined && rows.last === 0 && now > 0) {
        rows.first = now;
      } else {
        rows.most = Math.max(rows.most, now - rows.last);
      }
      rows.last = now;
    }
  });
  observer.observe(document.body, { childList: true, subtree: true });
  return { tree, home: main, stop: () => observer.disconnect() };
}

/** renderHome renders Plans' home over server, and how to read its tree again, as another's change has it read. */
function renderHome(server: ReturnType<typeof pageServer>) {
  let store: RootStore | undefined;
  function CaptureStore() {
    const current = useStore();
    useEffect(() => {
      store = current;
    }, [current]);
    return <Outlet />;
  }
  const view = renderApp(home, server.app, { routes: [{ Component: CaptureStore, children: routes }] });
  return {
    ...view,
    readAgain: () =>
      act(async () => {
        await store?.pagesOf(server.notebook)?.load();
      }),
  };
}

test("a notebook of 1,000 pages at its root shows the first 100 of each list at once and the rest at most 400 a commit, all of them in the end", async () => {
  // Watched before the app renders: the first commit that has rows is the first change that sees them.
  const watched = watchRows();
  try {
    renderHome(pageServer({ nodes: flat(many) }));
    await waitFor(() => expect([watched.tree.first, watched.home.first]).not.toContain(undefined), wait);
    await waitFor(() => expect([treeRows(), homeRows()]).toEqual([many, many]), wait);
  } finally {
    watched.stop();
  }
  expect(watched.tree.first).toBeLessThanOrEqual(100);
  expect(watched.home.first).toBeLessThanOrEqual(100);
  // Drawn after the first: more than one change, none of more than a chunk.
  expect(watched.tree.most).toBeGreaterThan(0);
  expect(watched.home.most).toBeGreaterThan(0);
  expect(watched.tree.most).toBeLessThanOrEqual(400);
  expect(watched.home.most).toBeLessThanOrEqual(400);
  const nav = screen.getByRole("navigation", { name: "Pages of Plans" });
  const href = pagePath(manyPage(many).id);
  expect(nav.querySelector(`a[href="${href}"]`)?.textContent).toBe("p1000");
  expect(document.querySelector(`ul[aria-label="Pages"] a[href="${href}"]`)?.textContent).toBe("p1000");
}, 30_000);

test("a notebook of 1,000 pages, its lists all drawn, renders nothing more while idle", async () => {
  let reactions = 0;
  const stop = spy((event) => {
    if (event.type === "reaction") {
      reactions += 1;
    }
  });
  try {
    renderHome(pageServer({ nodes: flat(many) }));
    await waitFor(() => expect([treeRows(), homeRows()]).toEqual([many, many]), wait);
    // The count counts: each chunk is a render of the lists, which the spy reports.
    expect(reactions).toBeGreaterThan(0);
    reactions = 0;
    // Some frames' worth: a list that drew its last chunk asks for no other.
    await act(() => new Promise((resolve) => setTimeout(resolve, 500)));
  } finally {
    stop();
  }
  expect(reactions).toBe(0);
}, 30_000);

test("a list of pages all shown, grown by 500 at once, shows them at most 400 a commit", async () => {
  const server = pageServer({ nodes: flat(10) });
  const view = renderHome(server);
  await waitFor(() => expect([treeRows(), homeRows()]).toEqual([10, 10]), wait);
  const watched = watchRows();
  try {
    // As an import of a flat archive adds them, after the pages there: just past a chunk, more than one at once.
    server.nodes = [...server.nodes, ...flat(500, 11)];
    await view.readAgain();
    await waitFor(() => expect([treeRows(), homeRows()]).toEqual([510, 510]), wait);
  } finally {
    watched.stop();
  }
  expect(watched.tree.most).toBeGreaterThan(0);
  expect(watched.home.most).toBeGreaterThan(0);
  expect(watched.tree.most).toBeLessThanOrEqual(400);
  expect(watched.home.most).toBeLessThanOrEqual(400);
}, 30_000);

test("another's change to a tree of 1,000 pages renders only the row it adds, and runs a few reactions", async () => {
  const server = pageServer({ nodes: flat(many) });
  const view = renderHome(server);
  await waitFor(() => expect([treeRows(), homeRows()]).toEqual([many, many]), wait);
  // The count counts: each row rendered once at least.
  expect(rendered.rows).toBeGreaterThanOrEqual(many);
  rendered.rows = 0;
  let reactions = 0;
  const stop = spy((event) => {
    if (event.type === "reaction") {
      reactions += 1;
    }
  });
  try {
    server.nodes = [manyPage(5_000), ...server.nodes];
    await view.readAgain();
    await waitFor(() => expect(treeRows()).toBe(many + 1), wait);
    await waitFor(() => expect(homeRows()).toBe(many + 1), wait);
  } finally {
    stop();
  }
  // The row added, and one to spare; each observer that reads the tree once: the tree, its list, its menu, the home.
  expect(rendered.rows).toBeGreaterThanOrEqual(1);
  expect(rendered.rows).toBeLessThanOrEqual(2);
  expect(reactions).toBeGreaterThanOrEqual(1);
  expect(reactions).toBeLessThanOrEqual(10);
}, 30_000);

test("a row of the tree holds no menu of its own and no router link, until one is opened and after", async () => {
  const user = userEvent.setup();
  renderHome(pageServer({ nodes: flat(many) }));
  await waitFor(() => expect([treeRows(), homeRows()]).toEqual([many, many]), wait);
  const nav = screen.getByRole("navigation", { name: "Pages of Plans" });
  const inRows = () => [...nav.querySelectorAll("li")].reduce((sum, row) => sum + ofRadix(row), 0);
  expect(inRows()).toBe(0);

  const button = within(nav).getByRole("button", { name: "Actions for p0500" });
  await user.click(button);
  await screen.findByRole("menu", { name: "Actions for p0500" });
  // The tree's own menu is Radix's, outside the rows.
  expect(ofRadix(nav)).toBeGreaterThan(0);
  expect(inRows()).toBe(0);
  await user.keyboard("{Escape}");
  await waitFor(() => expect(document.activeElement).toBe(button));
  expect(inRows()).toBe(0);

  // A plain anchor does not say it discovers its route.
  expect(nav.querySelectorAll("li a")).toHaveLength(many);
  expect(discovering(nav.querySelector("ul") ?? nav)).toBe(0);
  const list = document.querySelector('ul[aria-label="Pages"]') ?? document.body;
  expect(list.querySelectorAll("a")).toHaveLength(many);
  expect(discovering(list)).toBe(0);
  // The shell's router links do: this tells the guard still sees what a router's link is.
  expect(screen.getByRole("link", { name: "Settings of Plans" }).getAttribute("data-discover")).toBe("true");
}, 30_000);

test("following a page of a tree of 1,000 renders the two rows whose mark changes", async () => {
  const user = userEvent.setup();
  const { router } = renderHome(pageServer({ nodes: flat(many) }));
  await waitFor(() => expect([treeRows(), homeRows()]).toEqual([many, many]), wait);
  const nav = screen.getByRole("navigation", { name: "Pages of Plans" });
  // Found by address and title, not by role: a role's query over two thousand rows takes seconds.
  const link = (n: number) => nav.querySelector(`a[href="${pagePath(manyPage(n).id)}"]`);
  await user.click(link(100) ?? nav);
  await waitFor(() => expect(shownTitle()).toBe("p0100"), wait);
  expect(link(100)?.getAttribute("aria-current")).toBe("page");
  rendered.rows = 0;

  await user.click(link(500) ?? nav);

  await waitFor(() => expect(shownTitle()).toBe("p0500"), wait);
  expect(router.state.location.pathname).toBe(pagePath(manyPage(500).id));
  expect(link(500)?.getAttribute("aria-current")).toBe("page");
  expect(link(100)?.getAttribute("aria-current")).toBeNull();
  expect(rendered.rows).toBeGreaterThanOrEqual(1);
  expect(rendered.rows).toBeLessThanOrEqual(2);
}, 30_000);

test("a dialog chosen from a row's menu renders once as it opens, not again as the menu closes", async () => {
  const user = userEvent.setup();
  renderHome(pageServer({ nodes: flat(many) }));
  await waitFor(() => expect([treeRows(), homeRows()]).toEqual([many, many]), wait);
  const nav = screen.getByRole("navigation", { name: "Pages of Plans" });
  await user.click(within(nav).getByRole("button", { name: "Actions for p0500" }));
  const item = await screen.findByRole("menuitem", { name: "Move to…" });
  moving.renders = 0;
  const focused: EventTarget[] = [];
  const record = (event: FocusEvent) => void focused.push(event.target ?? document);
  document.addEventListener("focusin", record, true);

  try {
    await user.click(item);

    const dialog = await screen.findByRole("dialog", { name: "Move p0500" });
    await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());
    await waitFor(() => expect(dialog.contains(document.activeElement)).toBe(true));
    await act(() => new Promise((resolve) => setTimeout(resolve, 100)));
  } finally {
    document.removeEventListener("focusin", record, true);
  }
  // Once: the row rendering again as its button's menu closes leaves the dialog be.
  expect(moving.renders).toBe(1);
  // The focus goes into the dialog once: the menu's closing does not ask the row's button for it, which the dialog's
  // trap would take back, its first control focused again.
  expect(focused.length).toBeGreaterThan(0);
  expect(new Set(focused).size).toBe(focused.length);
}, 30_000);
