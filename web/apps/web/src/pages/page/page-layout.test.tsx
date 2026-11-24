import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect } from "react";
import { Outlet } from "react-router";
import { afterEach, expect, test, vi } from "vitest";

import { arrived } from "../../app/arrival";
import { routes } from "../../app/routes";
import { eventHandlers } from "../../events/handlers";
import { useStore } from "../../stores/context";
import type { RootStore } from "../../stores/root.store";
import { eventServer, withEvents } from "../../test/event-server";
import { json, notebookJSON, problem, workspaceJSON } from "../../test/fakes";
import { pageEditor } from "../../test/page-editor";
import { section } from "../../test/page-panel";
import { bob, guide, install, linux, notes, pageNode, pagePath, pageServer, versionJSON } from "../../test/page-server";
import { renderApp } from "../../test/render";

// A page's shell: where it is, its title, its reading view, its subpages
// (M4/P5 design 3.5, 3.8).

afterEach(() => vi.useRealTimers());

const main = () => screen.getByRole("main");

test("a page shows where it is, its title, its reading view and its subpages, all in the main", async () => {
  renderApp(pagePath(install.id), pageServer().app);

  const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
  expect(main().contains(heading)).toBe(true);
  const crumbs = within(main()).getByRole("navigation", { name: "Breadcrumb" });
  expect(
    within(crumbs)
      .getAllByRole("listitem")
      .map((item) =>
        [...item.querySelectorAll(":scope > :not([aria-hidden])")].map((step) => step.textContent).join("")
      )
  ).toEqual(["Plans", "Guide", "Install"]);
  expect([...crumbs.querySelectorAll("[aria-hidden]")].map((separator) => separator.textContent)).toEqual(["/", "/"]);
  expect(within(crumbs).getByRole("link", { name: "Plans" }).getAttribute("href")).toBe(
    `/lab/notebooks/${notebookJSON.id}`
  );
  expect(within(crumbs).getByText("Install").getAttribute("aria-current")).toBe("page");
  expect(within(crumbs).queryByRole("link", { name: "Install" })).toBeNull();
  // Named by the page: it gets the focus when its content is wider than it shows.
  expect((await within(main()).findByRole("article", { name: "Install" })).innerHTML).toBe("<p>Install</p>");
  const subpages = within(main()).getByRole("list", { name: "Subpages" });
  expect(within(subpages).getByRole("link", { name: "Linux" }).getAttribute("href")).toBe(pagePath(linux.id));
});

// M12/P2/S1, 12.3: the link leaves with the page it is on; its page is arrived at.
test("a subpage's link and a breadcrumb's lead to a page arrived at: its title takes the focus the link took with it", async () => {
  const user = userEvent.setup();
  renderApp(pagePath(install.id), pageServer().app);
  await screen.findByRole("heading", { level: 1, name: "Install" });

  const subpages = await within(main()).findByRole("list", { name: "Subpages" });
  await user.click(within(subpages).getByRole("link", { name: "Linux" }));
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Linux" })));

  const crumbs = within(main()).getByRole("navigation", { name: "Breadcrumb" });
  await user.click(within(crumbs).getByRole("link", { name: "Guide" }));
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Guide" })));
});

test("a page without children lists none", async () => {
  renderApp(pagePath(linux.id), pageServer().app);

  expect(await screen.findByRole("article")).toBeTruthy();
  expect(screen.queryByRole("list", { name: "Subpages" })).toBeNull();
});

test("a page the tree does not have is not found, inside the notebook", async () => {
  renderApp(pagePath("0199a2b4-0000-7000-8000-0000000001ff"), pageServer().app);

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
  expect(screen.getByRole("navigation", { name: "Pages of Plans" })).toBeTruthy();
});

test("until the tree is read, the page shows nothing of itself; Try again reads it", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  server.nodesDown = true;
  renderApp(pagePath(install.id), server.app);

  await user.click(await within(main()).findByRole("button", { name: "Try again" }));
  expect(screen.queryByRole("heading", { level: 1, name: "Install" })).toBeNull();
  server.nodesDown = false;
  await user.click(within(main()).getByRole("button", { name: "Try again" }));

  expect(await screen.findByRole("heading", { level: 1, name: "Install" })).toBeTruthy();
});

test("a page gone to within its notebook whose tree could not be read says why at once, with Try again (M5-polish 27.2)", async () => {
  const server = pageServer();
  server.nodesDown = true;
  const { router } = renderApp(`/lab/notebooks/${notebookJSON.id}`, server.app);
  await waitFor(() => expect(server.sent).toContain("GET nodes"));
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));

  await act(() => router.navigate(pagePath(install.id)));

  // The failure is the notebook's own, as it shows: not one from before it was left.
  expect(await within(main()).findByRole("button", { name: "Try again" })).toBeTruthy();
  expect(server.sent.filter((each) => each === "GET nodes")).toHaveLength(1);
});

test("the reading view says so when it cannot be read; Try again reads it", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  server.viewsDown = true;
  renderApp(pagePath(install.id), server.app);

  await screen.findByRole("heading", { level: 1, name: "Install" });
  await user.click(await within(main()).findByRole("button", { name: "Try again" }));
  server.viewsDown = false;
  await user.click(within(main()).getByRole("button", { name: "Try again" }));

  expect((await screen.findByRole("article")).innerHTML).toBe("<p>Install</p>");
});

test("a busy server's reading view is read again after its Retry-After", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  let busy = true;
  const server = pageServer({
    answers: {
      [`GET /api/v0/pages/${install.id}/view`]: () =>
        busy
          ? problem(503, "server_busy", {}, { "Retry-After": "1" })
          : json({ html: "<p>Read at last</p>", revision: 1, assets_expire_at: null }),
    },
  });
  renderApp(pagePath(install.id), server.app);

  expect(await screen.findByText("The server is busy. Try again in a moment.")).toBeTruthy();
  busy = false;
  await act(() => vi.advanceTimersByTimeAsync(1_000));

  expect((await screen.findByRole("article")).innerHTML).toBe("<p>Read at last</p>");
});

test("the reading view read again shows another's write", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer();
  renderApp(pagePath(install.id), server.app);
  expect((await screen.findByRole("article")).innerHTML).toBe("<p>Install</p>");

  server.views.set(install.id, { html: "<p>Install, changed</p>", revision: 2, assets_expire_at: null });
  await act(() => vi.advanceTimersByTimeAsync(6_000));
  act(() => void window.dispatchEvent(new Event("focus")));

  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Install, changed</p>"));
});

test("the tree read again shows another tab's changes in the page's title, breadcrumbs and subpages", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer();
  renderApp(pagePath(install.id), server.app);
  await screen.findByRole("heading", { level: 1, name: "Install" });

  const renamed = { ...install, name: "Setup" };
  server.nodes = [{ ...guide, name: "Handbook" }, renamed, linux, pageNode(5, "macOS", renamed), notes];
  await act(() => vi.advanceTimersByTimeAsync(6_000));
  act(() => void window.dispatchEvent(new Event("focus")));

  expect(await screen.findByRole("heading", { level: 1, name: "Setup" })).toBeTruthy();
  const crumbs = within(main()).getByRole("navigation", { name: "Breadcrumb" });
  expect(within(crumbs).getByRole("link", { name: "Handbook" })).toBeTruthy();
  const subpages = within(main()).getByRole("list", { name: "Subpages" });
  expect(
    within(subpages)
      .getAllByRole("link")
      .map((link) => link.textContent)
  ).toEqual(["Linux", "macOS"]);
});

/** left goes from the page shown to the workspace's home: the notebook's shell, and its tree's watch, are gone. */
async function left(router: ReturnType<typeof renderApp>["router"]) {
  await act(() => router.navigate("/lab"));
  await waitFor(() => expect(screen.queryByRole("navigation", { name: "Pages of Plans" })).toBeNull());
}

/** alertsInserted records the text of each alert inserted from now on, until stopped. */
function alertsInserted() {
  const texts: string[] = [];
  const watching = new MutationObserver((records) => {
    for (const node of records.flatMap((record) => [...record.addedNodes])) {
      if (node instanceof HTMLElement) {
        const alerts = node.matches("[role=alert]") ? [node] : [...node.querySelectorAll("[role=alert]")];
        texts.push(...alerts.map((alert) => alert.textContent ?? ""));
      }
    }
  });
  watching.observe(document.body, { childList: true, subtree: true });
  return { texts, stop: () => watching.disconnect() };
}

/** pageReads are the reads of a page's own parts among sent. */
const pageReads = (sent: readonly string[]) =>
  sent.filter((each) => /^GET (view|properties|backlinks|assets)/.test(each));

test("a page deleted elsewhere while its notebook was left shows none of itself when gone back to: the tree is read first, and the page is not found (M5-polish 27.2)", async () => {
  const server = pageServer();
  const { router } = renderApp(pagePath(notes.id), server.app);
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  await left(router);
  // Deleted elsewhere: no event reads the tree, no shell of the notebook showing.
  server.trashNode(install.id);
  const before = server.sent.length;
  const inserted = alertsInserted();

  await act(() => router.navigate(pagePath(install.id), { state: arrived }));

  const heading = await screen.findByRole("heading", { level: 1, name: "Page not found" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
  inserted.stop();
  expect(pageReads(server.sent.slice(before))).toEqual([]);
  expect(inserted.texts).toEqual([]);
  expect(server.sent.slice(before).filter((each) => each === "GET nodes")).toHaveLength(1);
});

test("a page gone to within its notebook shows at once: its tree is not read again for it (M5-polish 27.2)", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  renderApp(pagePath(notes.id), server.app);
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  const reads = () => server.sent.filter((each) => each === "GET nodes").length;
  const before = reads();

  await user.click(
    within(screen.getByRole("navigation", { name: "Pages of Plans" })).getByRole("link", { name: "Guide" })
  );

  expect(await screen.findByRole("heading", { level: 1, name: "Guide" })).toBeTruthy();
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  expect(reads()).toBe(before);
});

test("a page gone back to after its notebook was left shows once its tree is read: a read that fails says why, with Try again (M5-polish 27.2)", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  const { router } = renderApp(pagePath(notes.id), server.app);
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  await left(router);
  server.nodesDown = true;

  await act(() => router.navigate(pagePath(install.id)));

  const retry = await within(main()).findByRole("button", { name: "Try again" });
  expect(within(main()).getByRole("alert").textContent).not.toBe("");
  expect(screen.queryByRole("heading", { level: 1, name: "Install" })).toBeNull();
  server.nodesDown = false;
  await user.click(retry);
  expect(await screen.findByRole("heading", { level: 1, name: "Install" })).toBeTruthy();
});

test("a tree read sent before the notebook was left and answered after it was gone back to confirms nothing: the page waits for a read of its own (M5-polish 27.2)", async () => {
  const events = eventServer();
  const server = pageServer({ answers: { "GET /api/v0/events": events.answer } });
  const { router } = renderApp(pagePath(install.id), withEvents(server.app), { eventHandlers });
  await screen.findByRole("heading", { level: 1, name: "Install" });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  events.last().hello();
  await act(() => new Promise((resolve) => setTimeout(resolve, 100)));
  // An event has the tree read again: held, it answers the tree as it is now, Install in it.
  server.nodesHeld = true;
  events.last().send("pages", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, tree: true, pages: [] });
  await waitFor(() => expect(server.held).toHaveLength(1));
  await left(router);
  server.trashNode(install.id);
  const before = server.sent.length;

  await act(() => router.navigate(pagePath(install.id)));
  await waitFor(() => expect(server.held).toHaveLength(2));
  // The read sent before the notebook was left answers: it confirms nothing.
  act(() => server.held.shift()?.());
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));

  expect(pageReads(server.sent.slice(before))).toEqual([]);
  expect(screen.queryByRole("heading", { level: 1, name: "Install" })).toBeNull();
  act(() => server.release());
  expect(await screen.findByRole("heading", { level: 1, name: "Page not found" })).toBeTruthy();
});

test("a page whose notebook was left before its first tree read answered, deleted elsewhere since, shows none of itself when gone back to: the tree is read again first (M5-polish 27.2)", async () => {
  const server = pageServer();
  server.nodesHeld = true;
  const { router } = renderApp(pagePath(install.id), server.app);
  await waitFor(() => expect(server.held).toHaveLength(1));
  await act(() => router.navigate("/lab"));
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  server.nodesHeld = false;
  // The read sent as the notebook showed answers once it is left: the tree as it was, Install in it.
  act(() => server.release());
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  server.trashNode(install.id);
  const before = server.sent.length;
  const inserted = alertsInserted();

  await act(() => router.navigate(pagePath(install.id)));

  expect(await screen.findByRole("heading", { level: 1, name: "Page not found" })).toBeTruthy();
  inserted.stop();
  expect(pageReads(server.sent.slice(before))).toEqual([]);
  expect(inserted.texts).toEqual([]);
  expect(server.sent.slice(before).filter((each) => each === "GET nodes")).toHaveLength(1);
});

test("a page gone back to after a tree read failed as its notebook was left says nothing of that failure while its tree is read again (M5-polish 27.2)", async () => {
  const events = eventServer();
  const server = pageServer({ answers: { "GET /api/v0/events": events.answer } });
  const { router } = renderApp(pagePath(notes.id), withEvents(server.app), { eventHandlers });
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  events.last().hello();
  await act(() => new Promise((resolve) => setTimeout(resolve, 100)));
  // A blip: the tree read an event has fails, SWR to try again in 5 seconds, which the notebook left stops.
  server.nodesDown = true;
  const reads = () => server.sent.filter((each) => each === "GET nodes").length;
  const count = reads();
  events.last().send("pages", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, tree: true, pages: [] });
  await waitFor(() => expect(reads()).toBe(count + 1));
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  await left(router);
  server.nodesDown = false;
  server.nodesHeld = true;
  const inserted = alertsInserted();

  await act(() => router.navigate(pagePath(notes.id)));
  await waitFor(() => expect(server.held).toHaveLength(1));
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));

  expect(inserted.texts).toEqual([]);
  expect(screen.queryByRole("button", { name: "Try again" })).toBeNull();
  act(() => server.release());
  expect(await screen.findByRole("heading", { level: 1, name: "Notes" })).toBeTruthy();
  inserted.stop();
  expect(inserted.texts).toEqual([]);
});

// The page's head in one row (M13/P4/S1, decisions 858, 865, 894): its breadcrumbs, then its right group, who edited
// the page last first in it, then the right column's toggle, the page's menu and Edit.

/** edited finds the paragraph of the page's head that says text, its names in it. */
const edited = (text: string) => (_: string, element: Element | null) =>
  element?.tagName === "P" && element.textContent === text;

/** group is the page's head's right group: the right column's toggle's. */
function group(): HTMLElement {
  const toggle = within(main()).getByRole("button", { name: "About this page" });
  if (toggle.parentElement === null) {
    throw new Error("the toggle is in no group");
  }
  return toggle.parentElement;
}

test("the page's head says who edited the page last, by its newest version, the name in a bdi, the time in full as its title; read once with the right column's; a paragraph, no status", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-10-04T10:00:00Z"));
  const server = pageServer();
  server.withVersions(install.id, [
    versionJSON(1, null, "Install\n", { by: bob, at: "2026-09-21T09:00:00Z" }),
    versionJSON(2, 1, "Install\nmore\n", { at: "2026-10-03T08:00:00Z" }),
  ]);
  renderApp(pagePath(install.id), server.app);

  const said = await within(main()).findByText(edited("Edited by Ada yesterday"));
  expect([said.querySelector("bdi")?.textContent, said.getAttribute("title")]).toEqual(["Ada", "Oct 3, 2026, 8:00 AM"]);
  expect(group().firstElementChild?.contains(said)).toBe(true);
  expect(said.closest("output, [role=status]")).toBeNull();
  expect(
    within(main())
      .queryAllByRole("status")
      .some((status) => status.contains(said))
  ).toBe(false);
  await within(section("Versions")).findByRole("list", { name: "Versions" });
  expect(server.sent.filter((sent) => sent.startsWith("GET versions"))).toEqual(["GET versions Install"]);
});

test("in Chinese, the page's head says who edited it as the mockup has it", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-10-04T10:00:00Z"));
  const server = pageServer();
  server.withVersions(install.id, [versionJSON(1, null, "Install\n", { at: "2026-10-03T08:00:00Z" })]);
  server.app.preferences.setLocale("zh-CN");
  renderApp(pagePath(install.id), server.app);

  const said = await within(main()).findByText(edited("Ada 编辑于昨天"));
  expect(said.querySelector("bdi")?.textContent).toBe("Ada");
});

test.each([
  ["en", "Edited by Ada yesterday"],
  ["zh-CN", "Ada 编辑于昨天"],
] as const)(
  "%s: a long name is shortened alone in the page's head, the time kept whole (decision 899)",
  async (locale, text) => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.setSystemTime(new Date("2026-10-04T10:00:00Z"));
    const server = pageServer();
    server.withVersions(install.id, [versionJSON(1, null, "Install\n", { at: "2026-10-03T08:00:00Z" })]);
    server.app.preferences.setLocale(locale);
    renderApp(pagePath(install.id), server.app);

    const said = await within(main()).findByText(edited(text));
    const name = said.querySelector("bdi");
    // The sentence a row that does not shorten itself; the name in it shrinks and ends in an ellipsis (truncate).
    expect([said.className.split(" ").includes("truncate"), name?.className.split(" ")]).toEqual([
      false,
      ["min-w-0", "truncate"],
    ]);
    expect(said.className).toContain("whitespace-pre");
    // Too narrow for the words around the name, the start kept and the end cut (review A's P4S1A-F1-N2).
    expect(said.className.split(" ")).toContain("justify-end-safe");
  }
);

test("the page's head says nothing of who edited it while the page is edited", async () => {
  const server = pageServer();
  renderApp(pagePath(install.id), server.app);
  await within(main()).findByText(
    (_, element) => element?.tagName === "P" && (element.textContent ?? "").startsWith("Edited by ")
  );

  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true });
  await pageEditor();
  expect(within(main()).queryByText(/^Edited by /)).toBeNull();
});

test("a page without versions: the page's head's box of who edited it is there, empty", async () => {
  const server = pageServer();
  server.versions.set(install.id, []);
  renderApp(pagePath(install.id), server.app);
  await within(await screen.findByRole("complementary", { name: "About this page" })).findByText(
    "One version per edit."
  );

  const box = group().firstElementChild;
  expect(box?.querySelector("p")?.textContent).toBe("");
  expect(within(main()).queryByText(/^Edited by /)).toBeNull();
});

test("the page's head comes before its title: its breadcrumbs, then its right group", async () => {
  renderApp(pagePath(install.id), pageServer().app);

  const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
  const crumbs = within(main()).getByRole("navigation", { name: "Breadcrumb" });
  expect(crumbs.compareDocumentPosition(group()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(group().compareDocumentPosition(heading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(within(group()).getByRole("button", { name: "Edit" })).toBeTruthy();
  expect(within(group()).getByRole("button", { name: "Page actions" })).toBeTruthy();
});

test("Edit, holding the focus as an event makes the account a reader, gives it to the page's title, the window not scrolled", async () => {
  const server = pageServer({ role: "editor" });
  let root: RootStore | undefined;
  function Capture() {
    const store = useStore();
    useEffect(() => {
      root = store;
    }, [store]);
    return <Outlet />;
  }
  renderApp(pagePath(install.id), server.app, { routes: [{ Component: Capture, children: routes }] });
  const edit = await within(main()).findByRole("button", { name: "Edit" });
  edit.focus();
  const focus = vi.spyOn(HTMLElement.prototype, "focus");

  server.notebook = { ...server.notebook, role: "reader" };
  act(() => root?.notebooksOf(workspaceJSON)?.receive(server.notebook));

  expect(within(main()).queryByRole("button", { name: "Edit" })).toBeNull();
  const heading = screen.getByRole("heading", { level: 1, name: "Install" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
  const given = focus.mock.contexts.indexOf(heading);
  expect(given).not.toBe(-1);
  expect(focus.mock.calls[given]).toEqual([{ preventScroll: true }]);
});
