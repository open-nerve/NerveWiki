import type { monitorForElements } from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect } from "react";
import { Outlet } from "react-router";
import { expect, test, vi } from "vitest";

import { routes } from "../../app/routes";
import { useStore } from "../../stores/context";
import type { RootStore } from "../../stores/root.store";
import { notebookJSON, workspaceJSON, problem, json } from "../../test/fakes";
import { assetNode, guide, install, notes, pageNode, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

type Monitor = Parameters<typeof monitorForElements>[0];
const monitors = vi.hoisted((): Monitor[] => []);
vi.mock("@atlaskit/pragmatic-drag-and-drop/element/adapter", async (original) => ({
  ...(await original<typeof import("@atlaskit/pragmatic-drag-and-drop/element/adapter")>()),
  monitorForElements: (monitor: Monitor) => {
    monitors.push(monitor);
    return () => {};
  },
}));
vi.mock("@atlaskit/pragmatic-drag-and-drop-hitbox/list-item", async (original) => ({
  ...(await original<typeof import("@atlaskit/pragmatic-drag-and-drop-hitbox/list-item")>()),
  extractInstruction: () => ({ operation: "combine", blocked: false }),
}));

const tree = () => screen.findByRole("navigation", { name: "Pages of Plans" });
const moves = (server: ReturnType<typeof pageServer>) => server.sent.filter((line) => line.startsWith("MOVE"));
function renderSourceTree(server: ReturnType<typeof pageServer>) {
  let store: RootStore;
  function Capture() {
    const current = useStore();
    useEffect(() => {
      store = current;
    }, [current]);
    return <Outlet />;
  }
  return {
    ...renderApp(pagePath(notes.id), server.app, { routes: [{ Component: Capture, children: routes }] }),
    refresh: () =>
      act(async () => {
        await store.notebooksOf(workspaceJSON)?.load();
        await store.pagesOf(server.notebook)?.load();
      }),
    queueCreate: () => store.pagesOf(server.notebook)!.create(null, "Busy"),
  };
}

async function moveDialog(user: ReturnType<typeof userEvent.setup>, parent: string) {
  await user.click(await within(await tree()).findByRole("button", { name: "Actions for Notes" }));
  await user.click(await screen.findByRole("menuitem", { name: "Move to…" }));
  const dialog = await screen.findByRole("dialog", { name: "Move Notes" });
  await user.selectOptions(within(dialog).getByLabelText("Parent page"), parent);
  await user.click(within(dialog).getByRole("button", { name: "Move" }));
  return dialog;
}
async function drop(id: string, onto: string, monitor = monitors.at(-1)) {
  expect(monitor?.onDrop).toBeTypeOf("function");
  await act(async () =>
    monitor?.onDrop?.({
      source: { data: { page: id, notebook: notebookJSON.id } },
      location: { current: { dropTargets: [{ data: { page: onto } }] } },
    } as never)
  );
}

test.each(["editor", "admin"] as const)(
  "source management follows the current %s role for deep pages",
  async (role) => {
    const server = pageServer({ role, mode: "llm_wiki", sourceRootId: guide.id });
    renderApp(pagePath(install.id), server.app);
    const nav = await tree();
    await userEvent.setup().click(await within(nav).findByRole("button", { name: "Actions for Install" }));
    expect(
      within(await screen.findByRole("menu"))
        .getAllByRole("menuitem")
        .map((item) => item.textContent)
    ).toEqual(
      role === "admin"
        ? ["Paste Markdown", "Upload files", "Rename", "Move to…", "Delete"]
        : ["Paste Markdown", "Upload files"]
    );
    await userEvent.setup().keyboard("{Escape}");
    const link = within(nav).getByRole("link", { name: "Install (source)" });
    expect(link.parentElement?.hasAttribute("draggable")).toBe(role === "admin");
  }
);

test.each(["dialog", "drag"] as const)("writer moving a subtree into sources confirms once: %s", async (entry) => {
  const user = userEvent.setup();
  const child = pageNode(30, "Child", notes);
  const server = pageServer({
    role: "editor",
    mode: "llm_wiki",
    sourceRootId: guide.id,
    nodes: [guide, install, notes, child],
  });
  renderApp(pagePath(notes.id), server.app);
  const nav = await tree();
  await within(nav).findByRole("button", { name: "Actions for Notes" });
  async function attempt() {
    if (entry === "drag") await drop(notes.id, install.id);
    else {
      if (screen.queryByRole("dialog", { name: "Move Notes" }) === null) {
        await user.click(within(nav).getByRole("button", { name: "Actions for Notes" }));
        await user.click(await screen.findByRole("menuitem", { name: "Move to…" }));
      }
      const dialog = screen.getByRole("dialog", { name: "Move Notes" });
      await user.selectOptions(within(dialog).getByLabelText("Parent page"), install.id);
      await user.click(within(dialog).getByRole("button", { name: "Move" }));
    }
  }
  await attempt();
  let confirmation = await screen.findByRole("alertdialog", { name: "Move into sources?" });
  expect(confirmation.textContent).toContain(
    "After moving, you cannot edit the content or manage these sources. An admin can move them out."
  );
  expect(moves(server)).toEqual([]);
  await user.click(within(confirmation).getByRole("button", { name: "Cancel" }));
  expect(moves(server)).toEqual([]);
  expect(server.nodes.find((node) => node.id === notes.id)?.parent_id).toBeNull();
  await attempt();
  confirmation = await screen.findByRole("alertdialog", { name: "Move into sources?" });
  expect(screen.getAllByRole("alertdialog")).toHaveLength(1);
  await user.click(within(confirmation).getByRole("button", { name: "Move" }));
  await waitFor(() => expect(server.nodes.find((node) => node.id === notes.id)?.parent_id).toBe(install.id));
  expect(moves(server)).toEqual(["MOVE Notes under Install, after last"]);
  expect(server.nodes.find((node) => node.id === child.id)?.parent_id).toBe(notes.id);
});

test("a retained tree drag cannot move an existing writer source", async () => {
  const server = pageServer({ role: "editor", mode: "llm_wiki", sourceRootId: guide.id });
  renderApp(pagePath(install.id), server.app);
  await within(await tree()).findByRole("button", { name: "Actions for Install" });
  await drop(install.id, notes.id);
  expect(moves(server)).toEqual([]);
  expect(screen.queryByRole("alertdialog")).toBeNull();
});

test.each(["admin", "protected-root"] as const)("moving without a source confirmation: %s", async (kind) => {
  const server = pageServer({ role: kind === "admin" ? "admin" : "editor", mode: "llm_wiki", sourceRootId: guide.id });
  renderApp(pagePath(notes.id), server.app);
  await within(await tree()).findByRole("button", { name: "Actions for Notes" });
  await drop(kind === "admin" ? notes.id : guide.id, kind === "admin" ? guide.id : notes.id);
  if (kind === "admin") await waitFor(() => expect(moves(server)).toHaveLength(1));
  // The writer's source root is independently protected, even at an ordinary destination.
  else expect(moves(server)).toEqual([]);
  expect(screen.queryByRole("alertdialog")).toBeNull();
});

test("a writer's ordinary move remains immediate", async () => {
  const parent = pageNode(30, "Outside");
  const server = pageServer({
    role: "editor",
    mode: "llm_wiki",
    sourceRootId: guide.id,
    nodes: [guide, install, notes, parent],
  });
  renderApp(pagePath(notes.id), server.app);
  await within(await tree()).findByRole("button", { name: "Actions for Notes" });
  await drop(notes.id, parent.id);
  await waitFor(() => expect(moves(server)).toEqual(["MOVE Notes under Outside, after last"]));
  expect(screen.queryByRole("alertdialog")).toBeNull();
});

test.each(
  (["drag", "dialog"] as const).flatMap((entry) =>
    (["route", "role", "mode", "root", "node-parent", "destination-parent", "destination-gone"] as const).map(
      (change) => ({ entry, change })
    )
  )
)("a pending $entry is invalidated by $change without a write", async ({ entry, change }) => {
  const user = userEvent.setup();
  const outside = pageNode(30, "Outside");
  const server = pageServer({
    role: "editor",
    mode: "llm_wiki",
    sourceRootId: guide.id,
    nodes: [guide, install, notes, outside],
  });
  const view = renderSourceTree(server);
  await within(await tree()).findByRole("button", { name: "Actions for Notes" });
  if (entry === "drag") await drop(notes.id, install.id);
  else await moveDialog(user, install.id);
  await screen.findByRole("alertdialog", { name: "Move into sources?" });
  expect(moves(server)).toEqual([]);

  if (change === "route") await act(() => view.router.navigate(pagePath(guide.id)));
  else {
    if (change === "role") server.notebook = { ...server.notebook, role: "reader" };
    if (change === "mode") server.notebook = { ...server.notebook, mode: "standard", source_root_id: null };
    if (change === "root") server.notebook = { ...server.notebook, source_root_id: outside.id };
    if (change === "node-parent")
      server.nodes = server.nodes.map((node) => (node.id === notes.id ? { ...node, parent_id: outside.id } : node));
    if (change === "destination-parent")
      server.nodes = server.nodes.map((node) => (node.id === install.id ? { ...node, parent_id: null } : node));
    if (change === "destination-gone") server.nodes = server.nodes.filter((node) => node.id !== install.id);
    await view.refresh();
  }
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(moves(server)).toEqual([]);
});

test.each(["Rename", "Delete"] as const)(
  "an open page %s rechecks permissions when a page becomes the source root",
  async (action) => {
    const user = userEvent.setup();
    const server = pageServer({ role: "editor", mode: "llm_wiki", sourceRootId: guide.id });
    const view = renderSourceTree(server);
    await user.click(await within(await tree()).findByRole("button", { name: "Actions for Notes" }));
    await user.click(await screen.findByRole("menuitem", { name: action }));
    const dialog = await screen.findByRole(action === "Rename" ? "dialog" : "alertdialog");
    if (action === "Rename") {
      await user.clear(within(dialog).getByLabelText("Title"));
      await user.type(within(dialog).getByLabelText("Title"), "My original");
    }
    // Changing the source root preserves this row and its held dialog, unlike moving the row itself.
    server.notebook = { ...server.notebook, source_root_id: notes.id };
    await view.refresh();
    await user.click(within(dialog).getByRole("button", { name: action === "Rename" ? "Save" : "Delete" }));
    await within(dialog).findByRole("alert");
    expect(server.sent.filter((line) => /^(PATCH|DELETE) /.test(line))).toEqual([]);
    expect(dialog.isConnected).toBe(true);
  }
);

test("a source move refusal keeps the chosen parent and position in its form", async () => {
  const user = userEvent.setup();
  const server = pageServer({
    role: "editor",
    mode: "llm_wiki",
    sourceRootId: guide.id,
    answers: {
      "POST /api/v0/nodes/*/move": () => {
        server.sent.push("MOVE refused");
        return problem(409, "page.source_too_large");
      },
    },
  });
  renderSourceTree(server);
  const dialog = await moveDialog(user, install.id);
  const confirmation = await screen.findByRole("alertdialog", { name: "Move into sources?" });
  await user.click(within(confirmation).getByRole("button", { name: "Move" }));
  await within(dialog).findByRole("alert");
  expect((within(dialog).getByLabelText("Parent page") as HTMLSelectElement).value).toBe(install.id);
  expect((within(dialog).getByLabelText("Position") as HTMLSelectElement).value).toBe("last");
  expect(moves(server)).toEqual(["MOVE refused"]);
  expect(server.nodes.find((node) => node.id === notes.id)?.parent_id).toBeNull();
});

test("a duplicate tree drop link releases the sidebar focus to the existing source attachment", async () => {
  const user = userEvent.setup();
  const known = assetNode(42, "known.png", install);
  const copy = assetNode(43, "copy.png", notes);
  const server = pageServer({
    role: "editor",
    mode: "llm_wiki",
    sourceRootId: guide.id,
    nodes: [guide, install, notes, known, copy],
  });
  server.sourceAssetBytes.set(known.id, "89504e4700");
  server.sourceAssetBytes.set(copy.id, "89504e4700");
  const before = [...server.nodes];
  const view = renderSourceTree(server);
  const nav = await tree();
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  await drop(notes.id, guide.id);
  const confirmation = await screen.findByRole("alertdialog", { name: "Move into sources?" });
  await user.click(within(confirmation).getByRole("button", { name: "Move" }));
  const alert = await within(nav).findByRole("alert");
  const link = within(alert).getByRole("link", { name: "Guide/Install/known.png" });
  expect(moves(server)).toEqual(["MOVE Notes under Guide, after last"]);
  expect(server.nodes).toEqual(before);

  await user.click(link);
  await waitFor(() => expect(view.router.state.location.pathname).toBe(pagePath(install.id)));
  expect(view.router.state.location.state).toEqual({ attachment: known.id });
  await waitFor(() => expect(document.activeElement?.getAttribute("data-attachment")).toBe(known.id));
  expect(within(nav).queryByRole("alert")).toBeNull();
  expect(server.nodes).toEqual(before);
  expect(moves(server)).toHaveLength(1);
});

test.each(["drag", "dialog"] as const)(
  "repeated $entry submissions while confirming or sending write only once",
  async (entry) => {
    const user = userEvent.setup();
    let release!: () => void;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    const server = pageServer({
      role: "editor",
      mode: "llm_wiki",
      sourceRootId: guide.id,
      answers: {
        "POST /api/v0/nodes/*/move": async () => {
          server.sent.push("MOVE Notes pending");
          await gate;
          const moved = { ...notes, parent_id: install.id };
          server.nodes = server.nodes.map((node) => (node.id === notes.id ? moved : node));
          return json(moved);
        },
      },
    });
    renderSourceTree(server);
    await within(await tree()).findByRole("button", { name: "Actions for Notes" });
    let form: HTMLFormElement | null = null;
    if (entry === "drag") {
      await drop(notes.id, install.id);
      await drop(notes.id, install.id);
    } else {
      const dialog = await moveDialog(user, install.id);
      form = dialog.querySelector("form");
      expect(form).not.toBeNull();
      fireEvent.submit(form!);
    }
    expect(moves(server)).toEqual([]);
    const confirmation = await screen.findByRole("alertdialog", { name: "Move into sources?" });
    const button = within(confirmation).getByRole("button", { name: "Move" });
    await act(async () => {
      fireEvent.click(button);
      fireEvent.click(button);
    });
    await waitFor(() => expect(moves(server)).toEqual(["MOVE Notes pending"]));
    if (entry === "drag") await drop(notes.id, install.id);
    else fireEvent.submit(form!);
    expect(moves(server)).toEqual(["MOVE Notes pending"]);
    await act(async () => {
      release();
      await gate;
    });
    await waitFor(() => expect(server.nodes.find((node) => node.id === notes.id)?.parent_id).toBe(install.id));
    expect(moves(server)).toHaveLength(1);
  }
);

test.each(["route", "role"] as const)(
  "a confirmed move still queued behind another write rechecks %s",
  async (change) => {
    const user = userEvent.setup();
    let release!: () => void;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    const server = pageServer({
      role: "editor",
      mode: "llm_wiki",
      sourceRootId: guide.id,
      answers: {
        [`POST /api/v0/notebooks/${notebookJSON.id}/pages`]: async () => {
          server.sent.push("POST held");
          await gate;
          return problem(409, "page.title_taken");
        },
      },
    });
    const view = renderSourceTree(server);
    await within(await tree()).findByRole("button", { name: "Actions for Notes" });
    const first = view.queueCreate().catch(() => undefined);
    await waitFor(() => expect(server.sent).toContain("POST held"));
    await drop(notes.id, install.id);
    const confirmation = await screen.findByRole("alertdialog", { name: "Move into sources?" });
    await user.click(within(confirmation).getByRole("button", { name: "Move" }));
    expect(moves(server)).toEqual([]);
    if (change === "route") await act(() => view.router.navigate(pagePath(guide.id)));
    else {
      server.notebook = { ...server.notebook, role: "reader" };
      await view.refresh();
    }
    const reads = server.sent.filter((line) => line === "GET nodes").length;
    await act(async () => {
      release();
      await first;
    });
    await waitFor(() =>
      expect(server.sent.filter((line) => line === "GET nodes").length).toBeGreaterThanOrEqual(reads + 2)
    );
    expect(moves(server)).toEqual([]);
    expect(server.nodes.find((node) => node.id === notes.id)?.parent_id).toBeNull();
  }
);

test("a late rejected drag does not show its refusal after leaving the initiating page", async () => {
  const user = userEvent.setup();
  let release!: () => void;
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  const server = pageServer({
    role: "editor",
    mode: "llm_wiki",
    sourceRootId: guide.id,
    answers: {
      "POST /api/v0/nodes/*/move": async () => {
        server.sent.push("MOVE Notes pending");
        await gate;
        return problem(409, "page.source_too_large");
      },
    },
  });
  const view = renderSourceTree(server);
  await within(await tree()).findByRole("button", { name: "Actions for Notes" });
  await drop(notes.id, install.id);
  const confirmation = await screen.findByRole("alertdialog", { name: "Move into sources?" });
  await user.click(within(confirmation).getByRole("button", { name: "Move" }));
  await waitFor(() => expect(moves(server)).toEqual(["MOVE Notes pending"]));
  await act(() => view.router.navigate(pagePath(guide.id)));
  const reads = server.sent.filter((line) => line === "GET nodes").length;
  await act(async () => {
    release();
    await gate;
  });
  await waitFor(() => expect(server.sent.filter((line) => line === "GET nodes").length).toBeGreaterThan(reads));
  expect(within(await tree()).queryByRole("alert")).toBeNull();
  expect(view.router.state.location.pathname).toBe(pagePath(guide.id));
});
