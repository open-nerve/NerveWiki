import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect } from "react";
import { Outlet } from "react-router";
import { expect, onTestFinished, test, vi } from "vitest";

import { routes } from "../../app/routes";
import type { Enhancement, ReadingContext } from "../../reading/enhancement";
import { taskToggle } from "../../reading/task-toggle";
import { unresolvedLinks } from "../../reading/unresolved-links";
import { PageService } from "../../services/page.service";
import { useStore } from "../../stores/context";
import { EditSession } from "../../stores/edit-session";
import type { RootStore } from "../../stores/root.store";
import { attachments, picker } from "../../test/attachments";
import { workspaceJSON } from "../../test/fakes";
import { pageEditor } from "../../test/page-editor";
import { ada, guide, install, notes, pageNode, pagePath, pageServer, versionJSON } from "../../test/page-server";
import { renderApp } from "../../test/render";

const agents = pageNode(20, "AGENTS");
const entries = ["Edit", "Mod+E", "task", "restore", "edit-here"] as const;
const changes = ["mode", "root", "tree", "role"] as const;
type CapabilityChange = (typeof changes)[number];
type PageServer = ReturnType<typeof pageServer>;

/** Observe the actual login's root, leaving the application's routes and providers intact. */
function capturedPage(path: string, server: PageServer, enhancements: readonly Enhancement[] = [taskToggle]) {
  let captured: RootStore | undefined;
  function Capture() {
    const root = useStore();
    useEffect(() => {
      captured = root;
    }, [root]);
    return <Outlet />;
  }
  const rendered = renderApp(path, server.app, { routes: [{ Component: Capture, children: routes }], enhancements });
  return {
    ...rendered,
    stores() {
      const root = captured;
      const notebooks = root?.notebooksOf(workspaceJSON);
      const pages = root?.pagesOf(server.notebook);
      if (root === undefined || notebooks === undefined || pages === undefined) {
        throw new Error("the page has no signed-in stores");
      }
      return { root, notebooks, pages };
    },
  };
}

/** Each change starts with the same editable page, then independently removes its body capability. */
function changingServer(change: CapabilityChange): PageServer {
  const server = pageServer({
    role: "editor",
    mode: change === "root" || change === "tree" ? "llm_wiki" : "standard",
    sourceRootId: change === "root" ? notes.id : change === "tree" ? guide.id : null,
  });
  if (change === "tree") {
    server.nodes = server.nodes.map((node) => (node.id === install.id ? { ...node, parent_id: notes.id } : node));
  }
  return server;
}

async function refreshCapability(
  server: PageServer,
  captured: ReturnType<typeof capturedPage>,
  change: CapabilityChange,
  blocked: boolean
): Promise<void> {
  const { pages, notebooks } = captured.stores();
  switch (change) {
    case "mode":
      server.notebook = {
        ...server.notebook,
        mode: blocked ? "llm_wiki" : "standard",
        source_root_id: blocked ? guide.id : null,
      };
      break;
    case "root":
      server.notebook = { ...server.notebook, source_root_id: blocked ? guide.id : notes.id };
      break;
    case "tree":
      server.nodes = server.nodes.map((node) =>
        node.id === install.id ? { ...node, parent_id: blocked ? guide.id : notes.id } : node
      );
      break;
    case "role":
      server.notebook = { ...server.notebook, role: blocked ? "reader" : "editor" };
      break;
  }
  await act(async () => {
    if (change === "tree") await pages.load();
    else await notebooks.load();
  });
  expect(captured.stores().pages).toBe(pages);
}

/** Receive the new notebook synchronously, while a handler from the preceding render is still attached. */
function blockBeforeRender(server: PageServer, captured: ReturnType<typeof capturedPage>, change: "source" | "role") {
  server.notebook =
    change === "source"
      ? { ...server.notebook, mode: "llm_wiki", source_root_id: guide.id }
      : { ...server.notebook, role: "reader" };
  captured.stores().notebooks.receive(server.notebook);
  expect(captured.stores().pages.notebook).toMatchObject(
    change === "source" ? { mode: "llm_wiki", source_root_id: guide.id } : { role: "reader" }
  );
}

const bodyWrites = (sent: readonly string[]) => sent.filter((line) => /^(OPEN|PUT|TOGGLE|RESTORE) /.test(line));

test.each(["editor", "admin"] as const)("source controls stay read only at every entry: %s", async (role) => {
  const user = userEvent.setup();
  const server = pageServer({ role, mode: "llm_wiki", sourceRootId: guide.id });
  server.hold(install.id, ada);
  server.withVersions(install.id, [versionJSON(1, null, "Before\n"), versionJSON(2, 1, "- [ ] Keep the original\n")]);
  server.withTasks(install.id, "- [ ] Keep the original\n", 2);
  const { router } = renderApp(pagePath(install.id), server.app, { enhancements: [taskToggle] });
  const article = await screen.findByRole("article", { name: "Install" });
  await screen.findByText("You are editing this page elsewhere.");

  expect(screen.queryByRole("button", { name: "Edit" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Edit here" })).toBeNull();
  expect(within(article).getByRole("checkbox").hasAttribute("disabled")).toBe(true);
  await user.keyboard("{Control>}e{/Control}");
  expect(server.sent.filter((line) => line.startsWith("OPEN") || line.startsWith("TOGGLE"))).toEqual([]);

  await act(() => router.navigate(`${pagePath(install.id)}/versions/1`));
  await screen.findByRole("heading", { level: 1, name: "Version 1" });
  await screen.findByRole("article", { name: "Version 1" });
  expect(screen.queryByRole("button", { name: "Restore this version" })).toBeNull();
  expect(server.sent.filter((line) => line.startsWith("RESTORE"))).toEqual([]);
});

// Separate cases ensure an earlier visible control cannot mask a later write entry.
test.each(entries)("source controls stay read only at every entry: isolated %s", async (entry) => {
  const server = pageServer({ role: "editor", mode: "llm_wiki", sourceRootId: guide.id });
  server.withVersions(install.id, [versionJSON(1, null, "Before\n"), versionJSON(2, 1, "- [ ] Original\n")]);
  server.withTasks(install.id, "- [ ] Original\n", 2);
  if (entry === "edit-here") server.hold(install.id, ada);
  const path = entry === "restore" ? `${pagePath(install.id)}/versions/1` : pagePath(install.id);
  renderApp(path, server.app, { enhancements: [taskToggle] });
  if (entry === "restore") {
    await screen.findByRole("heading", { level: 1, name: "Version 1" });
    await screen.findByRole("article", { name: "Version 1" });
    expect(screen.queryByRole("button", { name: "Restore this version" })).toBeNull();
    return;
  }
  const article = await screen.findByRole("article", { name: "Install" });
  if (entry === "Mod+E") {
    expect(fireEvent.keyDown(document.body, { key: "e", ctrlKey: true })).toBe(true);
    await act(async () => {
      await Promise.resolve();
    });
    expect(server.sent.filter((line) => line.startsWith("OPEN"))).toEqual([]);
  } else if (entry === "task") {
    expect(within(article).getByRole("checkbox").hasAttribute("disabled")).toBe(true);
  } else if (entry === "edit-here") {
    await screen.findByText("You are editing this page elsewhere.");
    expect(screen.queryByRole("button", { name: "Edit here" })).toBeNull();
  } else {
    expect(screen.queryByRole("button", { name: "Edit" })).toBeNull();
  }
});

test.each([guide, agents, notes])(
  "source controls leave $name's explanation or ordinary body editable",
  async (page) => {
    const user = userEvent.setup();
    const server = pageServer({
      role: "editor",
      mode: "llm_wiki",
      sourceRootId: guide.id,
      nodes: [guide, agents, notes],
    });
    server.withTasks(page.id, "- [ ] Editable\n");
    renderApp(pagePath(page.id), server.app, { enhancements: [taskToggle] });
    const article = await screen.findByRole("article", { name: page.name });
    expect(screen.getByRole("button", { name: "Edit" })).toBeTruthy();
    expect(within(article).getByRole("checkbox").hasAttribute("disabled")).toBe(false);
    await user.click(within(article).getByRole("checkbox"));
    await waitFor(() => expect(server.contents.get(page.id)?.content).toBe("- [x] Editable\n"));
    expect(bodyWrites(server.sent)).toEqual([`TOGGLE ${page.name} 3 true on 1`]);

    await user.click(screen.getByRole("button", { name: "Edit" }));
    const editor = await pageEditor();
    act(() => editor.type("A note.\n"));
    await user.click(screen.getByRole("button", { name: "Done" }));
    await screen.findByRole("article", { name: page.name });
    expect(server.contents.get(page.id)?.content).toBe("- [x] Editable\nA note.\n");
    expect(bodyWrites(server.sent)).toEqual([
      `TOGGLE ${page.name} 3 true on 1`,
      `OPEN ${page.name}`,
      `PUT ${page.name} ${JSON.stringify("- [x] Editable\nA note.\n")} on 2 in session-1`,
    ]);
  }
);

test.each(changes)("source controls stay read only at every entry: mounted reading follows %s", async (change) => {
  const server = changingServer(change);
  server.withTasks(install.id, "- [ ] Original\n");
  server.hold(install.id, ada);
  const captured = capturedPage(pagePath(install.id), server);
  const article = await screen.findByRole("article", { name: "Install" });
  const heading = screen.getByRole("heading", { level: 1, name: "Install" });
  await screen.findByRole("button", { name: "Edit here" });
  expect(within(article).getByRole("checkbox").hasAttribute("disabled")).toBe(false);

  await refreshCapability(server, captured, change, true);
  expect(screen.getByRole("article", { name: "Install" })).toBe(article);
  expect(screen.getByRole("heading", { level: 1, name: "Install" })).toBe(heading);
  expect(screen.queryByRole("button", { name: "Edit" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Edit here" })).toBeNull();
  expect(within(article).getByRole("checkbox").hasAttribute("disabled")).toBe(true);
  expect(fireEvent.keyDown(document.body, { key: "e", ctrlKey: true })).toBe(true);
  expect(bodyWrites(server.sent)).toEqual([]);

  await refreshCapability(server, captured, change, false);
  expect(screen.getByRole("article", { name: "Install" })).toBe(article);
  expect(screen.getByRole("heading", { level: 1, name: "Install" })).toBe(heading);
  expect(screen.getByRole("button", { name: "Edit" })).toBeTruthy();
  expect(screen.getByRole("button", { name: "Edit here" })).toBeTruthy();
  expect(within(article).getByRole("checkbox").hasAttribute("disabled")).toBe(false);
  expect(bodyWrites(server.sent)).toEqual([]);
});

test.each(changes)("source controls stay read only at every entry: mounted version follows %s", async (change) => {
  const server = changingServer(change);
  server.withVersions(install.id, [versionJSON(1, null, "Before\n"), versionJSON(2, 1, "Now\n")]);
  const captured = capturedPage(`${pagePath(install.id)}/versions/1`, server);
  const article = await screen.findByRole("article", { name: "Version 1" });
  const heading = screen.getByRole("heading", { level: 1, name: "Version 1" });
  await screen.findByRole("button", { name: "Restore this version" });

  await refreshCapability(server, captured, change, true);
  expect(screen.getByRole("article", { name: "Version 1" })).toBe(article);
  expect(screen.getByRole("heading", { level: 1, name: "Version 1" })).toBe(heading);
  expect(screen.queryByRole("button", { name: "Restore this version" })).toBeNull();
  expect(bodyWrites(server.sent)).toEqual([]);

  await refreshCapability(server, captured, change, false);
  expect(screen.getByRole("article", { name: "Version 1" })).toBe(article);
  expect(screen.getByRole("heading", { level: 1, name: "Version 1" })).toBe(heading);
  expect(screen.getByRole("button", { name: "Restore this version" })).toBeTruthy();
  expect(bodyWrites(server.sent)).toEqual([]);
});

test.each(["source", "role"] as const)(
  "source controls stay read only at every entry: retained task callback follows %s",
  async (change) => {
    const server = pageServer({ role: "editor" });
    server.withTasks(install.id, "- [ ] Original\n");
    let latest: ReadingContext | undefined;
    const observe: Enhancement = (_container, context) => {
      latest = context;
    };
    const captured = capturedPage(pagePath(install.id), server, [taskToggle, observe]);
    const article = await screen.findByRole("article", { name: "Install" });
    const toggle = latest?.toggleTask;
    if (toggle === undefined) throw new Error("the editable reading view supplied no task callback");
    expect(within(article).getByRole("checkbox").hasAttribute("disabled")).toBe(false);

    act(() => blockBeforeRender(server, captured, change));
    expect(latest?.toggleTask).toBeUndefined();
    expect(within(article).getByRole("checkbox").hasAttribute("disabled")).toBe(true);
    await act(async () => {
      await toggle(3, true);
    });

    expect(bodyWrites(server.sent)).toEqual([]);
    expect(server.contents.get(install.id)).toEqual({ content: "- [ ] Original\n", revision: 1 });
  }
);

test.each([
  ["Edit", "source"],
  ["Edit", "role"],
  ["edit-here", "source"],
  ["edit-here", "role"],
  ["Mod+E", "source"],
  ["Mod+E", "role"],
] as const)("source controls stay read only at every entry: retained %s handler follows %s", async (entry, change) => {
  const openings = vi.spyOn(EditSession.prototype, "open");
  onTestFinished(() => openings.mockRestore());
  const server = pageServer({ role: "editor" });
  server.withTasks(install.id, "- [ ] Original\n");
  if (entry === "edit-here") server.hold(install.id, ada);
  const captured = capturedPage(pagePath(install.id), server);
  const article = await screen.findByRole("article", { name: "Install" });
  const button = await screen.findByRole("button", { name: entry === "edit-here" ? "Edit here" : "Edit" });
  let editsAfterClick = -1;

  await act(async () => {
    blockBeforeRender(server, captured, change);
    // Still connected: this calls the preceding render's real handler before the observer hides the control.
    expect(button.isConnected).toBe(true);
    if (entry === "Mod+E") {
      expect(fireEvent.keyDown(document.body, { key: "e", ctrlKey: true })).toBe(true);
    } else {
      fireEvent.click(button);
    }
    editsAfterClick = captured.stores().root.edits.size;
    await Promise.resolve();
  });
  // Observe the real method without replacing it; even a broken guard's request must settle before checking its log.
  await act(async () => {
    await Promise.allSettled(
      openings.mock.results.flatMap((result) => (result.type === "return" ? [result.value] : []))
    );
  });

  expect(screen.getByRole("article", { name: "Install" })).toBe(article);
  expect(screen.queryByRole("button", { name: "Edit" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Edit here" })).toBeNull();
  expect(editsAfterClick).toBe(0);
  expect(openings).not.toHaveBeenCalled();
  expect(bodyWrites(server.sent)).toEqual([]);
  expect(server.contents.get(install.id)).toEqual({ content: "- [ ] Original\n", revision: 1 });
});

test.each(["source", "role"] as const)(
  "source controls stay read only at every entry: retained restore handler follows %s",
  async (change) => {
    const restores = vi.spyOn(PageService.prototype, "restoreVersion");
    onTestFinished(() => restores.mockRestore());
    const user = userEvent.setup();
    const server = pageServer({ role: "editor" });
    server.withVersions(install.id, [versionJSON(1, null, "Before\n"), versionJSON(2, 1, "Now\n")]);
    const captured = capturedPage(`${pagePath(install.id)}/versions/1`, server);
    await user.click(await screen.findByRole("button", { name: "Restore this version" }));
    const dialog = await screen.findByRole("alertdialog", { name: "Restore version 1?" });
    await within(dialog).findByRole("textbox", { name: "Changes from the page now to version 1" });
    const restore = within(dialog).getByRole("button", { name: "Restore" });
    await waitFor(() => expect(restore.getAttribute("aria-disabled")).toBeNull());

    await act(async () => {
      blockBeforeRender(server, captured, change);
      expect(restore.isConnected).toBe(true);
      fireEvent.click(restore);
      await Promise.resolve();
    });
    // The real service is preserved: its response settles before asserting that no restore request was made.
    await act(async () => {
      await Promise.allSettled(
        restores.mock.results.flatMap((result) => (result.type === "return" ? [result.value] : []))
      );
    });

    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(screen.queryByRole("button", { name: "Restore this version" })).toBeNull();
    expect(restores).not.toHaveBeenCalled();
    expect(bodyWrites(server.sent)).toEqual([]);
    expect(server.contents.get(install.id)).toEqual({ content: "Now\n", revision: 2 });
    expect(captured.router.state.location.pathname).toBe(`${pagePath(install.id)}/versions/1`);
  }
);

test.each([guide, agents, notes])(
  "source controls leave $name's explanation or ordinary version restorable",
  async (page) => {
    const user = userEvent.setup();
    const server = pageServer({
      role: "editor",
      mode: "llm_wiki",
      sourceRootId: guide.id,
      nodes: [guide, agents, notes],
    });
    server.withVersions(page.id, [versionJSON(1, null, "Before\n"), versionJSON(2, 1, "Now\n")]);
    renderApp(`${pagePath(page.id)}/versions/1`, server.app);
    await user.click(await screen.findByRole("button", { name: "Restore this version" }));
    const dialog = await screen.findByRole("alertdialog", { name: "Restore version 1?" });
    await within(dialog).findByRole("textbox", { name: "Changes from the page now to version 1" });
    await user.click(within(dialog).getByRole("button", { name: "Restore" }));

    expect((await screen.findByRole("article", { name: page.name })).textContent).toBe("Before");
    expect(bodyWrites(server.sent)).toEqual([`RESTORE ${page.name} 1 on 2`]);
    expect(server.contents.get(page.id)).toEqual({ content: "Before\n", revision: 3 });
  }
);

test.each([notes, install])("a source's unresolved link may create another page under $name", async (parent) => {
  const user = userEvent.setup();
  const server = pageServer({ role: "editor", mode: "llm_wiki", sourceRootId: guide.id });
  const original = { content: "[[Other page]]\n", revision: 1 };
  server.contents.set(install.id, original);
  server.views.set(install.id, {
    html: '<p><a class="nw-wikilink nw-unresolved" data-nw-target="Other page">Other page</a></p>',
    revision: 1,
    assets_expire_at: null,
  });
  server.landings.set("Other page", {
    node_id: null,
    landing: { parent_id: parent.id, title: "Other page" },
    reason: null,
  });
  const { router } = renderApp(pagePath(install.id), server.app, { enhancements: [unresolvedLinks] });
  const article = await screen.findByRole("article", { name: "Install" });
  expect(screen.queryByRole("button", { name: "Edit" })).toBeNull();
  await user.click(within(article).getByRole("button", { name: "Other page" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Create page “Other page”?" });
  await user.click(within(dialog).getByRole("button", { name: "Create page" }));

  await screen.findByRole("heading", { level: 1, name: "Other page" });
  const made = server.nodes.find((node) => node.name === "Other page");
  expect(made?.parent_id).toBe(parent.id);
  expect(made?.id).not.toBe(install.id);
  expect(router.state.location.pathname).toBe(pagePath(made?.id ?? ""));
  expect(server.sent.filter((line) => line.startsWith("POST "))).toEqual([`POST Other page under ${parent.name}`]);
  expect(bodyWrites(server.sent)).toEqual([]);
  expect(server.contents.get(install.id)).toEqual(original);
});

test("a source's read-only body still permits adding an attachment", async () => {
  const server = pageServer({ role: "editor", mode: "llm_wiki", sourceRootId: guide.id });
  server.withTasks(install.id, "- [ ] Original\n");
  renderApp(pagePath(install.id), server.app, { enhancements: [taskToggle] });
  const article = await screen.findByRole("article", { name: "Install" });
  expect(within(article).getByRole("checkbox").hasAttribute("disabled")).toBe(true);
  const section = await attachments();
  expect(within(section).getByRole("button", { name: "Upload" })).toBeTruthy();
  fireEvent.change(picker(section), { target: { files: [new File(["original bytes"], "evidence.txt")] } });

  await within(section).findByRole("link", { name: "evidence.txt (source)" });
  expect(server.sent.filter((line) => line.startsWith("UPLOAD "))).toEqual(["UPLOAD evidence.txt under Install"]);
  expect(server.nodes.find((node) => node.name === "evidence.txt")).toMatchObject({
    kind: "asset",
    parent_id: install.id,
  });
  expect(bodyWrites(server.sent)).toEqual([]);
  expect(server.contents.get(install.id)).toEqual({ content: "- [ ] Original\n", revision: 1 });
});
