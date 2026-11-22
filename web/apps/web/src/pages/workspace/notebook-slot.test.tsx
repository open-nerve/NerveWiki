import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { runInAction } from "mobx";
import { useEffect } from "react";
import { Outlet } from "react-router";
import { expect, onTestFinished, test, vi } from "vitest";

import { routes } from "../../app/routes";
import type { Notebook } from "../../services/notebook.service";
import { useStore } from "../../stores/context";
import type { RootStore } from "../../stores/root.store";
import { json, notebookJSON, workspaceJSON } from "../../test/fakes";
import { notes, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The open notebook in its item of the left column's list (M13/P2/S2, decisions 776–778, 798, 800): its row, its
// page tree and its places, in a container the workspace's shell keeps by the notebook's id, which the item of the
// notebook of the address holds, the same as the notebook moves from one group to the other.

const home = `/lab/notebooks/${notebookJSON.id}`;
/** Atlas is open to the workspace to read: a team notebook, beside Plans, the account's own. */
const atlas: Notebook = {
  ...notebookJSON,
  id: "0199a2b4-0000-7000-8000-0000000000c2",
  name: "Atlas",
  workspace_access: "viewer",
  role: "reader",
};

const tree = () => screen.findByRole("navigation", { name: "Pages of Plans" });

/** Plans' server, Atlas listed with it; list, when given, is what the notebooks' reads answer instead. */
function server(list?: () => Notebook[]) {
  const plans = pageServer(
    list === undefined ? {} : { answers: { "GET /api/v0/workspaces/lab/notebooks": () => json({ data: list() }) } }
  );
  plans.others = [atlas];
  return plans;
}

/** renderAt renders the app at path over plans' server, with a way to the signed-in stores, once they are. */
function renderAt(path: string, plans: ReturnType<typeof server>) {
  let root: RootStore | undefined;
  function Capture() {
    const store = useStore();
    useEffect(() => {
      root = store;
    }, [store]);
    return <Outlet />;
  }
  const rendered = renderApp(path, plans.app, { routes: [{ Component: Capture, children: routes }] });
  const stores = () => {
    const notebooks = root?.notebooksOf(workspaceJSON);
    if (root === undefined || notebooks === undefined) throw new Error("the page has no signed-in stores");
    return { root, notebooks };
  };
  return { ...rendered, stores };
}

/** The column's list of the group named name, its items' nodes found though a dialog hides the rest from roles. */
function group(name: string): HTMLElement | null {
  const heading = [...document.querySelectorAll("h2")].find((each) => each.textContent === name);
  return heading === undefined ? null : document.querySelector(`ul[aria-labelledby="${heading.id}"]`);
}

/** The headings and navigations in element, in the document's order, each as its tag and its name. */
function landmarks(element: Element): string[] {
  return [...element.querySelectorAll("h2, nav")].map((each) =>
    each.tagName === "H2" ? `h2 ${each.textContent}` : `nav ${each.getAttribute("aria-label")}`
  );
}

/** A box of the viewport's, as getBoundingClientRect answers it. */
const box = (top: number, height: number) => new DOMRect(0, top, 200, height);

test("on a notebook's page its item of the list holds its row's heading, its pages, then its places; another's item its link alone", async () => {
  renderApp(pagePath(notes.id), server().app);
  await within(await tree()).findByRole("link", { name: "Notes" });

  const mine = screen.getByRole("list", { name: "My notebooks" });
  const items = [...mine.children];
  expect(items).toHaveLength(1);
  expect(landmarks(items[0] as Element)).toEqual(["h2 Plans", "nav Pages of Plans", "nav More of Plans"]);
  const team = screen.getByRole("list", { name: "Team notebooks" });
  expect(
    within(team)
      .getAllByRole("link")
      .map((link) => [link.textContent, link.getAttribute("href")])
  ).toEqual([["Atlas", `/lab/notebooks/${atlas.id}`]]);
  expect(landmarks(team)).toEqual([]);
});

test("the notebook moved to the team's group, its tree goes with it, the same, the focus on its row's button kept and brought into the column's view", async () => {
  const plans = server();
  const { stores } = renderAt(pagePath(notes.id), plans);
  const nav = await tree();
  const button = await within(nav).findByRole("button", { name: "Actions for Notes" });
  button.focus();
  const area = document.querySelector<HTMLElement>("[data-sidebar-scroll]");
  if (area === null) throw new Error("the column has no area that scrolls");
  // No layout in jsdom: the column's area shows 0–200, the button is below it.
  const laidOut = vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (this: Element) {
    return this === area ? box(0, 200) : this === button ? box(400, 24) : box(0, 0);
  });
  onTestFinished(() => laidOut.mockRestore());

  act(() => stores().notebooks.receive({ ...plans.notebook, workspace_access: "viewer" }));

  expect(screen.queryByRole("list", { name: "My notebooks" })).toBeNull();
  const team = screen.getByRole("list", { name: "Team notebooks" });
  expect(screen.getByRole("navigation", { name: "Pages of Plans" })).toBe(nav);
  expect(team.contains(nav)).toBe(true);
  expect(landmarks(team)).toEqual(["h2 Plans", "nav Pages of Plans", "nav More of Plans"]);
  expect(document.activeElement).toBe(button);
  // Below the area's bottom by 224, 8 to spare.
  expect(area.scrollTop).toBe(232);
});

test("the notebook moved to the team's group with a page's rename open and typed in, the dialog stays, its text and its focus too", async () => {
  const user = userEvent.setup();
  const plans = server();
  const { stores } = renderAt(pagePath(notes.id), plans);
  await user.click(await within(await tree()).findByRole("button", { name: "Actions for Notes" }));
  await user.click(await screen.findByRole("menuitem", { name: "Rename" }));
  const dialog = await screen.findByRole("dialog", { name: "Rename Notes" });
  const title = within(dialog).getByLabelText<HTMLInputElement>("Title");
  await user.clear(title);
  await user.type(title, "Journal");
  expect(document.activeElement).toBe(title);

  act(() => stores().notebooks.receive({ ...plans.notebook, workspace_access: "viewer" }));

  expect(group("My notebooks")).toBeNull();
  expect(group("Team notebooks")?.contains(document.querySelector('nav[aria-label="Pages of Plans"]'))).toBe(true);
  expect(screen.getByRole("dialog", { name: "Rename Notes" })).toBe(dialog);
  expect(title.value).toBe("Journal");
  expect(dialog.contains(document.activeElement)).toBe(true);
});

test("the notebook moved to the team's group in place, a row's menu open, the menu stays and measures the row's button again", async () => {
  const user = userEvent.setup();
  const { stores } = renderAt(pagePath(notes.id), server());
  const nav = await tree();
  const button = await within(nav).findByRole("button", { name: "Actions for Notes" });
  await user.click(button);
  const menu = await screen.findByRole("menu", { name: "Actions for Notes" });
  await waitFor(() => expect(button.getAttribute("aria-expanded")).toBe("true"));
  const measured = vi.spyOn(button, "getBoundingClientRect");
  onTestFinished(() => measured.mockRestore());
  const plans = stores().notebooks.byId(notebookJSON.id);
  if (plans === undefined) throw new Error("the list has no Plans");

  // The same object changed: the tree does not render again, only the lists, which read the access (decision 800).
  act(() =>
    runInAction(() => {
      plans.workspace_access = "viewer";
    })
  );

  expect(group("My notebooks")).toBeNull();
  expect(group("Team notebooks")?.contains(nav)).toBe(true);
  expect(screen.getByRole("menu", { name: "Actions for Notes" })).toBe(menu);
  expect(measured).toHaveBeenCalled();
});

test("a notebook gone from the list while a page of it is edited with changes not saved keeps its row and tree, after both groups", async () => {
  let list = [notebookJSON, atlas];
  const plans = server(() => list);
  const { stores } = renderAt(pagePath(notes.id), plans);
  const nav = await tree();
  await within(nav).findByRole("link", { name: "Notes" });
  const { root, notebooks } = stores();
  const unsaved = vi.spyOn(root, "unsavedEdit").mockReturnValue(true);
  onTestFinished(() => unsaved.mockRestore());

  list = [atlas];
  await act(() => notebooks.load());

  expect(screen.queryByRole("list", { name: "My notebooks" })).toBeNull();
  const team = screen.getByRole("list", { name: "Team notebooks" });
  expect(screen.getByRole("navigation", { name: "Pages of Plans" })).toBe(nav);
  expect(team.contains(nav)).toBe(false);
  expect(screen.getByRole("navigation", { name: "Lab" }).contains(nav)).toBe(true);
  expect(team.compareDocumentPosition(nav) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(screen.getByRole("heading", { level: 2, name: "Plans" })).toBeTruthy();
});

test("another notebook's link is arrived at: its item turns into its row, and its home's title takes the focus", async () => {
  const user = userEvent.setup();
  renderApp(pagePath(notes.id), server().app);
  await within(await tree()).findByRole("link", { name: "Notes" });

  await user.click(within(screen.getByRole("list", { name: "Team notebooks" })).getByRole("link", { name: "Atlas" }));

  const heading = await screen.findByRole("heading", { level: 1, name: "Atlas" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
  expect(landmarks(screen.getByRole("list", { name: "Team notebooks" }))).toEqual([
    "h2 Atlas",
    "nav Pages of Atlas",
    "nav More of Atlas",
  ]);
  expect(landmarks(screen.getByRole("list", { name: "My notebooks" }))).toEqual([]);
});

test("an address of the notebook's id in upper case is no page: the notebook stays a link of its group, and nothing follows the groups", async () => {
  renderApp(`/lab/notebooks/${notebookJSON.id.toUpperCase()}`, server().app);
  await screen.findByRole("heading", { level: 1, name: "Page not found" });

  const mine = screen.getByRole("list", { name: "My notebooks" });
  expect(
    within(mine)
      .getAllByRole("link")
      .map((link) => link.textContent)
  ).toEqual(["Plans"]);
  expect(landmarks(screen.getByRole("navigation", { name: "Lab" }))).toEqual(["h2 My notebooks", "h2 Team notebooks"]);
  // Not even an empty holder: New notebook comes right after the groups.
  expect(screen.getByRole("list", { name: "Team notebooks" }).parentElement?.nextElementSibling).toBe(
    screen.getByRole("button", { name: "New notebook" })
  );
});

test("the notebook's row is its tree's heading: a link to its home, marked there, not on a page; its cog, to its general settings, and New page, the tree's, beside it", async () => {
  const user = userEvent.setup();
  renderApp(home, server().app);
  const nav = await tree();

  const heading = screen.getByRole("heading", { level: 2, name: "Plans" });
  expect(nav.contains(heading)).toBe(false);
  expect(heading.compareDocumentPosition(nav) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  const row = within(heading).getByRole("link");
  expect([row.textContent, row.getAttribute("href"), row.getAttribute("aria-current")]).toEqual([
    "Plans",
    home,
    "page",
  ]);
  const cog = screen.getByRole("link", { name: "Settings of Plans" });
  expect(cog.getAttribute("href")).toBe(`${home}/settings/general`);
  expect(heading.contains(cog)).toBe(false);
  expect(within(nav).getByRole("button", { name: "New page" })).toBeTruthy();

  await user.click(await within(nav).findByRole("link", { name: "Notes" }));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  expect(row.getAttribute("aria-current")).toBeNull();
});

test("in Chinese the cog is the notebook's settings", async () => {
  const plans = server();
  plans.app.preferences.setLocale("zh-CN");
  renderApp(home, plans.app);
  await screen.findByRole("navigation", { name: "Plans的页面" });

  expect(screen.getByRole("link", { name: "Plans的设置" }).getAttribute("href")).toBe(`${home}/settings/general`);
});

test("a reader's row has the cog, and no New page", async () => {
  renderApp(home, pageServer({ role: "reader" }).app);
  const nav = await tree();
  await within(nav).findByRole("link", { name: "Notes" });

  expect(screen.getByRole("link", { name: "Settings of Plans" })).toBeTruthy();
  expect(within(nav).queryByRole("button", { name: "New page" })).toBeNull();
});

test("the notebook's places are icons, each named and titled by its text", async () => {
  renderApp(home, server().app);
  await tree();

  const places = screen.getByRole("navigation", { name: "More of Plans" });
  expect(
    within(places)
      .getAllByRole("link")
      .map((link) => [link.textContent, link.getAttribute("title"), link.querySelectorAll("svg").length])
  ).toEqual([
    ["Directory", "Directory", 1],
    ["Health", "Health", 1],
    ["History", "History", 1],
    ["Trash", "Trash", 1],
  ]);
});
