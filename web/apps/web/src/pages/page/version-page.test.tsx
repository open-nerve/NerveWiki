import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { appLinks } from "../../reading/app-links";
import { taskToggle } from "../../reading/task-toggle";
import { unresolvedLinks } from "../../reading/unresolved-links";
import type { NotebookRole } from "../../services/notebook.service";
import { json, type Answer } from "../../test/fakes";
import { bob, guide, install, pagePath, pageServer, versionJSON } from "../../test/page-server";
import { renderApp } from "../../test/render";

// A version's page (M8 design 4.3, 4.13; P1 design 4.4).

afterEach(() => {
  vi.useRealTimers();
});

/** versionPath is the address of the version of the page id that holds revision. */
const versionPath = (id: string, revision: number | string) => `${pagePath(id)}/versions/${revision.toString()}`;

/** The enhancements of a version's view these tests look at: the task items', the links'. */
const enhancements = [taskToggle, appLinks, unresolvedLinks];

/**
 * installed is Install over pageServer, with three versions: the first, of
 * task items, its second, of one more line, and the newest, written over;
 * Ada's role in Plans is role, answers answering instead of it.
 */
function installed(role: NotebookRole = "admin", answers: Record<string, Answer> = {}) {
  const server = pageServer({ role, answers });
  server.withVersions(install.id, [
    versionJSON(1, null, "- [ ] one\n- [x] two\n", { kind: "import", by: bob, at: "2026-10-01T09:00:00Z" }),
    versionJSON(4, 1, "- [ ] one\n- [x] two\n- [ ] three\n", { at: "2026-10-02T09:00:00Z" }),
    versionJSON(6, 4, "Install now\n", { at: "2026-10-03T09:00:00Z" }),
  ]);
  return server;
}

test("a version shows where its page is, its revision, when it was written, by whom and what, and its view, read only: no task item ticks; its tab is named for it", async () => {
  const user = userEvent.setup();
  const server = installed();
  renderApp(versionPath(install.id, 4), server.app, { enhancements });

  expect(await screen.findByRole("heading", { level: 1, name: "Version 4" })).toBeTruthy();
  await waitFor(() => expect(document.title).toBe("Version 4 · Install · Plans · Nerve Wiki"));
  const crumbs = screen.getByRole("navigation", { name: "Breadcrumb" });
  expect(
    within(crumbs)
      .getAllByRole("link")
      .map((link) => [link.textContent, link.getAttribute("href")])
  ).toEqual([
    ["Plans", "/lab/notebooks/0199a2b4-0000-7000-8000-0000000000c1"],
    ["Guide", pagePath(guide.id)],
    ["Install", pagePath(install.id)],
  ]);
  expect(within(crumbs).getByText("Version 4").getAttribute("aria-current")).toBe("page");
  expect(await screen.findByText("· Ada · Edit")).toBeTruthy();
  expect(screen.getByText("Oct 2, 2026, 9:00 AM").getAttribute("dateTime")).toBe("2026-10-02T09:00:00Z");

  const tabs = screen.getByRole("tablist", { name: "Show this version" });
  expect(
    within(tabs)
      .getAllByRole("tab")
      .map((tab) => [tab.textContent, tab.getAttribute("aria-selected")])
  ).toEqual([
    ["Read", "true"],
    ["Compare with the previous", "false"],
    ["Compare with the current", "false"],
  ]);
  const article = await screen.findByRole("article", { name: "Version 4" });
  const boxes = await within(article).findAllByRole("checkbox");
  expect(boxes.map((box) => [(box as HTMLInputElement).disabled, (box as HTMLInputElement).checked])).toEqual([
    [true, false],
    [true, true],
    [true, false],
  ]);
  await user.click(boxes[0] as HTMLElement);
  expect(server.sent.filter((each) => each.startsWith("TOGGLE"))).toEqual([]);
  expect(server.sent).toContain("GET version view Install 4");
});

test("a link of a version's view to a page that is not there says so, offering no page to create, and asks the server nothing", async () => {
  const user = userEvent.setup();
  const server = installed();
  server.versionViews.set(`${install.id} 4`, {
    html: '<p><a class="nw-unresolved" data-nw-target="Nowhere">Nowhere</a></p>',
    revision: 4,
    assets_expire_at: null,
  });
  renderApp(versionPath(install.id, 4), server.app, { enhancements });

  await user.click(await screen.findByRole("button", { name: "Nowhere" }));
  const dialog = await screen.findByRole("alertdialog", { name: "“Nowhere” does not exist" });
  expect(
    within(dialog).getByText("No page has this name. A version is read only: no page is created from its links.")
  ).toBeTruthy();
  expect(within(dialog).queryByRole("button", { name: "Create page" })).toBeNull();
  expect(server.sent.filter((each) => each.startsWith("GET landing"))).toEqual([]);
});

test("the tabs are a named group the arrow keys move in, Enter choosing: what the version changed from the one before it, and what restoring it changes of the page now", async () => {
  const user = userEvent.setup();
  const server = installed();
  renderApp(versionPath(install.id, 4), server.app);
  const read = await screen.findByRole("tab", { name: "Read" });

  act(() => read.focus());
  await user.keyboard("{ArrowRight}");
  const previous = screen.getByRole("tab", { name: "Compare with the previous" });
  expect(document.activeElement).toBe(previous);
  // Chosen by Enter, not as the focus moves: each comparison reads what it compares.
  expect(previous.getAttribute("aria-selected")).toBe("false");
  await user.keyboard("{Enter}");
  expect(previous.getAttribute("aria-selected")).toBe("true");
  const changes = await screen.findByRole("textbox", { name: "Changes from version 1 to version 4" });
  expect(screen.getByRole("tabpanel").textContent).toContain("Version 1 → Version 4");
  expect(changes.textContent).toContain("three");
  expect(server.sent).toContain("GET version Install 1");

  await user.keyboard("{ArrowRight}{Enter}");
  const restoring = await screen.findByRole("textbox", { name: "Changes from the page now to version 4" });
  expect(screen.getByRole("tabpanel").textContent).toContain("The page now → Version 4");
  expect(restoring.textContent).toContain("three");
  expect(server.sent).toContain("GET content Install");

  await user.keyboard("{Home}{Enter}");
  expect(await screen.findByRole("article", { name: "Version 4" })).toBeTruthy();
});

test("the version that created the page has nothing before it to compare with", async () => {
  const server = installed();
  renderApp(versionPath(install.id, 1), server.app);

  await screen.findByText("· Bob · Import");
  expect(screen.getAllByRole("tab").map((tab) => tab.textContent)).toEqual(["Read", "Compare with the current"]);
});

test("an address of a revision a version holds below its own is replaced with its own, the version read once", async () => {
  const server = installed();
  const { router } = renderApp(versionPath(install.id, 3), server.app);

  expect(await screen.findByRole("heading", { level: 1, name: "Version 4" })).toBeTruthy();
  expect(router.state.location.pathname).toBe(versionPath(install.id, 4));
  expect(router.state.historyAction).toBe("REPLACE");
  await screen.findByText("· Ada · Edit");
  expect(server.sent.filter((each) => each.startsWith("GET version Install"))).toEqual(["GET version Install 3"]);
});

test("an address of no page, or of no revision, is no page; a revision no version holds says so, with Try again", async () => {
  const server = installed();
  const { router } = renderApp(versionPath(install.id, "x"), server.app);
  expect(await screen.findByRole("heading", { level: 1, name: "Page not found" })).toBeTruthy();

  await act(() => router.navigate(versionPath("0199a2b4-0000-7000-8000-000000000999", 1)));
  expect(await screen.findByRole("heading", { level: 1, name: "Page not found" })).toBeTruthy();

  await act(() => router.navigate(versionPath(install.id, 7)));
  expect(await screen.findByText("This page has no such version.")).toBeTruthy();
  expect(screen.getByRole("button", { name: "Try again" })).toBeTruthy();
});

test("who edits the page shows as on the page", async () => {
  const server = installed();
  server.hold(install.id, bob);
  renderApp(versionPath(install.id, 4), server.app);

  expect(await screen.findByText("Bob is editing this page.")).toBeTruthy();
});

test.each<[string, NotebookRole, number, boolean]>([
  ["an admin restores a version", "admin", 4, true],
  ["an editor restores a version", "editor", 1, true],
  ["no one restores the newest version", "admin", 6, false],
  ["a reader restores none", "reader", 4, false],
])("%s", async (_, role, revision, restores) => {
  const server = installed(role);
  renderApp(versionPath(install.id, revision), server.app);

  await screen.findByRole("tab", { name: "Read" });
  // The newest is known once the versions are read.
  await waitFor(() => expect(server.sent).toContain("GET versions Install"));
  await act(async () => {});
  expect(screen.queryByRole("button", { name: "Restore this version" }) !== null).toBe(restores);
  expect(screen.queryByText("Current") !== null).toBe(revision === 6);
});

test("leaving a version's page takes what it read out of the cache: come back to, it is read anew, not shown from before", async () => {
  const gate = { holding: false, held: [] as (() => void)[] };
  const server = installed("admin", {
    // The server's version, which waits to be released while the gate holds.
    "GET /api/v0/pages/*/versions/*": async (request) => {
      const revision = Number(new URL(request.url).pathname.split("/")[6]);
      if (gate.holding) {
        await new Promise<void>((resolve) => gate.held.push(resolve));
      }
      return json(server.versions.get(install.id)?.find((version) => version.revision === revision));
    },
  });
  const { router } = renderApp(versionPath(install.id, 4), server.app);
  await screen.findByText("· Ada · Edit");
  await screen.findByRole("article", { name: "Version 4" });

  await act(() => router.navigate(pagePath(install.id)));
  await screen.findByRole("heading", { level: 1, name: "Install" });
  // The version's page is left once the turn is over.
  await act(() => new Promise((resolve) => setTimeout(resolve, 0)));
  gate.holding = true;
  await act(() => router.navigate(versionPath(install.id, 4)));
  await screen.findByRole("heading", { level: 1, name: "Version 4" });
  expect(screen.getByText("Loading…")).toBeTruthy();
  expect(screen.queryByText("· Ada · Edit")).toBeNull();

  await waitFor(() => expect(gate.held).toHaveLength(1));
  act(() => gate.held[0]?.());
  expect(await screen.findByText("· Ada · Edit")).toBeTruthy();
});
