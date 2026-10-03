import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { json, notebookJSON, problem } from "../../test/fakes";
import { guide, install, linux, notes, pageNode, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The tree's writes: new pages, renaming, moving, deleting (M4/P5 design
// 3.7).

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

test("a page deleted while the tree cannot be read again stays listed, and its menu works again", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  renderApp(pagePath(guide.id), server.app);
  await screen.findByRole("heading", { level: 1, name: "Guide" });

  server.nodesDown = true;
  await choose(user, "Notes", "Delete");
  const dialog = await screen.findByRole("alertdialog", { name: "Delete Notes?" });
  await user.click(within(dialog).getByRole("button", { name: "Delete" }));
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(server.sent).toContain("DELETE Notes");
  expect(titles(await tree())).toContain("Notes");

  await choose(user, "Notes", "Delete");
  const again = await screen.findByRole("alertdialog", { name: "Delete Notes?" });
  expect((within(again).getByRole("button", { name: "Delete" }) as HTMLButtonElement).disabled).toBe(false);
  await user.click(within(again).getByRole("button", { name: "Cancel" }));

  const menu = within(await tree()).getByRole("button", { name: "Actions for Notes" });
  await waitFor(() => expect(document.activeElement).toBe(menu));
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
