import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { eventHandlers } from "../../events/handlers";
import type { NotebookRole } from "../../services/notebook.service";
import { eventServer, withEvents } from "../../test/event-server";
import { json, notebookJSON, problem, workspaceJSON, type Answer } from "../../test/fakes";
import {
  assetNode,
  bob,
  guide,
  install,
  linux,
  notes,
  pageNode,
  pagePath,
  pageServer,
  type FakeTrashItem,
} from "../../test/page-server";
import { renderApp } from "../../test/render";

// A notebook's trash (M8 design 4.5, 4.13; M8/P2 design 4.2, 4.6).

const home = `/lab/notebooks/${notebookJSON.id}`;
const trashPath = `${home}/trash`;

/**
 * trashed is pageServer with three items in its trash, newest first: the
 * attachment a.png of Guide, deleted by Bob; Notes, at the top level; and
 * Linux, under Guide / Install. Ada's role in Plans is role, answers
 * answering instead of it.
 */
function trashed(role: NotebookRole = "admin", answers: Record<string, Answer> = {}) {
  const picture = assetNode(20, "a.png", guide);
  const server = pageServer({ role, answers, nodes: [guide, install, linux, notes, picture] });
  server.trashNode(linux.id);
  server.trashNode(notes.id);
  server.trashNode(picture.id).deletedBy = bob;
  return { server, picture };
}

/** rows are the items the trash lists, each row's text. */
const rows = () => within(screen.getByRole("list", { name: "Trash" })).getAllByRole("listitem");

test("the trash lists its items newer first, each by its name, leading to its preview, where it was, who deleted it, until when it can be restored, what it holds and where a restore puts it; the page and its tab are named for it", async () => {
  const { server } = trashed();
  renderApp(trashPath, server.app);

  expect(await screen.findByRole("heading", { level: 1, name: "Trash" })).toBeTruthy();
  await waitFor(() => expect(document.title).toBe("Trash · Plans · Nerve Wiki"));
  const [picture, top, deep] = rows();
  expect(
    within(picture as HTMLElement)
      .getByRole("link")
      .getAttribute("href")
  ).toBe(`${trashPath}/0199a2b4-0000-7000-8000-000000000120`);
  expect((picture as HTMLElement).textContent).toContain("a.png");
  expect((picture as HTMLElement).textContent).toContain("From: Guide");
  expect((picture as HTMLElement).textContent).toContain("Deleted by Bob · ");
  expect((picture as HTMLElement).textContent).toContain("Pages: 0, attachments: 1");
  expect((picture as HTMLElement).textContent).toContain("Restores under “Guide”");
  expect((top as HTMLElement).textContent).toContain("From: the notebook's top level");
  expect((top as HTMLElement).textContent).toContain("Restores at the notebook's top level");
  expect((deep as HTMLElement).textContent).toContain("From: Guide / Install");
  expect((deep as HTMLElement).textContent).toContain("Deleted by Ada · ");
  expect((deep as HTMLElement).textContent).toContain("Can be restored until Dec 3, 2026, 7:01 AM");
  expect((deep as HTMLElement).textContent).toContain("Pages: 1, attachments: 0");
  expect((deep as HTMLElement).textContent).toContain("Restores under “Install”");
  // When it was deleted, in full as its title.
  expect(
    within(deep as HTMLElement)
      .getByTitle("Oct 4, 2026, 8:01 AM")
      .getAttribute("dateTime")
  ).toBe("2026-10-04T08:01:00.000Z");
  expect(server.sent).toContain("GET trash");
});

test("an item whose parent is in the trash restores at the top level, and says so; one whose pages would nest too deep where it was, too", async () => {
  const chain = [pageNode(30, "L1")];
  for (let level = 2; level <= 10; level++) {
    chain.push(pageNode(29 + level, `L${level.toString()}`, chain.at(-1)));
  }
  const server = pageServer({ nodes: [guide, install, linux, notes, ...chain] });
  server.trashNode(linux.id);
  server.trashNode(install.id);
  // The tenth level deleted, then its line moved a level deeper: it would nest eleven deep where it was.
  server.trashNode(chain[9]?.id ?? "");
  const above = pageNode(29, "L0");
  server.nodes = [
    above,
    ...server.nodes.map((node) => (node.id === chain[0]?.id ? { ...node, parent_id: above.id } : node)),
  ];
  renderApp(trashPath, server.app);

  await screen.findByRole("list", { name: "Trash" });
  expect(rows().map((row) => row.querySelector("p:last-child")?.textContent)).toEqual([
    "Restores at the top level: where it was, it would nest more than 10 levels deep",
    "Restores under “Guide”",
    "Restores at the top level: the page it was under is in the trash",
  ]);
});

test("an empty trash says so", async () => {
  renderApp(trashPath, pageServer().app);

  expect(await screen.findByText("The trash is empty.")).toBeTruthy();
  expect(screen.queryByRole("list", { name: "Trash" })).toBeNull();
});

test("a writer's Restore names the item by where it was; items of one name and place each by when it was deleted too, their links as well", async () => {
  const again = { ...notes, id: "0199a2b4-0000-7000-8000-000000000140" };
  const server = pageServer({ nodes: [guide, install, linux, notes] });
  server.trashNode(notes.id);
  server.nodes = [...server.nodes, again];
  server.trashNode(again.id);
  server.trashNode(linux.id);
  renderApp(trashPath, server.app);

  await screen.findByRole("list", { name: "Trash" });
  expect(
    screen.getAllByRole("button", { name: /^Restore/ }).map((button) => button.getAttribute("aria-label"))
  ).toEqual([
    "Restore “Linux” (From: Guide / Install)",
    "Restore “Notes” (From: the notebook's top level, deleted Oct 4, 2026, 8:02 AM)",
    "Restore “Notes” (From: the notebook's top level, deleted Oct 4, 2026, 8:01 AM)",
  ]);
  expect(
    within(screen.getByRole("list", { name: "Trash" }))
      .getAllByRole("link")
      .map((link) => link.getAttribute("aria-label"))
  ).toEqual([
    "Linux (From: Guide / Install)",
    "Notes (From: the notebook's top level, deleted Oct 4, 2026, 8:02 AM)",
    "Notes (From: the notebook's top level, deleted Oct 4, 2026, 8:01 AM)",
  ]);
  // The visible text is the start of each name.
  expect(screen.getAllByRole("button", { name: /^Restore/ }).map((button) => button.textContent)).toEqual([
    "Restore",
    "Restore",
    "Restore",
  ]);
});

test("a reader sees the trash and its items, without Restore", async () => {
  const { server } = trashed("reader");
  renderApp(trashPath, server.app);

  await screen.findByRole("list", { name: "Trash" });
  expect(rows()).toHaveLength(3);
  expect(screen.queryByRole("button", { name: /^Restore/ })).toBeNull();
});

test("More reads the next fifty items after those read, and leaves once the trash is read whole, the focus on the first it added", async () => {
  const user = userEvent.setup();
  const pages = Array.from({ length: 51 }, (_, i) => pageNode(200 + i, `Page ${(i + 1).toString()}`));
  const server = pageServer({ nodes: pages });
  for (const page of pages) {
    server.trashNode(page.id);
  }
  renderApp(trashPath, server.app);

  await screen.findByRole("list", { name: "Trash" });
  expect(rows()).toHaveLength(50);
  const more = screen.getByRole("button", { name: "More of the trash" });
  await user.click(more);

  await waitFor(() => expect(rows()).toHaveLength(51));
  expect(screen.queryByRole("button", { name: "More of the trash" })).toBeNull();
  expect(document.activeElement?.textContent).toBe("Page 1");
  expect(server.sent.filter((each) => each.startsWith("GET trash"))).toEqual(["GET trash", "GET trash below Page 2"]);
});

/** restoreOf is the Restore of the item named name, the first of its rows. */
const restoreOf = (name: string) => screen.getAllByRole("button", { name: new RegExp(`^Restore “${name}”`) })[0];

test("a restore stays in the trash, which says where the item went with a way to it, the title taking the focus, described by it; the list is read again, without it, and so is the tree", async () => {
  const user = userEvent.setup();
  const { server } = trashed();
  renderApp(trashPath, server.app);
  await screen.findByRole("list", { name: "Trash" });
  const before = server.sent.length;

  await user.click(restoreOf("Linux") as HTMLElement);

  const status = await screen.findByRole("status");
  await waitFor(() => expect(status.textContent).toBe("Restored “Linux” under “Install”. Open"));
  const heading = screen.getByRole("heading", { level: 1, name: "Trash" });
  expect(document.activeElement).toBe(heading);
  expect(heading.getAttribute("aria-describedby")).toBe(status.id);
  await waitFor(() => expect(rows()).toHaveLength(2));
  expect(server.sent.slice(before)).toEqual(["RESTORE Linux", "GET nodes", "GET trash"]);
  expect(within(status).getByRole("link", { name: "Open" }).getAttribute("href")).toBe(pagePath(linux.id));
  const tree = screen.getByRole("navigation", { name: "Pages of Plans" });
  await user.click(within(tree).getByRole("button", { name: "Subpages of Guide" }));
  await user.click(within(tree).getByRole("button", { name: "Subpages of Install" }));
  expect(within(tree).getByRole("link", { name: "Linux" })).toBeTruthy();

  await user.click(within(status).getByRole("link", { name: "Open" }));
  expect(await screen.findByRole("heading", { level: 1, name: "Linux" })).toBe(document.activeElement);
});

test("an item restored by another name says so: a sibling where it went has its name", async () => {
  const user = userEvent.setup();
  const { server } = trashed();
  renderApp(trashPath, server.app);
  await screen.findByRole("list", { name: "Trash" });
  server.nodes = [...server.nodes, pageNode(50, "Notes")];

  await user.click(restoreOf("Notes") as HTMLElement);

  await waitFor(() =>
    expect(screen.getByRole("status").textContent).toBe(
      "Restored as “Notes 2” at the notebook's top level: a page or attachment there had its name. Open"
    )
  );
});

test("an attachment restored opens where it is: its page, the focus on its row; one at the top level, the notebook's home", async () => {
  const user = userEvent.setup();
  const { server, picture } = trashed();
  const loose = assetNode(21, "b.pdf");
  server.nodes = [...server.nodes, loose];
  server.trashNode(loose.id);
  renderApp(trashPath, server.app);
  await screen.findByRole("list", { name: "Trash" });

  await user.click(restoreOf("a.png") as HTMLElement);
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Restored “a.png” under “Guide”. Open"));
  await user.click(within(screen.getByRole("status")).getByRole("link", { name: "Open" }));

  expect(await screen.findByRole("heading", { level: 1, name: "Guide" })).toBeTruthy();
  await waitFor(() => expect(document.activeElement?.textContent).toBe("a.png (opens in a new tab)"));
  expect(document.activeElement?.getAttribute("data-attachment")).toBe(picture.id);

  await user.click(
    within(screen.getByRole("navigation", { name: "More of Plans" })).getByRole("link", { name: "Trash" })
  );
  await user.click(await screen.findByRole("button", { name: /^Restore “b\.pdf”/ }));
  await user.click(await within(screen.getByRole("status")).findByRole("link", { name: "Open" }));
  expect(await screen.findByRole("heading", { level: 1, name: "Plans" })).toBeTruthy();
  await waitFor(() => expect(document.activeElement?.textContent).toBe("b.pdf (opens in a new tab)"));
});

type Server = ReturnType<typeof trashed>["server"];

// What became of Linux once the list read it, as the server answers its restore: each refusal the trash says.
test.each<[string, (server: Server) => void, string, boolean]>([
  [
    "restored elsewhere, page.not_in_trash",
    (server) => void server.restoreElsewhere(linux.id),
    "“Linux” changed meanwhile: the trash shows it as it is now.",
    true,
  ],
  [
    "purged in part, page.trash_changed",
    (server) => {
      (server.trash.get(linux.id) as FakeTrashItem).purged = true;
    },
    "“Linux” changed meanwhile: the trash shows it as it is now.",
    false,
  ],
  [
    "past its time, page.restore_expired",
    (server) => {
      (server.trash.get(linux.id) as FakeTrashItem).expired = true;
    },
    "“Linux” can no longer be restored: its time in the trash is over.",
    true,
  ],
  [
    "restored and deleted again with the page above it, page.not_trash_root",
    (server) => {
      server.restoreElsewhere(linux.id);
      server.trashNode(install.id);
    },
    "“Linux” was deleted again with a page above it: restore that page, which holds it now.",
    true,
  ],
  [
    "of too long a history, page.restore_too_large",
    (server) => {
      (server.trash.get(linux.id) as FakeTrashItem).history = 400_001;
    },
    "“Linux” is too large to restore here. Pages: 1, attachments: 0.",
    false,
  ],
  [
    "as the server is busy, server_busy",
    (server) => {
      server.restoresBusy = true;
    },
    "The server is busy. Try again in a moment.",
    false,
  ],
])(
  "a restore of an item %s says why above the list, which is read again; the item gone from it, its Restore gives the focus to the title, else keeps it",
  async (_case, change, text, gone) => {
    const user = userEvent.setup();
    const { server } = trashed();
    renderApp(trashPath, server.app);
    await screen.findByRole("list", { name: "Trash" });
    change(server);
    const before = server.sent.length;

    const restore = restoreOf("Linux") as HTMLElement;
    await user.click(restore);

    expect((await screen.findByRole("alert")).textContent).toBe(text);
    await waitFor(() => expect(server.sent.slice(before)).toEqual(["RESTORE Linux", "GET nodes", "GET trash"]));
    await waitFor(() => expect(rows().some((row) => row.textContent?.includes("From: Guide / Install"))).toBe(!gone));
    expect(document.activeElement).toBe(gone ? screen.getByRole("heading", { level: 1, name: "Trash" }) : restore);
    expect(screen.getByRole("status").textContent).toBe("");
  }
);

test("a restore refused, the role taken away meanwhile, says why, and reads the notebooks again: a reader by now, the Restores go, the focus to the title", async () => {
  const user = userEvent.setup();
  let role: NotebookRole = "admin";
  const reads: string[] = [];
  const { server } = trashed("admin", {
    "GET /api/v0/workspaces/lab/notebooks": () => {
      reads.push("GET notebooks");
      return json({ data: [{ ...notebookJSON, role }] });
    },
    "POST /api/v0/nodes/*/restore": () => problem(403, "forbidden"),
  });
  renderApp(trashPath, server.app);
  await screen.findByRole("list", { name: "Trash" });
  role = "reader";
  reads.length = 0;

  await user.click(restoreOf("Linux") as HTMLElement);

  expect((await screen.findByRole("alert")).textContent).toBe("You do not have permission to do this.");
  await waitFor(() => expect(screen.queryByRole("button", { name: /^Restore/ })).toBeNull());
  expect(reads).toEqual(["GET notebooks"]);
  expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Trash" }));
});

test("a restore refused as not found says why, and reads the notebooks again: one out of reach by now leaves", async () => {
  const user = userEvent.setup();
  const reads: string[] = [];
  let listed = [notebookJSON];
  const { server } = trashed("admin", {
    "GET /api/v0/workspaces/lab/notebooks": () => {
      reads.push("GET notebooks");
      return json({ data: listed });
    },
  });
  renderApp(trashPath, server.app);
  await screen.findByRole("list", { name: "Trash" });
  server.notebookGone = true;
  reads.length = 0;

  await user.click(restoreOf("Linux") as HTMLElement);

  expect((await screen.findByRole("alert")).textContent).toBe("This page does not exist, or you have no access to it.");
  expect(reads).toEqual(["GET notebooks"]);

  listed = [];
  await user.click(restoreOf("Linux") as HTMLElement);
  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
});

test("Restore pressed twice sends one restore: busy while it is out", async () => {
  const user = userEvent.setup();
  const { server } = trashed();
  renderApp(trashPath, server.app);
  await screen.findByRole("list", { name: "Trash" });
  server.restoresHeld = true;
  const restore = restoreOf("Linux") as HTMLElement;

  await user.dblClick(restore);
  await user.click(restore);

  await waitFor(() => expect(server.sent.filter((each) => each.startsWith("RESTORE"))).toEqual(["RESTORE Linux"]));
  expect(restore.getAttribute("aria-busy")).toBe("true");
  expect(restore.getAttribute("aria-disabled")).toBe("true");
  act(() => server.release());
  await waitFor(() => expect(screen.getByRole("status").textContent).toContain("Restored “Linux”"));
  expect(server.sent.filter((each) => each.startsWith("RESTORE"))).toEqual(["RESTORE Linux"]);
});

const pagesEvent = (tree: boolean) => ({
  workspace_id: workspaceJSON.id,
  notebook_id: notebookJSON.id,
  tree,
  pages: [],
});

test("the trash is read again as the tree changes, a deletion or a restore elsewhere, and on each connection; not for a content written", async () => {
  const events = eventServer();
  const { server } = trashed("admin", { "GET /api/v0/events": events.answer });
  renderApp(trashPath, withEvents(server.app), { eventHandlers });
  await screen.findByRole("list", { name: "Trash" });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  events.last().hello();
  await waitFor(() => expect(server.sent.filter((each) => each === "GET trash")).toHaveLength(2));

  server.restoreElsewhere(linux.id);
  events.last().send("pages", pagesEvent(true));
  await waitFor(() => expect(rows()).toHaveLength(2));
  expect(server.sent.filter((each) => each === "GET trash")).toHaveLength(3);

  events.last().send("pages", pagesEvent(false));
  await act(() => new Promise((resolve) => setTimeout(resolve, 600)));
  expect(server.sent.filter((each) => each === "GET trash")).toHaveLength(3);
});

test("a page deleted in the tree and restored from the trash in the same tab is in the tree again, and its page opens", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  renderApp(pagePath(install.id), server.app);
  const tree = await screen.findByRole("navigation", { name: "Pages of Plans" });
  await user.click(await within(tree).findByRole("button", { name: "Actions for Install" }));
  await user.click(screen.getByRole("menuitem", { name: "Delete" }));
  await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }));
  // Its shell goes to its parent, arrived at.
  expect(await screen.findByRole("heading", { level: 1, name: "Guide" })).toBeTruthy();
  await waitFor(() => expect(within(tree).queryByRole("link", { name: "Install" })).toBeNull());

  await user.click(
    within(screen.getByRole("navigation", { name: "More of Plans" })).getByRole("link", { name: "Trash" })
  );
  await user.click(await screen.findByRole("button", { name: "Restore “Install” (From: Guide)" }));
  await user.click(await within(screen.getByRole("status")).findByRole("link", { name: "Open" }));

  expect(await screen.findByRole("heading", { level: 1, name: "Install" })).toBe(document.activeElement);
  expect(within(tree).getByRole("link", { name: "Install" }).getAttribute("aria-current")).toBe("page");
});
