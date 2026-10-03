import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import type { Notebook } from "../../services/notebook.service";
import { json, notebookJSON, problem } from "../../test/fakes";
import { guide, install, linux, notes, pageNode, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The notebook's pages in the left column (M4/P5 design 3.6), and on its
// home (3.5).

afterEach(() => vi.useRealTimers());

const home = `/lab/notebooks/${notebookJSON.id}`;
const tree = () => screen.findByRole("navigation", { name: "Pages of Plans" });
/** The tree's links, each as its title. */
const titles = (nav: HTMLElement) =>
  within(nav)
    .getAllByRole("link")
    .map((link) => link.textContent);

test("the notebook's pages are in the left column, beside the main, their roots shown", async () => {
  renderApp(home, pageServer().app);

  const nav = await tree();
  expect(screen.getByRole("main").contains(nav)).toBe(false);
  await within(nav).findByRole("link", { name: "Guide" });
  expect(titles(nav)).toEqual(["Guide", "Notes"]);
  expect(within(nav).getByRole("link", { name: "Notes" }).getAttribute("href")).toBe(pagePath(notes.id));
  expect(within(nav).getByRole("button", { name: "Subpages of Guide" }).getAttribute("aria-expanded")).toBe("false");
  expect(within(nav).queryByRole("button", { name: "Subpages of Notes" })).toBeNull();
});

test("pages of the same title are told apart in their controls and dialogs by where they are", async () => {
  const user = userEvent.setup();
  const again = pageNode(5, "Notes", guide);
  renderApp(pagePath(again.id), pageServer({ nodes: [guide, install, linux, notes, again] }).app);

  const nav = await tree();
  await within(nav).findByRole("button", { name: "Actions for Notes (in Guide)" });
  expect(within(nav).getByRole("button", { name: "Actions for Notes (in Plans)" })).toBeTruthy();
  expect(within(nav).getByRole("button", { name: "Actions for Install" })).toBeTruthy();
  await user.click(within(nav).getByRole("button", { name: "Actions for Notes (in Guide)" }));
  await user.click(await screen.findByRole("menuitem", { name: "Rename" }));
  expect(await screen.findByRole("dialog", { name: "Rename Notes (in Guide)" })).toBeTruthy();
});

test("a page's subpages open and close", async () => {
  const user = userEvent.setup();
  renderApp(home, pageServer().app);
  const nav = await tree();
  const button = await within(nav).findByRole("button", { name: "Subpages of Guide" });

  await user.click(button);

  expect(button.getAttribute("aria-expanded")).toBe("true");
  const list = document.getElementById(button.getAttribute("aria-controls") ?? "");
  expect(
    list === null
      ? []
      : within(list)
          .getAllByRole("link")
          .map((link) => link.textContent)
  ).toEqual(["Install"]);
  await user.click(button);
  expect(button.getAttribute("aria-expanded")).toBe("false");
  expect(button.getAttribute("aria-controls")).toBeNull();
  expect(titles(nav)).toEqual(["Guide", "Notes"]);
});

test("the page shown is marked, its ancestors open", async () => {
  renderApp(pagePath(linux.id), pageServer().app);

  const nav = await tree();
  const link = await within(nav).findByRole("link", { name: "Linux" });
  expect(link.getAttribute("aria-current")).toBe("page");
  expect(titles(nav)).toEqual(["Guide", "Install", "Linux", "Notes"]);
  expect(within(nav).getByRole("link", { name: "Guide" }).getAttribute("aria-current")).toBeNull();
});

test("the tree says so when it cannot be read, in the left column and on the home; Try again reads it", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  server.nodesDown = true;
  renderApp(home, server.app);

  const nav = await tree();
  expect(await within(screen.getByRole("main")).findByRole("button", { name: "Try again" })).toBeTruthy();
  expect(within(screen.getByRole("main")).queryByRole("list", { name: "Pages" })).toBeNull();
  await user.click(await within(nav).findByRole("button", { name: "Try again" }));
  server.nodesDown = false;
  await user.click(within(nav).getByRole("button", { name: "Try again" }));

  expect(await within(nav).findByRole("link", { name: "Guide" })).toBeTruthy();
});

test("each notebook has its own tree, and keeps what is open", async () => {
  const user = userEvent.setup();
  const atlas: Notebook = { ...notebookJSON, id: "0199a2b4-0000-7000-8000-0000000000c2", name: "Atlas" };
  const maps = { ...pageNode(5, "Maps"), notebook_id: atlas.id };
  const server = pageServer({
    answers: {
      "GET /api/v0/workspaces/lab/notebooks": () => json({ data: [atlas, notebookJSON] }),
      [`GET /api/v0/notebooks/${atlas.id}/nodes`]: () => json({ data: [maps] }),
    },
  });
  renderApp(home, server.app);
  await user.click(await within(await tree()).findByRole("button", { name: "Subpages of Guide" }));

  await user.click(within(screen.getByRole("navigation", { name: "Lab" })).getByRole("link", { name: "Atlas" }));
  const atlasTree = await screen.findByRole("navigation", { name: "Pages of Atlas" });
  expect(await within(atlasTree).findByRole("link", { name: "Maps" })).toBeTruthy();
  expect(screen.queryByRole("navigation", { name: "Pages of Plans" })).toBeNull();

  await user.click(within(screen.getByRole("navigation", { name: "Lab" })).getByRole("link", { name: "Plans" }));
  expect(titles(await tree())).toEqual(["Guide", "Install", "Notes"]);
});

test("a write's failure in one notebook's tree is not carried into another's", async () => {
  const user = userEvent.setup();
  const atlas: Notebook = { ...notebookJSON, id: "0199a2b4-0000-7000-8000-0000000000c2", name: "Atlas" };
  const server = pageServer({
    answers: {
      "GET /api/v0/workspaces/lab/notebooks": () => json({ data: [atlas, notebookJSON] }),
      [`GET /api/v0/notebooks/${atlas.id}/nodes`]: () => json({ data: [] }),
      [`POST /api/v0/notebooks/${notebookJSON.id}/pages`]: () => problem(500, "internal_error"),
    },
  });
  renderApp(home, server.app);
  const plans = await tree();
  await user.click(await within(plans).findByRole("button", { name: "New page" }));
  expect(await within(plans).findByRole("alert")).toBeTruthy();

  await user.click(within(screen.getByRole("navigation", { name: "Lab" })).getByRole("link", { name: "Atlas" }));
  const atlasTree = await screen.findByRole("navigation", { name: "Pages of Atlas" });
  expect(within(atlasTree).queryByRole("alert")).toBeNull();
});

test("the notebook's home lists its root pages", async () => {
  renderApp(home, pageServer({ nodes: [guide, install, notes] }).app);

  const list = await within(screen.getByRole("main")).findByRole("list", { name: "Pages" });
  expect(
    within(list)
      .getAllByRole("link")
      .map((link) => link.textContent)
  ).toEqual(["Guide", "Notes"]);
  expect(within(list).getByRole("link", { name: "Guide" }).getAttribute("href")).toBe(pagePath(guide.id));
  expect(screen.queryByText("No pages yet.")).toBeNull();
});

test("the tree read again shows another tab's changes, in the left column and on the home", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer();
  renderApp(home, server.app);
  const nav = await tree();
  await within(nav).findByRole("link", { name: "Guide" });

  server.nodes = [{ ...guide, name: "Handbook" }, install, linux, notes, pageNode(5, "Atlas")];
  await act(() => vi.advanceTimersByTimeAsync(6_000));
  act(() => void window.dispatchEvent(new Event("focus")));

  await waitFor(() => expect(titles(nav)).toEqual(["Handbook", "Notes", "Atlas"]));
  const list = within(screen.getByRole("main")).getByRole("list", { name: "Pages" });
  expect(
    within(list)
      .getAllByRole("link")
      .map((link) => link.textContent)
  ).toEqual(["Handbook", "Notes", "Atlas"]);
});
