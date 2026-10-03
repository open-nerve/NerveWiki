import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import type { Notebook } from "../../services/notebook.service";
import { json, notebookJSON } from "../../test/fakes";
import { guide, install, linux, notes, pageNode, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The notebook's pages in the left column (M4/P5 design 3.6), and on its
// home (3.5).

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

test("the tree says so when it cannot be read; Try again reads it", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  server.nodesDown = true;
  renderApp(home, server.app);

  const nav = await tree();
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
