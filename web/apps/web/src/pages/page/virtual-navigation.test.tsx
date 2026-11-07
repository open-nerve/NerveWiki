import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, onTestFinished, test, vi } from "vitest";

import { eventHandlers } from "../../events/handlers";
import { appLinks } from "../../reading/app-links";
import { unresolvedLinks } from "../../reading/unresolved-links";
import type { NotebookRole } from "../../services/notebook.service";
import { PageTreeStore } from "../../stores/page-tree.store";
import { eventServer, withEvents } from "../../test/event-server";
import { json, notebookJSON, workspaceJSON } from "../../test/fakes";
import { wrote } from "../../test/history-server";
import { indexFacts } from "../../test/index-server";
import { guide, install, notes, pageNode, pagePath, pageServer, versionJSON } from "../../test/page-server";
import { renderApp } from "../../test/render";

const home = `/lab/notebooks/${notebookJSON.id}`;
const raw = pageNode(70, "raw");
const agents = pageNode(71, "AGENTS");
const actualIndex = pageNode(72, "index");
const enhancements = [appLinks, unresolvedLinks];
const link = (target: string, kind = "nw-wikilink nw-unresolved") =>
  `<a class="${kind}" data-nw-target="${target}">open target</a>`;

function wiki(role: NotebookRole = "reader", mode: "standard" | "llm_wiki" = "llm_wiki") {
  const server = pageServer({
    role,
    mode,
    sourceRootId: mode === "llm_wiki" ? raw.id : null,
    nodes: [guide, install, notes, raw, agents, actualIndex],
    answers: {
      [`GET /api/v0/notebooks/${notebookJSON.id}/index`]: () => json({ ...indexFacts(), mode }),
    },
  });
  server.log("existing", { message: "A real history row", changes: [wrote(guide, 1, 2)] });
  return server;
}

test("P4/W01 virtual_rows: ordinary links leave the real store intact and disappear on the same event connection", async () => {
  const observed = vi.spyOn(PageTreeStore.prototype, "childrenOf");
  onTestFinished(() => observed.mockRestore());
  const events = eventServer();
  const server = pageServer({
    mode: "llm_wiki",
    sourceRootId: raw.id,
    nodes: [raw, agents, guide, install],
    answers: { "GET /api/v0/events": events.answer },
  });
  renderApp(home, withEvents(server.app), { eventHandlers });
  const nav = await screen.findByRole("navigation", { name: "Pages of Plans" });
  await within(nav).findByRole("link", { name: "raw (source area)" });
  const directory = within(nav).getByRole("link", { name: "Directory" });
  const history = within(nav).getByRole("link", { name: "History" });
  expect([directory.getAttribute("href"), history.getAttribute("href")]).toEqual([`${home}/index`, `${home}/history`]);
  expect(directory.closest("[draggable]")).toBeNull();
  expect(history.closest("[draggable]")).toBeNull();
  expect(within(nav).queryByRole("button", { name: "Actions for Directory" })).toBeNull();
  expect(within(nav).queryByRole("button", { name: "Actions for History" })).toBeNull();
  const actualTree = observed.mock.contexts.find(
    (tree): tree is PageTreeStore => tree instanceof PageTreeStore && tree.notebookId === notebookJSON.id
  );
  expect(actualTree).toBeDefined();
  const expectedIDs = [raw.id, agents.id, guide.id, install.id];
  expect(actualTree?.nodes?.map((node) => node.id)).toEqual(expectedIDs);
  await waitFor(() => expect(events.streams).toHaveLength(1));
  const stream = events.last();
  act(() => stream.hello());
  await waitFor(() => expect(server.sent.filter((each) => each === "GET nodes").length).toBeGreaterThanOrEqual(2));
  server.notebook = { ...server.notebook, mode: "standard", source_root_id: null };
  act(() => stream.send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id }));
  await waitFor(() => expect(within(nav).queryByRole("link", { name: "Directory" })).toBeNull());
  expect(within(nav).queryByRole("link", { name: "History" })).toBeNull();
  expect(within(nav).getByRole("link", { name: "raw" })).toBeTruthy();
  expect(within(nav).getByRole("link", { name: "AGENTS" })).toBeTruthy();
  expect(actualTree?.nodes?.map((node) => node.id)).toEqual(expectedIDs);
  expect(events.streams).toEqual([stream]);
  expect(stream.ended).toBe(false);
});

test.each([
  ["reader", false, "index", "index", "Directory"],
  ["admin", true, "/LOG.MD", "history", "History"],
  ["editor", false, "log.md.md", "history", "History"],
] as const)(
  "P4/W02 reader_virtual: %s readOnly=%s opens %s without landing or creation",
  async (role, readOnly, target, route, title) => {
    const server = wiki(role);
    const html = `<p>${link(target)}</p>`;
    let path = pagePath(guide.id);
    if (readOnly) {
      server.withVersions(guide.id, [versionJSON(1, null, "old\n"), versionJSON(2, 1, "new\n")]);
      server.versionViews.set(`${guide.id} 1`, { html, revision: 1, assets_expire_at: null });
      path += "/versions/1";
    } else {
      server.views.set(guide.id, { html, revision: 1, assets_expire_at: null });
    }
    const { router } = renderApp(path, server.app, { enhancements });
    const button = await screen.findByRole("button", { name: "open target" });
    expect(button.tagName).toBe("A");
    await userEvent.click(button);
    await waitFor(() => expect(router.state.location.pathname).toBe(`${home}/${route}`));
    const heading = await screen.findByRole("heading", { level: 1, name: title });
    if (route === "index") expect(await screen.findByRole("link", { name: "Alpha" })).toBeTruthy();
    else expect(await screen.findByText("A real history row")).toBeTruthy();
    await waitFor(() => expect(document.activeElement).toBe(heading));
    expect(server.sent.filter((each) => each.startsWith("GET landing") || each.startsWith("POST"))).toEqual([]);
    expect(screen.queryByRole("alertdialog")).toBeNull();
  }
);

test("P4/W02 reader_virtual: a relative target that reaches the root navigates from its actual parent chain", async () => {
  const server = wiki();
  server.views.set(install.id, { html: `<p>${link("../INDEX.md.md")}</p>`, revision: 1, assets_expire_at: null });
  const { router } = renderApp(pagePath(install.id), server.app, { enhancements });
  await within(await screen.findByRole("navigation", { name: "Pages of Plans" })).findByRole("link", {
    name: "Install",
  });
  await userEvent.click(await screen.findByRole("button", { name: "open target" }));
  await screen.findByRole("link", { name: "Alpha" });
  expect(router.state.location.pathname).toBe(`${home}/index`);
  expect(server.sent.filter((each) => each.startsWith("GET landing"))).toEqual([]);
});

test("P4/W02 reader_virtual: a resolved real index keeps its real node destination", async () => {
  const server = wiki();
  server.views.set(guide.id, {
    html: `<p><a class="nw-wikilink" data-nw-target="index" data-nw-node="${actualIndex.id}">actual index</a></p>`,
    revision: 1,
    assets_expire_at: null,
  });
  const { router } = renderApp(pagePath(guide.id), server.app, { enhancements });
  const resolved = await screen.findByRole("link", { name: "actual index" });
  expect(resolved.getAttribute("href")).toBe(pagePath(actualIndex.id));
  await userEvent.click(resolved);
  await screen.findByRole("heading", { level: 1, name: "index" });
  expect(router.state.location.pathname).toBe(pagePath(actualIndex.id));
  expect(server.sent.filter((each) => each.startsWith("GET landing") || each.startsWith("POST"))).toEqual([]);
});

test.each([
  ["standard", "index", "link", guide],
  ["llm_wiki", "notes/index", "link", guide],
  ["llm_wiki", "./index", "link", install],
  ["llm_wiki", "../log", "link", guide],
  ["llm_wiki", "ındex", "link", guide],
  ["llm_wiki", "index.md.md.md", "link", guide],
  ["llm_wiki", "index", "embed", guide],
  ["llm_wiki", "log", "image", guide],
] as const)("P4/W02 guards: %s %s %s keeps the original unresolved behavior", async (mode, target, kind, page) => {
  const server = wiki("reader", mode);
  const html =
    kind === "image"
      ? `<span class="nw-image">${link(target)}</span>`
      : link(target, `nw-wikilink nw-unresolved${kind === "embed" ? " nw-embed" : ""}`);
  server.views.set(page.id, { html: `<p>${html}</p>`, revision: 1, assets_expire_at: null });
  const { router } = renderApp(pagePath(page.id), server.app, { enhancements });
  await within(await screen.findByRole("navigation", { name: "Pages of Plans" })).findByRole("link", {
    name: page.name,
  });
  await userEvent.click(await screen.findByRole("button", { name: "open target" }));
  expect(await screen.findByRole("alertdialog", { name: `“${target}” does not exist` })).toBeTruthy();
  expect(router.state.location.pathname).toBe(pagePath(page.id));
  expect(server.sent.filter((each) => each.startsWith("GET landing") || each.startsWith("POST"))).toEqual([]);
});
