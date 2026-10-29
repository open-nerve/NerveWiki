import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect } from "react";
import { Outlet } from "react-router";
import { expect, test, vi } from "vitest";

import { routes } from "../../app/routes";
import { useStore } from "../../stores/context";
import { PageTreeStore } from "../../stores/page-tree.store";
import type { RootStore } from "../../stores/root.store";
import { json, notebookJSON, problem, workspaceJSON, type Answer } from "../../test/fakes";
import { guide, install, notes, pageNode, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

const tree = () => screen.findByRole("navigation", { name: "Pages of Plans" });
const creates = (server: ReturnType<typeof pageServer>) => server.sent.filter((line) => line.startsWith("POST "));

function sourceServer(answers: Record<string, Answer> = {}) {
  return pageServer({ role: "editor", mode: "llm_wiki", sourceRootId: guide.id, answers });
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => (resolve = yes));
  return { promise, resolve };
}

function sourcePage(server: ReturnType<typeof pageServer>) {
  let captured: RootStore | undefined;
  function Capture() {
    const root = useStore();
    useEffect(() => {
      captured = root;
    }, [root]);
    return <Outlet />;
  }
  return {
    ...renderApp(pagePath(install.id), server.app, { routes: [{ Component: Capture, children: routes }] }),
    stores() {
      const root = captured;
      const pages = root?.pagesOf(server.notebook);
      const notebooks = root?.notebooksOf(workspaceJSON);
      if (pages === undefined || notebooks === undefined) throw new Error("the source has no signed-in stores");
      return { pages, notebooks };
    },
  };
}

async function pasteDialog(user: ReturnType<typeof userEvent.setup>, parent = "Guide") {
  await user.click(await within(await tree()).findByRole("button", { name: `Actions for ${parent}` }));
  await user.click(await screen.findByRole("menuitem", { name: "Paste Markdown" }));
  return screen.findByRole("dialog", { name: `Paste Markdown under ${parent}` });
}

function draft(dialog: HTMLElement, title = "Evidence", content = "# Evidence\n\n  original text\n") {
  fireEvent.change(within(dialog).getByRole("textbox", { name: "Title" }), { target: { value: title } });
  fireEvent.change(within(dialog).getByRole("textbox", { name: "Markdown" }), { target: { value: content } });
}

test.each([guide, install])("pasted Markdown creates one titled source under $name and goes to it", async (parent) => {
  const user = userEvent.setup();
  const server = sourceServer();
  const { router } = sourcePage(server);
  const dialog = await pasteDialog(user, parent.name);
  const content = "\uFEFF# Evidence\n\n  original text\n";
  draft(dialog, "  E\u0301vidence  ", "\uFEFF# Evidence\r\n\r\n  original text\r");
  expect(dialog.textContent).toContain("normalizes line endings");
  expect(dialog.textContent).toContain("Upload a .md file");

  await user.click(within(dialog).getByRole("button", { name: "Add source" }));

  const heading = await screen.findByRole("heading", { level: 1, name: "Évidence" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
  const created = server.nodes.find((node) => node.name === "Évidence");
  expect(created).toMatchObject({ parent_id: parent.id, kind: "page" });
  expect(server.contents.get(created!.id)).toEqual({ content, revision: 1 });
  expect(creates(server)).toEqual([`POST Évidence under ${parent.name}`]);
  expect(server.sent.filter((line) => /^(PUT|OPEN) /.test(line))).toEqual([]);
  expect(router.state.location.pathname).toBe(pagePath(created!.id));
});

test.each(["source root", "parent tree"] as const)(
  "queued ordinary creation does not auto-name a source after the %s changes",
  async (change) => {
    const user = userEvent.setup();
    const requested = deferred<void>();
    const answer = deferred<Response>();
    const busy = pageNode(73, "Busy");
    const server = sourceServer({
      "POST /api/v0/nodes/*/move": () => {
        requested.resolve();
        return answer.promise;
      },
    });
    server.nodes.push(busy);
    const rendered = sourcePage(server);
    const nav = await tree();
    const actions = await within(nav).findByRole("button", { name: "Actions for Notes" });
    const { pages, notebooks } = rendered.stores();
    // A legal ordinary move occupies the actual tree queue; page creation still uses the normal fake HTTP route.
    const moving = pages.move(busy.id, { parent_id: null });
    await requested.promise;
    const create = vi.spyOn(pages, "create");
    await user.click(actions);
    await user.click(await screen.findByRole("menuitem", { name: "New subpage" }));
    expect(create).toHaveBeenCalledOnce();
    expect(creates(server)).toEqual([]);
    const pending = create.mock.results[0]?.value as Promise<unknown>;

    if (change === "source root") {
      server.notebook = { ...server.notebook, source_root_id: notes.id };
      act(() => notebooks.receive(server.notebook));
      expect(pages.notebook?.source_root_id).toBe(notes.id);
    } else {
      server.nodes = server.nodes.map((node) =>
        node.id === notes.id ? Object.assign({}, node, { parent_id: guide.id }) : node
      );
      await act(() => pages.load());
      expect(pages.nodeById(notes.id)?.parent_id).toBe(guide.id);
    }
    const from = rendered.router.state.location.pathname;
    await act(async () => {
      answer.resolve(json(busy));
      await Promise.allSettled([moving, pending]);
    });

    expect(creates(server)).toEqual([]);
    expect(server.nodes.some((node) => node.name === "Untitled")).toBe(false);
    expect(rendered.router.state.location.pathname).toBe(from);
  }
);

test.each([
  [" \t ", "Required."],
  ["界".repeat(86), "At most 255 bytes"],
  ["bad/name", "Cannot contain"],
  ["CON.txt", "Windows reserves this name"],
])("pasted Markdown keeps an invalid title %j in the form without writing", async (title, why) => {
  const user = userEvent.setup();
  const server = sourceServer();
  sourcePage(server);
  const dialog = await pasteDialog(user);
  draft(dialog, title);

  await user.click(within(dialog).getByRole("button", { name: "Add source" }));

  const input = within(dialog).getByRole("textbox", { name: "Title" });
  expect(input.getAttribute("aria-invalid")).toBe("true");
  expect(dialog.textContent).toContain(why);
  expect(document.activeElement).toBe(input);
  expect(creates(server)).toEqual([]);
  expect(within(dialog).getByRole("textbox", { name: "Markdown" })).toHaveProperty(
    "value",
    "# Evidence\n\n  original text\n"
  );
});

test("pasted Markdown keeps a taken title and the body until the user explicitly chooses another", async () => {
  const user = userEvent.setup();
  const server = sourceServer();
  server.nodes.push(pageNode(70, "Evidence", guide));
  sourcePage(server);
  const dialog = await pasteDialog(user);
  draft(dialog);

  await user.click(within(dialog).getByRole("button", { name: "Add source" }));

  await waitFor(() =>
    expect(within(dialog).getByRole("textbox", { name: "Title" }).getAttribute("aria-invalid")).toBe("true")
  );
  expect(creates(server)).toEqual(["POST Evidence under Guide"]);
  expect(within(dialog).getByRole("textbox", { name: "Title" })).toHaveProperty("value", "Evidence");
  expect(within(dialog).getByRole("textbox", { name: "Markdown" })).toHaveProperty(
    "value",
    "# Evidence\n\n  original text\n"
  );
  draft(dialog, "Chosen by me");
  await user.click(within(dialog).getByRole("button", { name: "Add source" }));
  await screen.findByRole("heading", { level: 1, name: "Chosen by me" });
  expect(creates(server)).toEqual(["POST Evidence under Guide", "POST Chosen by me under Guide"]);
});

test("pasted Markdown shows a server title refusal without clearing the draft", async () => {
  const user = userEvent.setup();
  const server = sourceServer({
    [`POST /api/v0/notebooks/${notebookJSON.id}/pages`]: () =>
      problem(422, "validation_failed", { errors: [{ field: "title", code: "not_allowed", message: "reserved" }] }),
  });
  sourcePage(server);
  const dialog = await pasteDialog(user);
  draft(dialog);

  await user.click(within(dialog).getByRole("button", { name: "Add source" }));

  await waitFor(() => expect(dialog.textContent).toContain("Windows reserves this name"));
  expect(within(dialog).getByRole("textbox", { name: "Title" })).toHaveProperty("value", "Evidence");
  expect(within(dialog).getByRole("textbox", { name: "Markdown" })).toHaveProperty(
    "value",
    "# Evidence\n\n  original text\n"
  );
});

test.each(["closed", "role changed"] as const)(
  "a queued paste sends nothing after its dialog is %s",
  async (change) => {
    const user = userEvent.setup();
    const requested = deferred<void>();
    const answer = deferred<Response>();
    const posted: string[] = [];
    const server = sourceServer({
      [`POST /api/v0/notebooks/${notebookJSON.id}/pages`]: async (request) => {
        const { title } = (await request.json()) as { title: string };
        posted.push(title);
        requested.resolve();
        return answer.promise;
      },
    });
    const rendered = sourcePage(server);
    const dialog = await pasteDialog(user);
    const { pages, notebooks } = rendered.stores();
    const waiting = pages.create(null, "Busy");
    await requested.promise;
    const create = vi.spyOn(pages, "createWithContent");
    draft(dialog);
    await user.click(within(dialog).getByRole("button", { name: "Add source" }));
    expect(create).toHaveBeenCalledOnce();
    const pending = create.mock.results[0]?.value as Promise<unknown>;

    if (change === "closed") {
      await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    } else {
      server.notebook = { ...server.notebook, role: "reader" };
      act(() => notebooks.receive(server.notebook));
    }
    const busy = pageNode(71, "Busy");
    server.nodes.push(busy);
    answer.resolve(
      json({ ...busy, ancestors: [], revision: 1, byte_size: 0, content_updated_at: "", content_updated_by: "" }, 201)
    );
    await act(async () => {
      await waiting;
      await Promise.allSettled([pending]);
    });
    expect(posted).toEqual(["Busy"]);
    expect(server.nodes.some((node) => node.name === "Evidence")).toBe(false);
  }
);

test.each(["closed", "another route", "opened again"] as const)(
  "a sent paste only refreshes its tree after its dialog is %s",
  async (change) => {
    const user = userEvent.setup();
    const requested = deferred<void>();
    const answer = deferred<Response>();
    let signal: AbortSignal | undefined;
    const server = sourceServer({
      [`POST /api/v0/notebooks/${notebookJSON.id}/pages`]: (request) => {
        signal = request.signal;
        requested.resolve();
        return answer.promise;
      },
    });
    const rendered = sourcePage(server);
    const dialog = await pasteDialog(user);
    const create = vi.spyOn(PageTreeStore.prototype, "createWithContent");
    draft(dialog);
    await user.click(within(dialog).getByRole("button", { name: "Add source" }));
    await requested.promise;
    expect(create).toHaveBeenCalledOnce();
    const pending = create.mock.results[0]?.value as Promise<unknown>;

    if (change === "another route") {
      await act(() => rendered.router.navigate(pagePath(notes.id)));
    } else {
      await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
      if (change === "opened again") draft(await pasteDialog(user), "A new draft", "keep this draft");
    }
    expect(signal?.aborted).toBe(false);
    const from = rendered.router.state.location.pathname;
    const destinations: string[] = [];
    const stop = rendered.router.subscribe((state) =>
      destinations.push((state.navigation.location ?? state.location).pathname)
    );
    const created = pageNode(72, "Evidence", guide);
    server.nodes.push(created);
    answer.resolve(
      json(
        { ...created, ancestors: [], revision: 1, byte_size: 9, content_updated_at: "", content_updated_by: "" },
        201
      )
    );
    await act(async () => pending);
    stop();

    expect(rendered.stores().pages.nodeById(created.id)).toBeDefined();
    expect(destinations).not.toContain(pagePath(created.id));
    expect(rendered.router.state.location.pathname).toBe(from);
    if (change === "opened again") {
      const next = screen.getByRole("dialog", { name: "Paste Markdown under Guide" });
      expect(within(next).getByRole("textbox", { name: "Title" })).toHaveProperty("value", "A new draft");
      expect(within(next).getByRole("textbox", { name: "Markdown" })).toHaveProperty("value", "keep this draft");
    }
  }
);
