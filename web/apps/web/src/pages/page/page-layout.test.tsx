import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { json, notebookJSON, problem } from "../../test/fakes";
import { guide, install, linux, notes, pageNode, pagePath, pageServer } from "../../test/page-server";
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
  expect((await within(main()).findByRole("article")).innerHTML).toBe("<p>Install</p>");
  const subpages = within(main()).getByRole("list", { name: "Subpages" });
  expect(within(subpages).getByRole("link", { name: "Linux" }).getAttribute("href")).toBe(pagePath(linux.id));
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
          : json({ html: "<p>Read at last</p>", revision: 1 }),
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

  server.views.set(install.id, { html: "<p>Install, changed</p>", revision: 2 });
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
