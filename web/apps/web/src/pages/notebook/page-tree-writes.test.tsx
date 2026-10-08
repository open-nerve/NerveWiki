import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { json, notebookJSON, problem } from "../../test/fakes";
import {
  ada,
  assetNode,
  bob,
  guide,
  install,
  linux,
  notes,
  pageNode,
  pagePath,
  pageServer,
} from "../../test/page-server";
import { renderApp } from "../../test/render";

// The tree's writes: new pages, renaming, moving, deleting (M4/P5 design
// 3.7).

afterEach(() => vi.useRealTimers());

const home = `/lab/notebooks/${notebookJSON.id}`;
const tree = () => screen.findByRole("navigation", { name: "Pages of Plans" });
const titles = (nav: HTMLElement) =>
  within(nav)
    .getAllByRole("link")
    .map((link) => link.textContent);

/** The options of select, each as its text. */
const options = (select: HTMLElement) =>
  within(select)
    .getAllByRole("option")
    .map((option) => option.textContent);

/**
 * lateTreeServer is a page server that answers the tree's reads only a
 * while later, as over a network: a page deleted leaves the tree before
 * the deletion settles.
 */
function lateTreeServer() {
  const server = pageServer({
    answers: {
      [`GET /api/v0/notebooks/${notebookJSON.id}/nodes`]: async () => {
        await new Promise((resolve) => setTimeout(resolve, 30));
        return json({ data: server.nodes });
      },
    },
  });
  return server;
}

/** Opens the menu of the page named name and chooses item. */
async function choose(user: ReturnType<typeof userEvent.setup>, name: string, item: string) {
  await user.click(await within(await tree()).findByRole("button", { name: `Actions for ${name}` }));
  await user.click(await screen.findByRole("menuitem", { name: item }));
}

test("New page creates an Untitled page at the root and goes to it, arrived at", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  renderApp(home, server.app);

  await user.click(await within(await tree()).findByRole("button", { name: "New page" }));

  const heading = await screen.findByRole("heading", { level: 1, name: "Untitled" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
  expect(server.sent).toContain("POST Untitled under root");
  expect(titles(await tree())).toEqual(["Guide", "Notes", "Untitled"]);
  expect(
    within(await tree())
      .getByRole("link", { name: "Untitled" })
      .getAttribute("aria-current")
  ).toBe("page");
});

test("a title taken among the siblings, or answered taken, has the creation try the next", async () => {
  const user = userEvent.setup();
  let refused = false;
  const racing = pageServer({
    nodes: [guide, notes, pageNode(20, "UNTITLED")],
    answers: {
      [`POST /api/v0/notebooks/${notebookJSON.id}/pages`]: async (request) => {
        const { title } = (await request.clone().json()) as { title: string };
        racing.sent.push(`POST ${title}`);
        if (!refused) {
          refused = true;
          return problem(409, "page.title_taken");
        }
        const node = { ...pageNode(40, title) };
        racing.nodes = [...racing.nodes, node];
        return json(
          { ...node, ancestors: [], revision: 1, byte_size: 0, content_updated_at: "", content_updated_by: "" },
          201
        );
      },
    },
  });
  renderApp(home, racing.app);

  await user.click(await within(await tree()).findByRole("button", { name: "New page" }));

  expect(await screen.findByRole("heading", { level: 1, name: "Untitled 3" })).toBeTruthy();
  expect(racing.sent.filter((line) => line.startsWith("POST"))).toEqual(["POST Untitled 2", "POST Untitled 3"]);
});

test("an attachment beside the new page is not in the tree, and its name is taken: the creation skips it (M7/P2 design 3.10)", async () => {
  const user = userEvent.setup();
  const server = pageServer({ nodes: [guide, notes, assetNode(30, "UNTITLED")] });
  renderApp(home, server.app);

  await user.click(await within(await tree()).findByRole("button", { name: "New page" }));

  expect(await screen.findByRole("heading", { level: 1, name: "Untitled 2" })).toBeTruthy();
  expect(server.sent.filter((line) => line.startsWith("POST"))).toEqual(["POST Untitled 2 under root"]);
  expect(titles(await tree())).toEqual(["Guide", "Notes", "Untitled 2"]);
});

test("a creation gives up after three titles taken, and says why", async () => {
  const user = userEvent.setup();
  const posted: string[] = [];
  const server = pageServer({
    answers: {
      [`POST /api/v0/notebooks/${notebookJSON.id}/pages`]: async (request) => {
        posted.push(((await request.clone().json()) as { title: string }).title);
        return problem(409, "page.title_taken");
      },
    },
  });
  renderApp(home, server.app);
  const nav = await tree();

  await user.click(await within(nav).findByRole("button", { name: "New page" }));

  expect(await within(nav).findByRole("alert")).toBeTruthy();
  expect(posted).toEqual(["Untitled", "Untitled 2", "Untitled 3"]);
  expect(screen.getByRole("heading", { level: 1, name: "Plans" })).toBeTruthy();
});

test("New subpage creates under the page, which opens", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  renderApp(home, server.app);

  await choose(user, "Notes", "New subpage");

  expect(await screen.findByRole("heading", { level: 1, name: "Untitled" })).toBeTruthy();
  expect(server.sent).toContain("POST Untitled under Notes");
  expect(
    within(await tree())
      .getByRole("button", { name: "Subpages of Notes" })
      .getAttribute("aria-expanded")
  ).toBe("true");
});

test("Rename renames in a dialog, which closes; the focus goes back to the menu's button", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  renderApp(home, server.app);

  await choose(user, "Notes", "Rename");
  const dialog = await screen.findByRole("dialog", { name: "Rename Notes" });
  const field = within(dialog).getByLabelText("Title");
  await user.clear(field);
  await user.type(field, "Journal");
  await user.click(within(dialog).getByRole("button", { name: "Save" }));

  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(server.sent).toContain("PATCH Notes to Journal");
  expect(titles(await tree())).toEqual(["Guide", "Journal"]);
  await waitFor(() =>
    expect(document.activeElement).toBe(
      within(screen.getByRole("navigation", { name: "Pages of Plans" })).getByRole("button", {
        name: "Actions for Journal",
      })
    )
  );
});

test("a title taken, or one the rules refuse, stays in the rename dialog", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  renderApp(home, server.app);
  await choose(user, "Notes", "Rename");
  const dialog = await screen.findByRole("dialog", { name: "Rename Notes" });
  const field = within(dialog).getByLabelText("Title");

  await user.clear(field);
  await user.click(within(dialog).getByRole("button", { name: "Save" }));
  expect(await within(dialog).findByText("Required.")).toBeTruthy();
  expect(server.sent.some((line) => line.startsWith("PATCH"))).toBe(false);

  await user.type(field, "a/b");
  await user.click(within(dialog).getByRole("button", { name: "Save" }));
  expect(await within(dialog).findByText(/^Cannot contain/)).toBeTruthy();

  await user.clear(field);
  await user.type(field, "guide");
  await user.click(within(dialog).getByRole("button", { name: "Save" }));
  expect(await within(dialog).findByRole("alert")).toBeTruthy();
  expect(server.sent).toContain("PATCH Notes to guide");
  expect(screen.getByRole("dialog", { name: "Rename Notes" })).toBeTruthy();
});

/** linking.pages_locked of Linux, which Bob edits, and Notes, which Ada does: the links a rename or move writes again. */
const pagesLocked = () =>
  problem(409, "linking.pages_locked", {
    locks: [
      { page_id: linux.id, ...bob },
      { page_id: notes.id, ...ada },
    ],
  });

/** A second Notes, under Guide: the tree tells the two apart by where they are. */
const otherNotes = pageNode(5, "Notes", guide);

test("a rename whose links' pages are being edited names them, as the tree does, and their editors in its dialog", async () => {
  const user = userEvent.setup();
  const server = pageServer({
    nodes: [guide, install, linux, notes, otherNotes],
    answers: { "PATCH /api/v0/nodes/*": pagesLocked },
  });
  renderApp(home, server.app);
  await choose(user, "Guide", "Rename");
  const dialog = await screen.findByRole("dialog", { name: "Rename Guide" });
  await user.clear(within(dialog).getByLabelText("Title"));
  await user.type(within(dialog).getByLabelText("Title"), "Handbook");
  await user.click(within(dialog).getByRole("button", { name: "Save" }));

  const alert = await within(dialog).findByRole("alert");
  expect(
    within(alert)
      .getAllByRole("listitem")
      .map((item) => item.textContent)
  ).toEqual(["Bob is editing “Linux”.", "You are editing “Notes (in Plans)”."]);
  expect(alert.textContent).toContain("This change would write the links on these pages again");
  expect(server.nodes.map((node) => node.name)).toEqual(["Guide", "Install", "Linux", "Notes", "Notes"]);
});

test("a move whose links' pages are being edited names them, as the tree does, and their editors in its dialog; a busy server says so", async () => {
  const user = userEvent.setup();
  let answer = pagesLocked;
  const server = pageServer({
    nodes: [guide, install, linux, notes, otherNotes],
    answers: { "POST /api/v0/nodes/*/move": () => answer() },
  });
  renderApp(home, server.app);
  await choose(user, "Notes (in Plans)", "Move to…");
  const dialog = await screen.findByRole("dialog", { name: "Move Notes (in Plans)" });
  await user.selectOptions(within(dialog).getByLabelText("Parent page"), "Guide");
  await user.click(within(dialog).getByRole("button", { name: "Move" }));

  const alert = await within(dialog).findByRole("alert");
  expect(
    within(alert)
      .getAllByRole("listitem")
      .map((item) => item.textContent)
  ).toEqual(["Bob is editing “Linux”.", "You are editing “Notes (in Plans)”."]);

  answer = () => problem(503, "server_busy", {}, { "Retry-After": "1" });
  await user.click(within(dialog).getByRole("button", { name: "Move" }));
  await waitFor(() =>
    expect(within(dialog).getByRole("alert").textContent).toBe("The server is busy. Try again in a moment.")
  );
});

test("Delete says how many subpages go with the page; the page shown goes to the parent's place", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  renderApp(pagePath(linux.id), server.app);
  await screen.findByRole("heading", { level: 1, name: "Linux" });

  await choose(user, "Install", "Delete");
  const dialog = await screen.findByRole("alertdialog", { name: "Delete Install?" });
  expect(dialog.textContent).toContain("Its subpages go with it: 1.");
  await user.click(within(dialog).getByRole("button", { name: "Delete" }));

  const heading = await screen.findByRole("heading", { level: 1, name: "Guide" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
  expect(server.sent).toContain("DELETE Install");
  expect(titles(await tree())).toEqual(["Guide", "Notes"]);
});

test("a root page deleted while shown sends its shell to the notebook's home", async () => {
  const user = userEvent.setup();
  renderApp(pagePath(install.id), pageServer().app);
  await screen.findByRole("heading", { level: 1, name: "Install" });

  await choose(user, "Guide", "Delete");
  const dialog = await screen.findByRole("alertdialog", { name: "Delete Guide?" });
  expect(dialog.textContent).toContain("Its subpages go with it: 2.");
  await user.click(within(dialog).getByRole("button", { name: "Delete" }));

  expect(await screen.findByRole("heading", { level: 1, name: "Plans" })).toBeTruthy();
});

test("a deletion someone's edit refuses names them and the page they edit, in the dialog; nothing goes", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  server.hold(linux.id, bob);
  renderApp(pagePath(notes.id), server.app);
  await screen.findByRole("heading", { level: 1, name: "Notes" });

  await choose(user, "Guide", "Delete");
  const dialog = await screen.findByRole("alertdialog", { name: "Delete Guide?" });
  await user.click(within(dialog).getByRole("button", { name: "Delete" }));

  expect((await within(dialog).findByRole("alert")).textContent).toBe("Bob is editing “Linux”.");
  expect(server.nodes).toHaveLength(4);
});

test("a deletion someone's edit refuses names the page they edit as the tree does, by where it is among pages of its title", async () => {
  const user = userEvent.setup();
  const server = pageServer({ nodes: [guide, install, linux, notes, pageNode(6, "Linux")] });
  server.hold(linux.id, bob);
  renderApp(home, server.app);

  await choose(user, "Guide", "Delete");
  const dialog = await screen.findByRole("alertdialog", { name: "Delete Guide?" });
  await user.click(within(dialog).getByRole("button", { name: "Delete" }));

  expect((await within(dialog).findByRole("alert")).textContent).toBe("Bob is editing “Linux (in Guide / Install)”.");
});

test("a deletion cancelled gives the focus back to the menu's button; one done, to the heading, though the tree is read late", async () => {
  const user = userEvent.setup();
  renderApp(pagePath(guide.id), lateTreeServer().app);
  await screen.findByRole("heading", { level: 1, name: "Guide" });

  await choose(user, "Notes", "Delete");
  await user.click(
    within(await screen.findByRole("alertdialog", { name: "Delete Notes?" })).getByRole("button", { name: "Cancel" })
  );
  const actions = within(await tree()).getByRole("button", { name: "Actions for Notes" });
  await waitFor(() => expect(document.activeElement).toBe(actions));

  await choose(user, "Notes", "Delete");
  await user.click(
    within(await screen.findByRole("alertdialog", { name: "Delete Notes?" })).getByRole("button", { name: "Delete" })
  );
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("heading", { level: 2, name: "Plans" })));
  await new Promise((resolve) => setTimeout(resolve, 60));
  expect(document.activeElement).toBe(screen.getByRole("heading", { level: 2, name: "Plans" }));
});

test("the page shown deleted, its parent's page at hand, the focus goes straight to that page's heading", async () => {
  const user = userEvent.setup();
  renderApp(pagePath(linux.id), lateTreeServer().app);
  await screen.findByRole("heading", { level: 1, name: "Linux" });
  await user.click(await within(await tree()).findByRole("link", { name: "Install" }));
  await screen.findByRole("heading", { level: 1, name: "Install" });
  await user.click(within(await tree()).getByRole("link", { name: "Linux" }));
  await screen.findByRole("heading", { level: 1, name: "Linux" });

  // The tree's heading is not on the way: a screen reader would read it first.
  const passed = vi.fn();
  screen.getByRole("heading", { level: 2, name: "Plans" }).addEventListener("focus", passed);

  await choose(user, "Linux", "Delete");
  await user.click(
    within(await screen.findByRole("alertdialog", { name: "Delete Linux?" })).getByRole("button", { name: "Delete" })
  );
  const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
  await new Promise((resolve) => setTimeout(resolve, 60));
  expect(document.activeElement).toBe(heading);
  expect(passed).not.toHaveBeenCalled();
});

test("a page shown whose subtree's parent went too sends its shell to the notebook's home", async () => {
  const user = userEvent.setup();
  const server = pageServer({
    answers: {
      // Another tab deleted Guide in the meantime, and Install with it.
      "DELETE /api/v0/nodes/*": () => {
        server.nodes = [notes];
        return new Response(null, { status: 204 });
      },
    },
  });
  renderApp(pagePath(linux.id), server.app);
  await screen.findByRole("heading", { level: 1, name: "Linux" });

  await choose(user, "Install", "Delete");
  const dialog = await screen.findByRole("alertdialog", { name: "Delete Install?" });
  await user.click(within(dialog).getByRole("button", { name: "Delete" }));

  expect(await screen.findByRole("heading", { level: 1, name: "Plans" })).toBeTruthy();
});

test("a page deleted that is not shown gives the focus to the tree's heading", async () => {
  const user = userEvent.setup();
  renderApp(pagePath(guide.id), pageServer().app);
  await screen.findByRole("heading", { level: 1, name: "Guide" });

  await choose(user, "Notes", "Delete");
  const dialog = await screen.findByRole("alertdialog", { name: "Delete Notes?" });
  expect(dialog.textContent).not.toContain("subpages");
  await user.click(within(dialog).getByRole("button", { name: "Delete" }));

  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("heading", { level: 2, name: "Plans" })));
  expect(screen.getByRole("heading", { level: 1, name: "Guide" })).toBeTruthy();
});

test("a page deleted leaves the tree though it cannot be read again; the page shown goes to the parent's place", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  renderApp(pagePath(linux.id), server.app);
  await screen.findByRole("heading", { level: 1, name: "Linux" });
  server.nodesDown = true;

  await choose(user, "Notes", "Delete");
  await user.click(
    within(await screen.findByRole("alertdialog", { name: "Delete Notes?" })).getByRole("button", { name: "Delete" })
  );
  await waitFor(() =>
    expect(titles(screen.getByRole("navigation", { name: "Pages of Plans" }))).not.toContain("Notes")
  );
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("heading", { level: 2, name: "Plans" })));
  expect(server.sent).toContain("DELETE Notes");

  await choose(user, "Install", "Delete");
  await user.click(
    within(await screen.findByRole("alertdialog", { name: "Delete Install?" })).getByRole("button", { name: "Delete" })
  );
  const heading = await screen.findByRole("heading", { level: 1, name: "Guide" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
  expect(titles(await tree())).toEqual(["Guide"]);
});

test("Move to offers the parents that can hold the page and its places; the page moves", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  renderApp(home, server.app);

  await choose(user, "Guide", "Move to…");
  const dialog = await screen.findByRole("dialog", { name: "Move Guide" });
  const parent = within(dialog).getByLabelText("Parent page");
  expect(options(parent)).toEqual(["The notebook's top level", "Notes"]);
  const position = within(dialog).getByLabelText("Position");
  expect(options(position)).toEqual(["First", "After Notes", "Last"]);
  expect((position as HTMLSelectElement).value).toBe("first");

  await user.selectOptions(parent, "Notes");
  expect(options(position)).toEqual(["First", "Last"]);
  await user.click(within(dialog).getByRole("button", { name: "Move" }));

  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(server.sent).toContain("MOVE Guide under Notes, after last");
  expect(titles(await tree())).toEqual(["Notes", "Guide"]);
  // Under another parent, the page's item is another: its menu's button has the focus.
  const actions = within(await tree()).getByRole("button", { name: "Actions for Guide" });
  await waitFor(() => expect(document.activeElement).toBe(actions));
});

test("a parent the tree, read again, no longer offers goes back to the page's own: what shows goes out", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
  const server = pageServer();
  renderApp(home, server.app);
  await choose(user, "Notes", "Move to…");
  const dialog = await screen.findByRole("dialog", { name: "Move Notes" });
  const parent = within(dialog).getByLabelText("Parent page") as HTMLSelectElement;
  await user.selectOptions(parent, "Guide / Install");

  // Another tab deletes Install.
  server.nodes = [guide, notes];
  await act(() => vi.advanceTimersByTimeAsync(6_000));
  act(() => void window.dispatchEvent(new Event("focus")));
  await waitFor(() => expect(options(parent)).toEqual(["The notebook's top level", "Guide"]));
  expect(parent.value).toBe("");
  expect((within(dialog).getByLabelText("Position") as HTMLSelectElement).value).toBe("last");
  await user.click(within(dialog).getByRole("button", { name: "Move" }));

  await waitFor(() => expect(server.sent).toContain("MOVE Notes under root, after last"));
});

test("a parent the tree, read again, no longer offers goes back to the page's own, not to the top level", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
  const server = pageServer();
  renderApp(pagePath(linux.id), server.app);
  await choose(user, "Linux", "Move to…");
  const dialog = await screen.findByRole("dialog", { name: "Move Linux" });
  const parent = within(dialog).getByLabelText("Parent page") as HTMLSelectElement;
  await user.selectOptions(parent, "Notes");

  // Another tab deletes Notes.
  server.nodes = [guide, install, linux];
  await act(() => vi.advanceTimersByTimeAsync(6_000));
  act(() => void window.dispatchEvent(new Event("focus")));
  await waitFor(() => expect(options(parent)).toEqual(["The notebook's top level", "Guide", "Guide / Install"]));
  expect(parent.value).toBe(install.id);
  await user.click(within(dialog).getByRole("button", { name: "Move" }));

  await waitFor(() => expect(server.sent).toContain("MOVE Linux under Install, after last"));
});

test("a place the tree, read again, no longer offers goes back to last", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
  const server = pageServer();
  renderApp(home, server.app);
  await choose(user, "Notes", "Move to…");
  const dialog = await screen.findByRole("dialog", { name: "Move Notes" });
  const position = within(dialog).getByLabelText("Position") as HTMLSelectElement;
  await user.selectOptions(position, "After Guide");

  // Another tab deletes Guide.
  server.nodes = [notes];
  await act(() => vi.advanceTimersByTimeAsync(6_000));
  act(() => void window.dispatchEvent(new Event("focus")));
  await waitFor(() => expect(options(position)).toEqual(["First", "Last"]));
  expect(position.value).toBe("last");
  await user.click(within(dialog).getByRole("button", { name: "Move" }));

  await waitFor(() => expect(server.sent).toContain("MOVE Notes under root, after last"));
});

test("a move refused stays in its dialog", async () => {
  const user = userEvent.setup();
  const server = pageServer({ answers: { "POST /api/v0/nodes/*/move": () => problem(409, "page.too_deep") } });
  renderApp(home, server.app);

  await choose(user, "Notes", "Move to…");
  const dialog = await screen.findByRole("dialog", { name: "Move Notes" });
  await user.selectOptions(within(dialog).getByLabelText("Parent page"), "Guide / Install / Linux");
  await user.click(within(dialog).getByRole("button", { name: "Move" }));

  expect(await within(dialog).findByRole("alert")).toBeTruthy();
});

test.each([
  ["parent_id", "Parent page", "No longer a page of this notebook. Choose another."],
  ["after_id", "Position", "No longer under this parent. Choose another."],
])("a %s refused shows why under its select, which gets the focus, and nothing above", async (field, label, why) => {
  const user = userEvent.setup();
  const server = pageServer({
    answers: {
      "POST /api/v0/nodes/*/move": () =>
        problem(422, "validation_failed", { errors: [{ field, code: "not_allowed", message: "refused" }] }),
    },
  });
  renderApp(home, server.app);

  await choose(user, "Guide", "Move to…");
  const dialog = await screen.findByRole("dialog", { name: "Move Guide" });
  await user.selectOptions(within(dialog).getByLabelText("Parent page"), "Notes");
  await user.click(within(dialog).getByRole("button", { name: "Move" }));

  const select = within(dialog).getByLabelText(label);
  await waitFor(() => expect(document.activeElement).toBe(select));
  expect(select.getAttribute("aria-invalid")).toBe("true");
  expect(document.getElementById(select.getAttribute("aria-describedby") ?? "")?.textContent).toBe(why);
  expect(within(dialog).queryByRole("alert")).toBeNull();
});

test("a writer's row drags from anywhere in it, its title too; a reader's does not drag", async () => {
  const { unmount } = renderApp(home, pageServer().app);
  const link = await within(await tree()).findByRole("link", { name: "Guide" });
  expect(link.parentElement?.getAttribute("draggable")).toBe("true");
  expect(link.getAttribute("draggable")).toBe("false");
  unmount();

  renderApp(home, pageServer({ role: "reader" }).app);
  const read = await within(await tree()).findByRole("link", { name: "Guide" });
  expect(read.parentElement?.hasAttribute("draggable")).toBe(false);
  expect(read.hasAttribute("draggable")).toBe(false);
});

test("a reader has neither New page nor the pages' menus", async () => {
  renderApp(home, pageServer({ role: "reader" }).app);

  const nav = await tree();
  await within(nav).findByRole("link", { name: "Guide" });
  expect(within(nav).queryByRole("button", { name: "New page" })).toBeNull();
  expect(within(nav).queryByRole("button", { name: /^Actions for/ })).toBeNull();
  expect(within(screen.getByRole("main")).queryByRole("button", { name: "New page" })).toBeNull();
});

test("the home's New page creates at the root", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  renderApp(home, server.app);

  await user.click(await within(screen.getByRole("main")).findByRole("button", { name: "New page" }));

  expect(await screen.findByRole("heading", { level: 1, name: "Untitled" })).toBeTruthy();
  expect(server.sent).toContain("POST Untitled under root");
});

test("the home says at once why a creation is refused otherwise, until the next one is out", async () => {
  const user = userEvent.setup();
  const posted: string[] = [];
  /** While hold is set, a creation's answer waits in waiting. */
  let hold = false;
  const waiting: ((answer: Response) => void)[] = [];
  renderApp(
    home,
    pageServer({
      answers: {
        [`POST /api/v0/notebooks/${notebookJSON.id}/pages`]: async (request) => {
          posted.push(((await request.clone().json()) as { title: string }).title);
          return hold ? new Promise<Response>((resolve) => waiting.push(resolve)) : problem(500, "internal_error");
        },
      },
    }).app
  );
  const main = await screen.findByRole("main");
  const button = await within(main).findByRole("button", { name: "New page" });

  await user.click(button);
  expect((await within(main).findByRole("alert")).textContent).toContain("Something went wrong on the server.");
  expect(posted).toEqual(["Untitled"]);

  hold = true;
  await user.click(button);
  await waitFor(() => expect(within(main).queryByRole("alert")).toBeNull());
  await waitFor(() => expect(waiting).toHaveLength(1));
  waiting[0]?.(problem(500, "internal_error"));
  expect(await within(main).findByRole("alert")).toBeTruthy();
});

test("a creation answered once the tab went elsewhere stays there", async () => {
  const user = userEvent.setup();
  const waiting: ((answer: Response) => void)[] = [];
  const server = pageServer({
    answers: {
      [`POST /api/v0/notebooks/${notebookJSON.id}/pages`]: () =>
        new Promise<Response>((resolve) => waiting.push(resolve)),
    },
  });
  const { router } = renderApp(home, server.app);
  const nav = await tree();

  await user.click(await within(nav).findByRole("button", { name: "New page" }));
  await user.click(within(nav).getByRole("link", { name: "Notes" }));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  await waitFor(() => expect(waiting).toHaveLength(1));
  const created = pageNode(60, "Untitled");
  // Every place the router is sent to from now on: a navigation is in its state as soon as it starts.
  const sent: string[] = [];
  const stop = router.subscribe((state) => sent.push((state.navigation.location ?? state.location).pathname));
  waiting[0]?.(json({ ...created, ancestors: [], revision: 1, byte_size: 0 }, 201));

  // The creation goes to the new page before its button comes back.
  await waitFor(() => expect(within(nav).getByRole("button", { name: "New page" })).toHaveProperty("disabled", false));
  stop();
  expect(sent).not.toContain(pagePath(created.id));
  expect(router.state.location.pathname).toBe(pagePath(notes.id));
  expect(screen.getByRole("heading", { level: 1, name: "Notes" })).toBeTruthy();
});

test("the tree's failure goes once the next creation goes through", async () => {
  const user = userEvent.setup();
  let refuse = true;
  renderApp(
    home,
    pageServer({
      answers: {
        [`POST /api/v0/notebooks/${notebookJSON.id}/pages`]: () =>
          refuse
            ? problem(500, "internal_error")
            : json({ ...pageNode(61, "Untitled"), ancestors: [], revision: 1, byte_size: 0 }, 201),
      },
    }).app
  );
  const nav = await tree();

  await user.click(await within(nav).findByRole("button", { name: "New page" }));
  expect(await within(nav).findByRole("alert")).toBeTruthy();
  refuse = false;
  await user.click(within(nav).getByRole("button", { name: "New page" }));

  await waitFor(() => expect(within(nav).queryByRole("alert")).toBeNull());
});

test("a page ten levels down offers no New subpage", async () => {
  const user = userEvent.setup();
  const levels = [pageNode(70, "L1")];
  for (let i = 2; i <= 10; i++) {
    levels.push(pageNode(69 + i, `L${i}`, levels[levels.length - 1]));
  }
  renderApp(pagePath(levels[9]?.id ?? ""), pageServer({ nodes: levels }).app);
  await screen.findByRole("heading", { level: 1, name: "L10" });

  await user.click(await within(await tree()).findByRole("button", { name: "Actions for L10" }));
  expect((await screen.findByRole("menuitem", { name: "New subpage" })).getAttribute("aria-disabled")).toBe("true");
  await user.keyboard("{Escape}");
  await user.click(within(await tree()).getByRole("button", { name: "Actions for L9" }));
  expect((await screen.findByRole("menuitem", { name: "New subpage" })).hasAttribute("aria-disabled")).toBe(false);
});
