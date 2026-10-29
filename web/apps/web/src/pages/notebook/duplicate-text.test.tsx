import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { observer } from "mobx-react-lite";
import { runInAction } from "mobx";
import { useEffect, useState } from "react";
import type { RouteObject } from "react-router";
import { expect, test } from "vitest";

import { routes } from "../../app/routes";
import { ApiError } from "../../services/api";
import { usePageTree, useStore } from "../../stores/context";
import type { PageTreeStore } from "../../stores/page-tree.store";
import type { RootStore } from "../../stores/root.store";
import { json, notebookJSON, workspaceJSON, type Answer } from "../../test/fakes";
import { assetNode, notes, pageNode, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";
import { duplicateText, nodeTarget, sourceAtPath, useNodeNavigate, type DuplicateContext } from "./duplicate-text";
import { useNotebook } from "./notebook-layout";

const raw = pageNode(71, "raw");
const folder = pageNode(72, "Folder", raw);
const known = pageNode(73, "Known", folder);
const asset = assetNode(74, "known.png", folder);
const home = `/lab/notebooks/${notebookJSON.id}`;
const probePath = `${home}/duplicate-probe`;
const duplicate = (id?: string) =>
  new ApiError(409, { code: "page.source_duplicate", ...(id === undefined ? {} : { duplicate_of: id }) });

function sourceServer(answers: Record<string, Answer> = {}) {
  return pageServer({
    role: "admin",
    mode: "llm_wiki",
    sourceRootId: raw.id,
    nodes: [raw, folder, known, asset, notes],
    answers,
  });
}

function probe(
  server: ReturnType<typeof pageServer>,
  problem: unknown,
  { copies = 1, context, additional = [] }: { copies?: number; context?: DuplicateContext; additional?: unknown[] } = {}
) {
  let pages: PageTreeStore | undefined;
  let root: RootStore | undefined;
  const Probe = observer(function Probe() {
    const tree = usePageTree(useNotebook());
    const store = useStore();
    const navigate = useNodeNavigate(tree);
    const [version, renderAgain] = useState(0);
    useEffect(() => {
      pages = tree;
      root = store;
    }, [tree, store]);
    return (
      <section aria-label="Duplicate explanation">
        <button type="button" onClick={() => renderAgain(version + 1)}>
          Render again
        </button>
        {Array.from({ length: copies }, (_, index) => (
          <div key={index}>{duplicateText(problem, tree, navigate, context)}</div>
        ))}
        {additional.map((entry, index) => (
          <div key={`extra-${index.toString()}`}>{duplicateText(entry, tree, navigate, context)}</div>
        ))}
      </section>
    );
  });
  const withProbe = (entries: RouteObject[]): RouteObject[] =>
    entries.map((entry) =>
      entry.index === true
        ? entry
        : entry.path === "notebooks/:id"
          ? { ...entry, children: [...(entry.children ?? []), { path: "duplicate-probe", Component: Probe }] }
          : entry.children === undefined
            ? entry
            : { ...entry, children: withProbe(entry.children) }
    );
  const rendered = renderApp(pagePath(notes.id), server.app, { routes: withProbe(routes) });
  return {
    ...rendered,
    async open() {
      await screen.findByRole("navigation", { name: "Pages of Plans" });
      await screen.findByRole("heading", { level: 1, name: "Notes" });
      await act(() => rendered.router.navigate(probePath));
      return screen.findByRole("region", { name: "Duplicate explanation" });
    },
    stores() {
      if (pages === undefined || root === undefined) throw new Error("the duplicate probe is not mounted");
      return { pages, root };
    },
  };
}

test.each([
  { target: known, path: "raw/Folder/Known.md" },
  { target: asset, path: "raw/Folder/known.png" },
])(
  "duplicate targets include attachments and stale trees: $path opens the current node and focuses it",
  async ({ target, path }) => {
    const user = userEvent.setup();
    const server = sourceServer();
    const view = probe(server, duplicate(target.id));
    const section = await view.open();
    const link = await within(section).findByRole("link", { name: path });
    expect(link.getAttribute("href")).toBe(pagePath(target.kind === "page" ? target.id : folder.id));
    await user.click(link);
    expect(view.router.state.location.pathname).toBe(pagePath(target.kind === "page" ? target.id : folder.id));
    if (target.kind === "asset") {
      expect(view.router.state.location.state).toEqual({ attachment: target.id });
      await waitFor(() => expect(document.activeElement?.getAttribute("data-attachment")).toBe(target.id));
    } else {
      const heading = await screen.findByRole("heading", { level: 1, name: target.name });
      expect(view.router.state.location.state).toEqual({ arrived: true });
      await waitFor(() => expect(document.activeElement).toBe(heading));
    }
  }
);

test("duplicate targets include attachments and stale trees: no duplicate ID says the operation contains duplicates without a lookup", async () => {
  const server = sourceServer();
  const view = probe(server, duplicate());
  const section = await view.open();
  const before = server.sent.filter((line) => line === "GET nodes").length;
  expect(within(section).getByText("This operation contains sources with identical bytes.")).toBeTruthy();
  expect(within(section).getByText("This operation was not written.")).toBeTruthy();
  expect(within(section).queryByRole("link")).toBeNull();
  expect(section.textContent).not.toMatch(/[0-9a-f]{8}-[0-9a-f-]{27}/i);
  await userEvent.setup().click(within(section).getByRole("button", { name: "Render again" }));
  expect(server.sent.filter((line) => line === "GET nodes")).toHaveLength(before);
});

test("duplicate targets include attachments and stale trees: a report without duplicate_of never uses its refused node_id as the duplicate", async () => {
  const server = sourceServer();
  const view = probe(
    server,
    { code: "page.source_duplicate", node_id: asset.id, duplicate_of: null },
    { context: { kind: "background" } }
  );
  const section = await view.open();
  const before = server.sent.filter((line) => line === "GET nodes").length;
  expect(within(section).getByText("This operation contains sources with identical bytes.")).toBeTruthy();
  expect(within(section).getByText("This batch was not written; earlier completed batches are kept.")).toBeTruthy();
  expect(within(section).queryByRole("link")).toBeNull();
  expect(section.textContent).not.toContain(asset.id);
  await userEvent.setup().click(within(section).getByRole("button", { name: "Render again" }));
  expect(server.sent.filter((line) => line === "GET nodes")).toHaveLength(before);
});

test("duplicate targets include attachments and stale trees: a missing target is fetched once for all copies and appears at its full path", async () => {
  const server = sourceServer();
  server.nodes = server.nodes.filter((node) => node.id !== asset.id);
  const view = probe(server, duplicate(asset.id), { copies: 2 });
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  const before = server.sent.filter((line) => line === "GET nodes").length;
  server.nodes = [...server.nodes, asset];
  const section = await view.open();
  expect(await within(section).findAllByRole("link", { name: "raw/Folder/known.png" })).toHaveLength(2);
  expect(server.sent.filter((line) => line === "GET nodes")).toHaveLength(before + 1);
  await userEvent.setup().click(within(section).getByRole("button", { name: "Render again" }));
  expect(server.sent.filter((line) => line === "GET nodes")).toHaveLength(before + 1);
});

test("duplicate targets include attachments and stale trees: a still missing target is unavailable without repeated refreshes", async () => {
  const server = sourceServer();
  const missing = "0199a2b4-0000-7000-8000-000000000999";
  const view = probe(server, duplicate(missing), { copies: 2 });
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  const before = server.sent.filter((line) => line === "GET nodes").length;
  const section = await view.open();
  expect(await within(section).findAllByText("The duplicate source is currently unavailable.")).toHaveLength(2);
  await waitFor(() => expect(server.sent.filter((line) => line === "GET nodes")).toHaveLength(before + 1));
  const { pages } = view.stores();
  await act(() => pages.load());
  await userEvent.setup().click(within(section).getByRole("button", { name: "Render again" }));
  expect(server.sent.filter((line) => line === "GET nodes")).toHaveLength(before + 2);
  expect(section.textContent).not.toContain(missing);
  expect(within(section).queryByRole("link")).toBeNull();
});

test("duplicate targets include attachments and stale trees: a failed refresh is not repeated on remount or render", async () => {
  const user = userEvent.setup();
  const server = sourceServer();
  const view = probe(server, duplicate("0199a2b4-0000-7000-8000-000000000998"));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  const before = server.sent.filter((line) => line === "GET nodes").length;
  server.nodesDown = true;
  const section = await view.open();
  await within(section).findByText("The duplicate source is currently unavailable.");
  await waitFor(() => expect(server.sent.filter((line) => line === "GET nodes")).toHaveLength(before + 1));
  await user.click(within(section).getByRole("button", { name: "Render again" }));
  await act(() => view.router.navigate(pagePath(notes.id)));
  await view.open();
  expect(server.sent.filter((line) => line === "GET nodes")).toHaveLength(before + 1);
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => (resolve = yes));
  return { promise, resolve };
}

test("duplicate targets include attachments and stale trees: different missing targets share one in-flight tree read", async () => {
  const held = deferred<Response>();
  let holding = false;
  const server = sourceServer({
    [`GET /api/v0/notebooks/${notebookJSON.id}/nodes`]: () => {
      server.sent.push("GET nodes");
      return holding ? held.promise : json({ data: server.nodes });
    },
  });
  server.nodes = server.nodes.filter((node) => node.id !== known.id && node.id !== asset.id);
  const view = probe(server, duplicate(known.id), { additional: [duplicate(asset.id)] });
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  const before = server.sent.filter((line) => line === "GET nodes").length;
  holding = true;
  const section = await view.open();
  await waitFor(() => expect(server.sent.filter((line) => line === "GET nodes")).toHaveLength(before + 1));
  server.nodes = [...server.nodes, known, asset];
  await act(async () => held.resolve(json({ data: server.nodes })));
  await within(section).findByRole("link", { name: "raw/Folder/Known.md" });
  await within(section).findByRole("link", { name: "raw/Folder/known.png" });
  expect(server.sent.filter((line) => line === "GET nodes")).toHaveLength(before + 1);
});

test.each(["route change", "unmount"] as const)(
  "duplicate targets include attachments and stale trees: a click's refresh cannot navigate after %s",
  async (change) => {
    const held = deferred<Response>();
    let holding = false;
    const server = sourceServer({
      [`GET /api/v0/notebooks/${notebookJSON.id}/nodes`]: () => {
        server.sent.push("GET nodes");
        return holding ? held.promise : json({ data: server.nodes });
      },
    });
    const view = probe(server, duplicate(asset.id));
    const section = await view.open();
    const link = await within(section).findByRole("link", { name: "raw/Folder/known.png" });
    holding = true;
    const before = server.sent.filter((line) => line === "GET nodes").length;
    act(() => {
      runInAction(() => {
        view.stores().pages.nodes = server.nodes.filter((node) => node.id !== asset.id);
      });
      fireEvent.click(link);
    });
    await waitFor(() => expect(server.sent.filter((line) => line === "GET nodes")).toHaveLength(before + 1));
    if (change === "route change") await act(() => view.router.navigate(pagePath(notes.id)));
    else view.unmount();
    await act(async () => held.resolve(json({ data: server.nodes })));
    expect(view.router.state.location.pathname).toBe(change === "route change" ? pagePath(notes.id) : probePath);
  }
);

test("duplicate targets include attachments and stale trees: a retained attachment link uses its new parent inside sources", async () => {
  const server = sourceServer();
  const other = pageNode(79, "Other", raw);
  server.nodes.push(other);
  const view = probe(server, duplicate(asset.id));
  const section = await view.open();
  const link = await within(section).findByRole("link", { name: "raw/Folder/known.png" });
  server.nodes = server.nodes.map((node) => (node.id === asset.id ? { ...node, parent_id: other.id } : node));
  act(() => {
    runInAction(() => {
      view.stores().pages.nodes = server.nodes;
    });
    fireEvent.click(link);
  });
  await waitFor(() => expect(view.router.state.location.pathname).toBe(pagePath(other.id)));
  expect(view.router.state.location.state).toEqual({ attachment: asset.id });
  await waitFor(() => expect(document.activeElement?.getAttribute("data-attachment")).toBe(asset.id));
});

test.each(["background", "import"] as const)(
  "duplicate targets include attachments and stale trees: %s keeps the correct write scope",
  async (kind) => {
    const server = sourceServer();
    const issue =
      kind === "import"
        ? { code: "duplicate_source", path: "incoming.md", to: "raw/Folder/Known.md" }
        : duplicate(known.id);
    const view = probe(server, issue, { context: { kind } });
    const section = await view.open();
    await within(section).findByRole("link", { name: "raw/Folder/Known.md" });
    expect(within(section).queryByText("This operation was not written.")).toBeNull();
    if (kind === "import") {
      expect(
        within(section).getByText("incoming.md was skipped because identical source content already exists.")
      ).toBeTruthy();
      expect(section.textContent).not.toContain("This batch was not written");
    } else {
      expect(within(section).getByText("This batch was not written; earlier completed batches are kept.")).toBeTruthy();
    }
  }
);

test.each(["move outside", "downgrade", "delete"] as const)(
  "duplicate targets include attachments and stale trees: clicking a retained link after %s does not use its old target",
  async (change) => {
    const server = sourceServer();
    const view = probe(server, duplicate(asset.id));
    const section = await view.open();
    const link = await within(section).findByRole("link", { name: "raw/Folder/known.png" });
    if (change === "downgrade") server.notebook = { ...server.notebook, mode: "standard", source_root_id: null };
    else
      server.nodes =
        change === "delete"
          ? server.nodes.filter((node) => node.id !== asset.id)
          : server.nodes.map((node) => (node.id === folder.id ? { ...node, parent_id: null } : node));
    const { pages, root } = view.stores();
    const clicked = new MouseEvent("click", { bubbles: true, cancelable: true, button: 0 });
    act(() => {
      runInAction(() => {
        root.notebooksOf(workspaceJSON)!.receive(server.notebook);
        pages.nodes = server.nodes;
      });
      // React has not repainted: this is a connected link invoking its current handler against newer stores.
      expect(link.isConnected).toBe(true);
      fireEvent(link, clicked);
    });
    expect(clicked.defaultPrevented).toBe(true);
    expect(view.router.state.location.pathname).toBe(probePath);
    expect(await within(section).findByText("The duplicate source is currently unavailable.")).toBeTruthy();
  }
);

test("duplicate targets include attachments and stale trees: report paths resolve pages with .md and assets without guessing node IDs", async () => {
  const server = sourceServer();
  const doubled = pageNode(75, "with.md", folder);
  server.nodes.push(doubled);
  const view = probe(server, duplicate());
  await view.open();
  const { pages } = view.stores();
  expect(nodeTarget(pages, asset.id)?.path).toBe("raw/Folder/known.png");
  expect(sourceAtPath(pages, "raw/Folder/Known.md")?.node.id).toBe(known.id);
  expect(sourceAtPath(pages, "raw/Folder/with.md.md")?.node.id).toBe(doubled.id);
  expect(sourceAtPath(pages, "raw/Folder/known.png")?.node.id).toBe(asset.id);
  expect(sourceAtPath(pages, "raw/Folder/Known")).toBeUndefined();
  expect(sourceAtPath(pages, "../../elsewhere.md")).toBeUndefined();
});
