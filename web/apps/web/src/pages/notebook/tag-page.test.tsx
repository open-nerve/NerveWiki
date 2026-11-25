import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { listColumnClass } from "../../app/page-classes";
import { pageTitleClass } from "../../app/settings-page";
import type { TreeNode } from "../../services/page.service";
import { appLinks } from "../../reading/app-links";
import { json, notebookJSON, problem } from "../../test/fakes";
import { guide, install, linux, notes, pageNode, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// A tag's pages (M6/P6 design 8).

const tagPath = (tag: string) => `/lab/notebooks/${notebookJSON.id}/tags/${encodeURIComponent(tag)}`;

/** The rows of the list of the tag's pages. */
const rows = (tag: string) => document.querySelectorAll(`ul[aria-label="Pages tagged #${tag}"] > li`).length;

test("a tag of a reading view leads to its pages, in the tree's order, each named as the tree names it", async () => {
  // A second Install, at the root: the tree tells the two apart.
  const other = pageNode(5, "Install");
  const server = pageServer({ nodes: [guide, install, linux, notes, other] });
  server.views.set(guide.id, {
    html: '<p><a class="nw-tag" data-nw-tag="Proj/a">#Proj/a</a></p>',
    revision: 1,
    assets_expire_at: null,
  });
  server.tags.set("Proj/a", [other.id, notes.id, linux.id, install.id]);
  const { router } = renderApp(pagePath(guide.id), server.app, { enhancements: [appLinks] });

  await userEvent.click(await screen.findByRole("link", { name: "#Proj/a" }));

  const heading = await screen.findByRole("heading", { level: 1, name: "#Proj/a" });
  expect(router.state.location.pathname).toBe(tagPath("Proj/a"));
  await waitFor(() => expect(document.activeElement).toBe(heading));
  const list = await screen.findByRole("list", { name: "Pages tagged #Proj/a" });
  const links = within(list).getAllByRole("link");
  expect(links.map((link) => link.textContent)).toEqual([
    "Install (in Guide)",
    "Linux",
    "Notes",
    "Install (at the notebook's top level)",
  ]);
  expect(links.map((link) => link.getAttribute("href"))).toEqual(
    [install, linux, notes, other].map((page) => pagePath(page.id))
  );
  expect(document.title).toBe("#Proj/a · Plans · Nerve Wiki");
  expect(server.sent).toContain("GET tag Proj/a");
});

test("a tag the page writes ending with '/' keeps it, its name one segment of the address", async () => {
  const server = pageServer();
  server.views.set(guide.id, {
    html: '<p><a class="nw-tag" data-nw-tag="a/">#a//</a></p>',
    revision: 1,
    assets_expire_at: null,
  });
  server.tags.set("a/", [notes.id]);
  const { router } = renderApp(pagePath(guide.id), server.app, { enhancements: [appLinks] });

  await userEvent.click(await screen.findByRole("link", { name: "#a//" }));

  expect(await screen.findByRole("heading", { level: 1, name: "#a/" })).toBeTruthy();
  expect(router.state.location.pathname).toBe(`/lab/notebooks/${notebookJSON.id}/tags/a%2F`);
  const list = await screen.findByRole("list", { name: "Pages tagged #a/" });
  expect(within(list).getByRole("link", { name: "Notes" })).toBeTruthy();
  expect(server.sent).toContain("GET tag a/");
});

test("a tag no page has says so, and a page the tree does not have is left out", async () => {
  const server = pageServer();
  server.tags.set("gone", ["0199a2b4-0000-7000-8000-0000000000f9"]);
  renderApp(tagPath("gone"), server.app);
  expect(await screen.findByText("No page has this tag.")).toBeTruthy();
  expect(screen.queryByRole("list", { name: "Pages tagged #gone" })).toBeNull();
});

test("a tag's pages that could not be read say why, and are read again on Try again", async () => {
  let fail = true;
  const server = pageServer({
    answers: {
      [`GET /api/v0/notebooks/${notebookJSON.id}/tags/*`]: () =>
        fail ? problem(500, "internal") : json({ data: [{ id: notes.id }] }),
    },
  });
  renderApp(tagPath("t"), server.app);

  const retry = await screen.findByRole("button", { name: "Try again" });
  fail = false;
  await userEvent.click(retry);

  const list = await screen.findByRole("list", { name: "Pages tagged #t" });
  expect(within(list).getByRole("link", { name: "Notes" })).toBeTruthy();
});

test("a tag's list, changed for another tag's of as many pages, renders its first rows again at once", async () => {
  // Two tags of 500 pages each, the ids of a length (M12/P4/S4 design 3.7).
  const pages: TreeNode[] = Array.from({ length: 1_000 }, (_, i) => ({
    ...pageNode(1, `p${String(i + 1).padStart(4, "0")}`),
    id: `0199a2b4-0000-7000-8001-${String(i + 1).padStart(12, "0")}`,
  }));
  const server = pageServer({ nodes: pages });
  server.tags.set(
    "a",
    pages.slice(0, 500).map((page) => page.id)
  );
  server.tags.set(
    "b",
    pages.slice(500).map((page) => page.id)
  );
  const wait = { timeout: 20_000 };
  const { router } = renderApp(tagPath("b"), server.app);
  await waitFor(() => expect(rows("b")).toBe(500), wait);
  await act(() => router.navigate(tagPath("a")));
  await waitFor(() => expect(rows("a")).toBe(500), wait);

  // Back to b, its pages at hand: its list's first change has its first rows only.
  let first: number | undefined;
  const observer = new MutationObserver(() => {
    if (first === undefined && rows("b") > 0) {
      first = rows("b");
    }
  });
  observer.observe(document.body, { childList: true, subtree: true });
  try {
    await act(() => router.navigate(tagPath("b")));
    await waitFor(() => expect(rows("b")).toBe(500), wait);
  } finally {
    observer.disconnect();
  }
  expect(first).toBeGreaterThan(0);
  expect(first).toBeLessThanOrEqual(100);
}, 30_000);

// M13/P5/S2 (decision 925; S1's handoff): a tag's pages are a list's page, 720px as the others, not the main's width;
// none, it says so with an icon.
test("a tag's pages are a list's page: its column and title the lists'; none, said with an icon", async () => {
  const server = pageServer();
  server.tags.set("plan", [guide.id]);
  renderApp(tagPath("plan"), server.app);
  const title = await screen.findByRole("heading", { level: 1, name: "#plan" });

  expect(title.className).toBe(pageTitleClass);
  expect(title.closest("section")?.className).toBe(listColumnClass);
  expect(title.closest("section")?.contains(await screen.findByRole("list", { name: "Pages tagged #plan" }))).toBe(
    true
  );
});

test("a tag no page has says so with an icon", async () => {
  renderApp(tagPath("none"), pageServer().app);
  const empty = await screen.findByText("No page has this tag.", { selector: "p" });
  expect(empty.previousElementSibling?.getAttribute("aria-hidden")).toBe("true");
  expect(empty.previousElementSibling?.querySelector("svg")).not.toBeNull();
});
