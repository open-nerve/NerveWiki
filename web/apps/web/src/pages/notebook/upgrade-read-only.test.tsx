import type * as Adapter from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect } from "react";
import { Outlet } from "react-router";
import { expect, test, vi } from "vitest";

import { routes } from "../../app/routes";
import { taskToggle } from "../../reading/task-toggle";
import { unresolvedLinks } from "../../reading/unresolved-links";
import type { Notebook, NotebookRole } from "../../services/notebook.service";
import { useStore } from "../../stores/context";
import type { RootStore } from "../../stores/root.store";
import { attachments, dropped, nodes, picker } from "../../test/attachments";
import { notebookJSON, upgradeOf, workspaceJSON } from "../../test/fakes";
import { created } from "../../test/history-server";
import { jobsServer } from "../../test/jobs-server";
import {
  ada,
  assetNode,
  guide,
  install,
  linux,
  notes,
  pageNode,
  pagePath,
  pageServer,
  versionJSON,
} from "../../test/page-server";
import { renderApp } from "../../test/render";

// What a notebook's admin writes from the web, as its controls offer it,
// while no upgrade is under way, and none of it while one is (M11/P1
// design 4.4, the controls' table): one judgment, writesNotebook, for each
// of them. Each row is its own test; each test shows the control without
// an upgrade first, so that its absence with one says something.

/** The drag and drop the tree registers, live: its monitor, and its rows' drop targets and draggables. */
const live = vi.hoisted(() => ({
  monitors: new Set<unknown>(),
  targets: new Set<Element>(),
  draggables: new Set<Element>(),
}));

vi.mock("@atlaskit/pragmatic-drag-and-drop/element/adapter", async (importOriginal) => {
  const actual = await importOriginal<typeof Adapter>();
  return {
    ...actual,
    monitorForElements: (args: Parameters<typeof actual.monitorForElements>[0]) => {
      const off = actual.monitorForElements(args);
      live.monitors.add(args);
      return () => {
        live.monitors.delete(args);
        off();
      };
    },
    dropTargetForElements: (args: Parameters<typeof actual.dropTargetForElements>[0]) => {
      const off = actual.dropTargetForElements(args);
      live.targets.add(args.element);
      return () => {
        live.targets.delete(args.element);
        off();
      };
    },
    draggable: (args: Parameters<typeof actual.draggable>[0]) => {
      const off = actual.draggable(args);
      live.draggables.add(args.element);
      return () => {
        live.draggables.delete(args.element);
        off();
      };
    },
  };
});

const home = `/lab/notebooks/${notebookJSON.id}`;
const upgrading = upgradeOf("upgrading");

/** The two cases of each row: no upgrade, the control offered; an upgrade under way, not. */
const cases = [
  ["no upgrade", null],
  ["an upgrade under way", upgrading],
] as const;

type PageServer = ReturnType<typeof pageServer>;

/** writes are the writes that went out, the server's own and those it refused as frozen (frozenWhileUpgrading). */
const writes = (sent: readonly string[]) =>
  sent.filter((line) => !line.startsWith("GET ") && !line.startsWith("BEAT ") && !line.startsWith("END "));

/** upgraded is server's Plans, with the upgrade upgrade (none: null), seen as role. */
function upgraded<T extends PageServer>(server: T, upgrade: Notebook["upgrade"], role: NotebookRole = "admin"): T {
  server.notebook = { ...server.notebook, role, upgrade };
  return server;
}

/** captured renders path for server's app, keeping the stores of the generation the page uses. */
function captured(path: string, server: PageServer, enhancements = [taskToggle]) {
  let root: RootStore | undefined;
  function Capture() {
    const store = useStore();
    useEffect(() => {
      root = store;
    }, [store]);
    return <Outlet />;
  }
  const rendered = renderApp(path, server.app, { routes: [{ Component: Capture, children: routes }], enhancements });
  return {
    ...rendered,
    root(): RootStore {
      if (root === undefined) {
        throw new Error("the page has no signed-in stores");
      }
      return root;
    },
    /** freeze gives Plans an upgrade under way at once, as a list read again would, before the page renders again. */
    freeze(): void {
      server.notebook = { ...server.notebook, upgrade: upgrading };
      this.root().notebooksOf(workspaceJSON)?.receive(server.notebook);
      expect(this.root().pagesOf(server.notebook)?.notebook?.upgrade).toEqual(upgrading);
    },
  };
}

/** expectOffered says whether the control found is there, once the page it is in has shown. */
function expectOffered(control: HTMLElement | null, offered: boolean): void {
  expect(control !== null).toBe(offered);
}

const offered = (upgrade: Notebook["upgrade"]) => upgrade === null;

test.each(cases)("the notebook's home offers New page: %s", async (_, upgrade) => {
  const server = upgraded(pageServer(), upgrade);
  renderApp(home, server.app);

  const heading = await screen.findByRole("heading", { level: 1, name: "Plans" });
  const section = heading.closest("section") as HTMLElement;
  expectOffered(within(section).queryByRole("button", { name: "New page" }), offered(upgrade));
});

test.each(cases)("the page tree offers New page: %s", async (_, upgrade) => {
  const server = upgraded(pageServer(), upgrade);
  renderApp(pagePath(guide.id), server.app);

  const tree = await screen.findByRole("navigation", { name: "Pages of Plans" });
  await within(tree).findByRole("link", { name: "Guide" });
  expectOffered(within(tree).queryByRole("button", { name: "New page" }), offered(upgrade));
});

test.each(cases)(
  "the page tree's rows offer their menu, New subpage, Rename, Move to… and Delete: %s",
  async (_, upgrade) => {
    const server = upgraded(pageServer(), upgrade);
    renderApp(pagePath(guide.id), server.app);

    const tree = await screen.findByRole("navigation", { name: "Pages of Plans" });
    await within(tree).findByRole("link", { name: "Notes" });
    expectOffered(within(tree).queryByRole("button", { name: "Actions for Guide" }), offered(upgrade));
    expectOffered(within(tree).queryByRole("button", { name: "Actions for Notes" }), offered(upgrade));
  }
);

test.each(cases)("the page tree moves and orders pages by drag and drop, monitored: %s", async (_, upgrade) => {
  const server = upgraded(pageServer(), upgrade);
  renderApp(pagePath(guide.id), server.app);

  const tree = await screen.findByRole("navigation", { name: "Pages of Plans" });
  await within(tree).findByRole("link", { name: "Notes" });
  expect(live.monitors.size).toBe(offered(upgrade) ? 1 : 0);
});

test.each(cases)("the page tree's rows take a drop: %s", async (_, upgrade) => {
  const server = upgraded(pageServer(), upgrade);
  renderApp(pagePath(guide.id), server.app);

  const tree = await screen.findByRole("navigation", { name: "Pages of Plans" });
  const row = (await within(tree).findByRole("link", { name: "Notes" })).parentElement as HTMLElement;
  expect(live.targets.has(row)).toBe(offered(upgrade));
});

test.each(cases)(
  "the page tree's rows drag, and the attachments' rows offer Rename, Move to… and Delete: %s",
  async (_, upgrade) => {
    // The rows of the tree drag only where they take a drop: the attachments' menu is the judgment's own.
    const user = userEvent.setup();
    const server = upgraded(pageServer({ nodes }), upgrade);
    renderApp(pagePath(guide.id), server.app);

    const section = await attachments();
    await user.click(await within(section).findByRole("button", { name: "Actions for data.zip" }));
    const menu = await screen.findByRole("menu");
    await within(menu).findByRole("menuitem", { name: "Copy embed" });
    for (const item of ["Rename", "Move to…", "Delete"]) {
      expectOffered(within(menu).queryByRole("menuitem", { name: item }), offered(upgrade));
    }
    await user.keyboard("{Escape}");
    const tree = screen.getByRole("navigation", { name: "Pages of Plans" });
    const row = within(tree).getByRole("link", { name: "Notes" }).parentElement as HTMLElement;
    expect(live.draggables.has(row)).toBe(offered(upgrade));
  }
);

test.each(cases)("a page offers Edit: %s", async (_, upgrade) => {
  const server = upgraded(pageServer(), upgrade);
  renderApp(pagePath(install.id), server.app);

  await screen.findByRole("article", { name: "Install" });
  await screen.findByRole("heading", { level: 1, name: "Install" });
  expectOffered(screen.queryByRole("button", { name: "Edit" }), offered(upgrade));
});

test.each(cases)("a page is edited by Ctrl+E: %s", async (_, upgrade) => {
  const server = upgraded(pageServer(), upgrade);
  renderApp(pagePath(install.id), server.app);
  await screen.findByRole("article", { name: "Install" });

  await act(async () => {
    fireEvent.keyDown(document.body, { key: "e", ctrlKey: true });
    await Promise.resolve();
  });

  if (offered(upgrade)) {
    await waitFor(() => expect(writes(server.sent)).toEqual(["OPEN Install"]));
  } else {
    await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
    expect(writes(server.sent)).toEqual([]);
  }
});

test.each(cases)("a page the account edits elsewhere offers Edit here: %s", async (_, upgrade) => {
  const server = upgraded(pageServer(), upgrade);
  server.hold(install.id, ada);
  renderApp(pagePath(install.id), server.app);

  await screen.findByText("You are editing this page elsewhere.");
  expectOffered(screen.queryByRole("button", { name: "Edit here" }), offered(upgrade));
});

test.each(cases)("a page's reading view ticks its task items: %s", async (_, upgrade) => {
  const server = upgraded(pageServer(), upgrade);
  server.withTasks(install.id, "- [ ] Keep\n");
  renderApp(pagePath(install.id), server.app, { enhancements: [taskToggle] });

  const article = await screen.findByRole("article", { name: "Install" });
  expect(within(article).getByRole("checkbox").hasAttribute("disabled")).toBe(!offered(upgrade));
});

test.each(cases)("a version's page offers Restore this version: %s", async (_, upgrade) => {
  const server = upgraded(pageServer(), upgrade);
  server.withVersions(install.id, [versionJSON(1, null, "Before\n"), versionJSON(2, 1, "Now\n")]);
  renderApp(`${pagePath(install.id)}/versions/1`, server.app);

  await screen.findByRole("article", { name: "Version 1" });
  expectOffered(screen.queryByRole("button", { name: "Restore this version" }), offered(upgrade));
});

test.each(cases)("a page takes files dropped on it: %s", async (_, upgrade) => {
  const server = upgraded(pageServer(), upgrade);
  server.uploadsHeld = true;
  renderApp(pagePath(install.id), server.app);
  const article = await screen.findByRole("article", { name: "Install" });
  const zone = article.parentElement as HTMLElement;

  const over = dropped([]);
  fireEvent.dragOver(article, { dataTransfer: over });
  expect(over.dropEffect).toBe(offered(upgrade) ? "copy" : "none");
  // A folder dropped says it is not uploaded before anything is sent: the drop was taken.
  fireEvent.drop(article, { dataTransfer: dropped([], ["photos"]) });
  const folders = "Folders are not uploaded: zip a folder of notes and import it from the notebook's settings.";
  expectOffered(within(zone).queryByText(folders), offered(upgrade));
});

test("files dropped on a page as an upgrade starts, its drop not yet rendered again, are not uploaded", async () => {
  const server = pageServer();
  const page = captured(pagePath(install.id), server);
  const article = await screen.findByRole("article", { name: "Install" });
  const over = dropped([]);
  fireEvent.dragOver(article, { dataTransfer: over });
  expect(over.dropEffect).toBe("copy");

  await act(async () => {
    page.freeze();
    // Still connected: the preceding render's handler takes the drop, which the upgrade must stop.
    expect(article.isConnected).toBe(true);
    fireEvent.drop(article, { dataTransfer: dropped([new File(["a"], "a.png")]) });
    await Promise.resolve();
  });
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));

  expect(page.root().assetsOf(server.notebook)?.uploads).toEqual([]);
  expect(writes(server.sent)).toEqual([]);
});

test.each(cases)("the attachments' section offers Upload and takes a drop: %s", async (_, upgrade) => {
  const server = upgraded(pageServer({ nodes }), upgrade);
  renderApp(pagePath(guide.id), server.app);

  const section = await attachments();
  await within(section).findByRole("link", { name: "data.zip" });
  expectOffered(within(section).queryByRole("button", { name: "Upload" }), offered(upgrade));
  const over = dropped([]);
  fireEvent.dragOver(section, { dataTransfer: over });
  expect(over.dropEffect).toBe(offered(upgrade) ? "copy" : "none");
});

test("files chosen in the attachments' picker as an upgrade starts, not yet rendered again, are not uploaded", async () => {
  const server = pageServer({ nodes });
  const page = captured(pagePath(guide.id), server);
  const section = await attachments();
  await within(section).findByRole("button", { name: "Upload" });
  const input = picker(section);

  await act(async () => {
    page.freeze();
    // Still connected: the preceding render's change handler takes the files, which the upgrade must stop.
    expect(input.isConnected).toBe(true);
    fireEvent.change(input, { target: { files: [new File(["a"], "a.png")] } });
    await Promise.resolve();
  });
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));

  expect(page.root().assetsOf(server.notebook)?.uploads).toEqual([]);
  expect(writes(server.sent)).toEqual([]);
});

test.each(cases)(
  "a source's upload that failed is retried, the LLM Wiki since downgraded and its upgrade %s",
  async (_, upgrade) => {
    // An LLM Wiki downgraded keeps a failed source's row in its tab; its upgrade then started is a real server's way.
    const user = userEvent.setup();
    const taken = pageNode(46, "taken", guide);
    const server = pageServer({ mode: "llm_wiki", sourceRootId: guide.id, nodes: [guide, install, notes, taken] });
    const page = captured(pagePath(guide.id), server);
    const section = await attachments();
    fireEvent.change(picker(section), { target: { files: [new File(["# Kept\n"], "taken.md")] } });
    const field = await within(section).findByRole("textbox", { name: "Title for taken.md" });
    const row = field.closest("li") as HTMLElement;
    await within(row).findByText("Another page or attachment here has this title. Choose another title.");
    await user.clear(field);
    await user.type(field, "Kept");
    const before = writes(server.sent);

    act(() => {
      server.notebook = { ...server.notebook, mode: "standard", source_root_id: null, upgrade };
      page.root().notebooksOf(workspaceJSON)?.receive(server.notebook);
    });
    const retry = within(row).getByRole("button", { name: "Add source" });
    await waitFor(() => expect(retry).toHaveProperty("disabled", !offered(upgrade)));
    expect(field).toHaveProperty("disabled", !offered(upgrade));
    await act(async () => {
      // A retained submission reaches the form's handler even with its controls disabled.
      fireEvent.submit(field.closest("form") as HTMLFormElement);
      await Promise.resolve();
    });
    await act(() => new Promise((resolve) => setTimeout(resolve, 50)));

    expect(writes(server.sent).slice(before.length)).toEqual(offered(upgrade) ? ["POST Kept under Guide"] : []);
  }
);

test.each([
  ["admin", null, "landing"],
  ["admin", upgrading, "upgrading"],
  ["editor", upgrading, "upgrading"],
  ["reader", upgrading, "reader"],
] as const)("a %s's link to no page, its notebook's upgrade %o, creates the page: %s", async (role, upgrade, says) => {
  const server = upgraded(pageServer({ role }), upgrade, role);
  server.views.set(guide.id, {
    html: `<p><a class="nw-wikilink nw-unresolved" data-nw-target="Fresh">Fresh</a></p>`,
    revision: 1,
    assets_expire_at: null,
  });
  renderApp(pagePath(guide.id), server.app, { enhancements: [unresolvedLinks] });
  await screen.findByRole("article", { name: "Guide" });

  await userEvent.click(await screen.findByRole("button", { name: "Fresh" }));

  if (says === "landing") {
    await screen.findByRole("alertdialog", { name: "Create page “Fresh”?" });
    expect(server.sent).toContain("GET landing Fresh");
    return;
  }
  const dialog = await screen.findByRole("alertdialog", { name: "“Fresh” does not exist" });
  expect(within(dialog).getByText(/^No page has this name\./u).textContent).toBe(
    says === "upgrading"
      ? "No page has this name. The notebook is being upgraded to an LLM Wiki: no page is created until the upgrade ends."
      : "No page has this name. The notebook's editors can create it from this link."
  );
  expect(server.sent.filter((line) => line.startsWith("GET landing"))).toEqual([]);
});

test.each([
  ["admin", null, "undo"],
  ["admin", upgrading, "upgrade"],
  ["editor", upgrading, "upgrade"],
  ["reader", upgrading, "role"],
] as const)("a change's page, for a %s, the upgrade %o, offers Undo change: %s", async (role, upgrade, offers) => {
  const server = upgraded(pageServer({ role }), upgrade, role);
  const made = server.log("made", { changes: [created(notes)] });
  renderApp(`${home}/history/${made.id}`, server.app);

  // The heading names the change once it is read; what it offers comes with it.
  await screen.findByRole("heading", { level: 1, name: /·/u });
  expectOffered(screen.queryByRole("button", { name: "Undo change" }), offers === "undo");
  const said = {
    upgrade: "This notebook is read-only until its upgrade ends.",
    role: "Editors and admins can undo this change.",
  } as const;
  for (const [why, text] of Object.entries(said)) {
    expectOffered(screen.queryByText(text), why === offers);
  }
});

/** inTrash is jobsServer with Install, under Guide, and Notes in the trash; Install's history too long to restore here. */
function inTrash(upgrade: Notebook["upgrade"]) {
  const server = upgraded(
    jobsServer({ nodes: [guide, install, linux, notes, assetNode(20, "a.png", guide)] }),
    upgrade
  );
  server.trashNode(install.id).history = 400_001;
  server.trashNode(notes.id);
  return server;
}

const tooLarge =
  "“Install” is too large to restore here: it holds more pages, attachments, content, links or versions than one restore brings back. Pages: 2, attachments: 0.";

test.each(cases)("the trash's rows offer Restore: %s", async (_, upgrade) => {
  const server = inTrash(upgrade);
  renderApp(`${home}/trash`, server.app);

  const list = await screen.findByRole("list", { name: "Trash" });
  await within(list).findAllByRole("listitem");
  expectOffered(within(list).queryByRole("button", { name: /^Restore “Notes”/u }), offered(upgrade));
});

test("the trash's offer to restore in the background goes as an upgrade starts", async () => {
  const user = userEvent.setup();
  const server = inTrash(null);
  const page = captured(`${home}/trash`, server);
  await screen.findByRole("list", { name: "Trash" });
  await user.click(screen.getAllByRole("button", { name: /^Restore “Install” \(/u })[0] as HTMLElement);
  await waitFor(() => expect(screen.getByRole("alert").textContent).toBe(tooLarge));
  const start = screen.getByRole("button", { name: /^Restore “Install” in the background/u });

  act(() => page.freeze());

  await waitFor(() => expect(start.isConnected).toBe(false));
  expect(screen.queryByRole("button", { name: /in the background/u })).toBeNull();
  expect(screen.queryByRole("button", { name: /^Restore “/u })).toBeNull();
});

test.each(cases)("a trash item's preview offers Restore: %s", async (_, upgrade) => {
  const server = inTrash(upgrade);
  renderApp(`${home}/trash/${notes.id}`, server.app);

  // The heading names the item once it is read; Restore comes with it.
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  expectOffered(screen.queryByRole("button", { name: /^Restore “Notes”/u }), offered(upgrade));
});

test("a trash item's preview's offer to restore in the background goes as an upgrade starts", async () => {
  const user = userEvent.setup();
  const server = inTrash(null);
  const page = captured(`${home}/trash/${install.id}`, server);
  await user.click(await screen.findByRole("button", { name: /^Restore “Install” \(/u }));
  await waitFor(() => expect(screen.getByRole("alert").textContent).toBe(tooLarge));
  const start = screen.getByRole("button", { name: /^Restore “Install” in the background/u });

  act(() => page.freeze());

  await waitFor(() => expect(start.isConnected).toBe(false));
  expect(screen.queryByRole("button", { name: /^Restore “Install”/u })).toBeNull();
});

test.each([
  ["admin", null, "import"],
  ["admin", upgrading, "upgrade"],
  ["editor", upgrading, "upgrade"],
  ["reader", upgrading, "role"],
] as const)(
  "the notebook's transfers, for a %s, the upgrade %o, offer an import: %s",
  async (role, upgrade, offers) => {
    const server = upgraded(jobsServer({ role }), upgrade, role);
    renderApp(`${home}/settings/transfer`, server.app);

    await screen.findByRole("heading", { level: 2, name: "Import" });
    await screen.findByRole("button", { name: "Export the whole notebook" });
    expectOffered(screen.queryByRole("button", { name: "Import a zip…" }), offers === "import");
    const texts = {
      upgrade: "This notebook is read-only until its upgrade ends.",
      role: "Only those who can edit this notebook import into it.",
    } as const;
    for (const [said, text] of Object.entries(texts)) {
      expectOffered(screen.queryByText(text), said === offers);
    }
  }
);
