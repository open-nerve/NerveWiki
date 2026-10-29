import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect } from "react";
import { Outlet } from "react-router";
import { expect, test } from "vitest";

import { routes } from "../../app/routes";
import { editorExtensions } from "../../editor/registry";
import { useStore } from "../../stores/context";
import { RootStore } from "../../stores/root.store";
import { attachments, dropped, picker, rows, transfers } from "../../test/attachments";
import { instanceJSON, json, notebookJSON, problem, tokensJSON, workspaceJSON } from "../../test/fakes";
import { pageEditor } from "../../test/page-editor";
import { assetNode, guide, install, notes, pageNode, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";
import { formOf } from "../../test/transfer";

type Server = ReturnType<typeof pageServer>;
const base = [guide, install, notes];
const assetPath = `POST /api/v0/notebooks/${notebookJSON.id}/assets`;
const creates = (server: Server) => server.sent.filter((line) => line.startsWith("POST "));
const uploads = (server: Server) => server.sent.filter((line) => line.startsWith("UPLOAD "));
const assetReads = (server: Server) => server.sent.filter((line) => line.startsWith("GET assets "));

/** Keep the real routes, stores and session; inspect the generation that the UI actually used. */
function renderSources(server: Server, parent = guide.id) {
  let store: RootStore | undefined;
  function CaptureStore() {
    const current = useStore();
    useEffect(() => {
      store = current;
    }, [current]);
    return <Outlet />;
  }
  return {
    ...renderApp(pagePath(parent), server.app, { routes: [{ Component: CaptureStore, children: routes }] }),
    store: () => {
      if (store === undefined) throw new Error("the route has not mounted its stores");
      return store;
    },
  };
}

function choose(section: HTMLElement, files: File[]): void {
  fireEvent.change(picker(section), { target: { files } });
}

async function openUpload(user: ReturnType<typeof userEvent.setup>, parent = "Guide"): Promise<HTMLElement> {
  await user.click(await screen.findByRole("button", { name: `Actions for ${parent}` }));
  await user.click(await screen.findByRole("menuitem", { name: "Upload files" }));
  return screen.findByRole("dialog", { name: `Upload sources under ${parent}` });
}

function contentOf(server: Server, title: string): string | undefined {
  const page = server.nodes.find((node) => node.name === title);
  expect(page, `the created page ${title}`).toMatchObject({ kind: "page" });
  return server.contents.get(page!.id)?.content;
}

function warnsBeforeLeaving(): boolean {
  const event = new Event("beforeunload", { cancelable: true });
  window.dispatchEvent(event);
  return event.defaultPrevented;
}

// P2 §4.4 / mutation #34: dropping the BOM or normalizing any newline changes these literal bytes.
test("Markdown uploads preserve the exact UTF-8 bytes including the BOM and every newline", async () => {
  const server = pageServer({ mode: "llm_wiki", sourceRootId: guide.id, nodes: base });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  const bytes = new Uint8Array([
    0xef, 0xbb, 0xbf, 0x23, 0x20, 0xe4, 0xb8, 0xad, 0x0d, 0x0a, 0x61, 0x0d, 0x62, 0x0a, 0x63, 0x0a,
  ]);

  choose(section, [new File([bytes], "evidence.MD")]);

  await waitFor(() => expect(creates(server)).toEqual(["POST evidence under Guide"]));
  const page = server.nodes.find((node) => node.name === "evidence");
  expect(page).toMatchObject({ parent_id: guide.id, kind: "page" });
  const content = server.contents.get(page!.id)?.content;
  expect(content).toBe("\uFEFF# 中\r\na\rb\nc\n");
  expect([...new TextEncoder().encode(content)]).toEqual([...bytes]);
  expect(uploads(server)).toEqual([]);
});

test("Markdown uploads preserve the exact UTF-8 bytes for empty files and each kind of final newline", async () => {
  const server = pageServer({ mode: "llm_wiki", sourceRootId: guide.id, nodes: base });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  // These expected strings are handwritten, not decoded by the production decoder or its helpers.
  const cases = [
    { title: "empty", bytes: [], content: "" },
    { title: "no-newline", bytes: [0x61], content: "a" },
    { title: "lf", bytes: [0x61, 0x0a], content: "a\n" },
    { title: "cr", bytes: [0x61, 0x0d], content: "a\r" },
    { title: "crlf", bytes: [0x61, 0x0d, 0x0a], content: "a\r\n" },
  ];

  choose(
    section,
    cases.map(({ title, bytes }) => new File([new Uint8Array(bytes)], `${title}.md`))
  );

  await waitFor(() => expect(creates(server)).toHaveLength(5));
  await waitFor(() => expect(within(section).queryByRole("list", { name: "Uploads" })).toBeNull());
  for (const { title, content, bytes } of cases) {
    expect(contentOf(server, title)).toBe(content);
    expect([...new TextEncoder().encode(contentOf(server, title))]).toEqual(bytes);
  }
  expect(uploads(server)).toEqual([]);
});

test("Markdown uploads preserve the exact UTF-8 bytes by refusing malformed UTF-8 without blocking a valid file", async () => {
  const server = pageServer({ mode: "llm_wiki", sourceRootId: guide.id, nodes: base });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  // A truncated four-byte character becomes one U+FFFD in a lenient decoder. Its three replacement bytes would
  // even pass a mere re-encoded-length check; fatal decoding itself must reject this three-byte input.
  choose(section, [
    new File([new Uint8Array([0xf0, 0x9f, 0x92])], "broken.md"),
    new File([new Uint8Array([0x6f, 0x6b])], "valid.md"),
  ]);

  await within(section).findByText("This Markdown file is not valid UTF-8.");
  await waitFor(() => expect(contentOf(server, "valid")).toBe("ok"));
  expect(creates(server)).toEqual(["POST valid under Guide"]);
  expect(server.nodes.some((node) => node.name === "broken")).toBe(false);
  expect(uploads(server)).toEqual([]);
});

test("Markdown uploads preserve the exact UTF-8 bytes at 5 MiB and reject a multibyte file one byte over", async () => {
  const server = pageServer({ mode: "llm_wiki", sourceRootId: guide.id, nodes: base });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  const atLimit = new File([new Uint8Array(5_242_880).fill(0x61)], "limit.md");
  // 1,747,627 three-byte characters are 5,242,881 bytes but fewer than 5 MiB JS characters.
  const over = new File(["中".repeat(1_747_627)], "over.md");
  expect([atLimit.size, over.size]).toEqual([5_242_880, 5_242_881]);

  choose(section, [atLimit, over]);

  await within(section).findByText("The file is larger than this server takes: 5 MB at most.");
  await waitFor(() => expect(server.nodes.some((node) => node.name === "limit")).toBe(true));
  const bytes = new TextEncoder().encode(contentOf(server, "limit"));
  expect(bytes.length).toBe(5_242_880);
  expect(bytes.every((byte) => byte === 0x61)).toBe(true);
  expect(creates(server)).toEqual(["POST limit under Guide"]);
  expect(server.nodes.some((node) => node.name === "over")).toBe(false);
  expect(uploads(server)).toEqual([]);
});

test("source uploads keep their queue and explicit title until empty or illegal suggestions are corrected inline", async () => {
  const user = userEvent.setup();
  const server = pageServer({ mode: "llm_wiki", sourceRootId: guide.id, nodes: base });
  renderSources(server);
  const dialog = await openUpload(user);

  choose(dialog, [
    new File(["valid body"], "legal.md"),
    new File(["empty suggestion body"], ".md"),
    new File(["forbidden suggestion body"], "bad:title.md"),
    new File(["reserved suggestion body"], "CON.md"),
  ]);

  await waitFor(() => expect(contentOf(server, "legal")).toBe("valid body"));
  const blank = within(dialog).getByRole("textbox", { name: "Title for .md" });
  const forbidden = within(dialog).getByRole("textbox", { name: "Title for bad:title.md" });
  const reserved = within(dialog).getByRole("textbox", { name: "Title for CON.md" });
  expect([blank, forbidden, reserved].map((input) => (input as HTMLInputElement).value)).toEqual([
    "",
    "bad:title",
    "CON",
  ]);
  expect(creates(server)).toEqual(["POST legal under Guide"]);
  expect(uploads(server)).toEqual([]);

  await user.type(blank, "Chosen by Ada");
  const row = blank.closest("li")!;
  await user.click(within(row).getByRole("button", { name: "Add source" }));

  await waitFor(() => expect(contentOf(server, "Chosen by Ada")).toBe("empty suggestion body"));
  expect(creates(server)).toEqual(["POST legal under Guide", "POST Chosen by Ada under Guide"]);
  expect(within(dialog).getByRole("textbox", { name: "Title for bad:title.md" })).toBe(forbidden);
  expect(within(dialog).getByRole("textbox", { name: "Title for CON.md" })).toBe(reserved);
});

test("source uploads keep their queue and explicit title after 409 while independent Markdown and attachments finish", async () => {
  const user = userEvent.setup();
  const existing = pageNode(20, "report", guide);
  const server = pageServer({ mode: "llm_wiki", sourceRootId: guide.id, nodes: [...base, existing] });
  server.contents.set(existing.id, { content: "existing report", revision: 1 });
  const view = renderSources(server);
  const dialog = await openUpload(user);
  choose(dialog, [
    new File(["new report\r\n"], "report.md"),
    new File(["independent page"], "other.md"),
    new File(["independent asset"], "scan.png"),
  ]);

  const field = await within(dialog).findByRole("textbox", { name: "Title for report.md" });
  const row = field.closest("li")!;
  await within(row).findByText("Another page or attachment here has this title. Choose another title.");
  await waitFor(() => expect(view.store().assetsOf(server.notebook)!.uploads).toHaveLength(1));
  expect(creates(server).toSorted()).toEqual(["POST other under Guide", "POST report under Guide"]);
  expect(uploads(server)).toEqual(["UPLOAD scan.png under Guide"]);
  expect(contentOf(server, "other")).toBe("independent page");
  expect(server.nodes.find((node) => node.name === "scan.png")).toMatchObject({ kind: "asset", parent_id: guide.id });
  const upload = view.store().assetsOf(server.notebook)!.uploads[0]!;

  await user.clear(field);
  await user.type(field, "Ada's explicit title");
  expect(view.store().assetsOf(server.notebook)!.uploads).toEqual([upload]);
  await user.click(within(dialog).getByRole("button", { name: "Close" }));
  const reopened = await openUpload(user);
  const edited = within(reopened).getByRole("textbox", { name: "Title for report.md" });
  expect((edited as HTMLInputElement).value).toBe("Ada's explicit title");
  expect(view.store().assetsOf(server.notebook)!.uploads).toEqual([upload]);
  await user.click(within(edited.closest("li")!).getByRole("button", { name: "Add source" }));

  await waitFor(() => expect(view.store().assetsOf(server.notebook)!.uploads).toHaveLength(0));
  expect(creates(server).slice(0, 2).toSorted()).toEqual(["POST other under Guide", "POST report under Guide"]);
  expect(creates(server).slice(2)).toEqual(["POST Ada's explicit title under Guide"]);
  expect(contentOf(server, "Ada's explicit title")).toBe("new report\r\n");
  expect(server.contents.get(existing.id)).toEqual({ content: "existing report", revision: 1 });
  expect(server.nodes.some((node) => node.name === "report 2")).toBe(false);
});

test("a failed Markdown upload cannot be retried after becoming reader and resumes when editing is restored", async () => {
  const user = userEvent.setup();
  const existing = pageNode(46, "taken", guide);
  const body = "kept source draft\r\n原文\n";
  const server = pageServer({ role: "editor", mode: "llm_wiki", sourceRootId: guide.id, nodes: [...base, existing] });
  server.contents.set(existing.id, { content: "the existing page", revision: 1 });
  const view = renderSources(server);
  const section = await attachments();
  const file = new File([body], "taken.md");
  choose(section, [file]);
  const field = await within(section).findByRole("textbox", { name: "Title for taken.md" });
  const row = field.closest("li")!;
  await within(row).findByText("Another page or attachment here has this title. Choose another title.");
  const assets = view.store().assetsOf(server.notebook)!;
  const upload = assets.uploads[0]!;
  const pages = view.store().pagesOf(server.notebook)!;
  const notebooks = view.store().notebooksOf(workspaceJSON)!;
  expect(creates(server)).toEqual(["POST taken under Guide"]);
  await user.clear(field);
  await user.type(field, "Kept chosen title");

  act(() => {
    server.notebook = { ...server.notebook, role: "reader" };
    notebooks.receive(server.notebook);
  });
  const retry = within(row).getByRole("button", { name: "Add source" });
  await waitFor(() => expect(field).toHaveProperty("disabled", true));
  expect(retry).toHaveProperty("disabled", true);
  expect(pages.notebook?.role).toBe("reader");
  const form = field.closest("form")!;
  expect(form.isConnected).toBe(true);
  await user.click(retry);
  await act(async () => {
    // A retained callback or scripted submission still reaches the form handler despite disabled controls.
    fireEvent.submit(form);
    await pages.load();
  });
  expect(creates(server)).toEqual(["POST taken under Guide"]);
  expect(server.createRequests).toHaveLength(1);
  expect(assets.uploads).toEqual([upload]);
  expect(upload.file).toBe(file);
  expect(upload.name).toBe("Kept chosen title");
  expect(field).toHaveProperty("value", "Kept chosen title");
  expect(upload.result).toBeUndefined();
  expect(upload.signal.aborted).toBe(false);
  expect(server.nodes.some((node) => node.name === "Kept chosen title")).toBe(false);

  act(() => {
    server.notebook = { ...server.notebook, role: "editor" };
    notebooks.receive(server.notebook);
  });
  await waitFor(() => expect(field).toHaveProperty("disabled", false));
  expect(retry).toHaveProperty("disabled", false);
  await user.click(retry);
  await waitFor(() => expect(assets.uploads).toHaveLength(0));
  expect(creates(server)).toEqual(["POST taken under Guide", "POST Kept chosen title under Guide"]);
  expect(server.createRequests).toHaveLength(2);
  expect(await server.createRequests[1]!.clone().json()).toEqual({
    parent_id: guide.id,
    title: "Kept chosen title",
    content: body,
  });
  expect(contentOf(server, "Kept chosen title")).toBe(body);
  expect(server.contents.get(existing.id)).toEqual({ content: "the existing page", revision: 1 });
  expect(upload.file).toBe(file);
  expect(upload.result).toMatchObject({ kind: "page", page: { name: "Kept chosen title" } });
});

test("a Markdown retry queued before becoming reader is refused before its POST and can be retried after role recovery", async () => {
  const user = userEvent.setup();
  const existing = pageNode(47, "taken", guide);
  const body = "queued source draft\r\n原文\n";
  const server = pageServer({ role: "editor", mode: "llm_wiki", sourceRootId: guide.id, nodes: [...base, existing] });
  server.contents.set(existing.id, { content: "the existing page", revision: 1 });
  const view = renderSources(server);
  const section = await attachments();
  const file = new File([body], "taken.md");
  choose(section, [file]);
  const field = await within(section).findByRole("textbox", { name: "Title for taken.md" });
  await within(field.closest("li")!).findByText(
    "Another page or attachment here has this title. Choose another title."
  );
  const assets = view.store().assetsOf(server.notebook)!;
  const upload = assets.uploads[0]!;
  const notebooks = view.store().notebooksOf(workspaceJSON)!;
  await user.clear(field);
  await user.type(field, "Recovered source");

  // This upload has reached the real create handler and holds the notebook's shared write queue.
  server.createsHeld = true;
  choose(section, [new File(["already sent blocker"], "blocker.md")]);
  await waitFor(() => expect(creates(server)).toEqual(["POST taken under Guide", "POST blocker under Guide"]));
  await user.click(within(field.closest("li")!).getByRole("button", { name: "Add source" }));
  await waitFor(() => expect(within(section).getAllByText("Creating…")).toHaveLength(2));
  expect(server.createRequests).toHaveLength(2);
  expect(upload.failure).toBeUndefined();

  act(() => {
    server.notebook = { ...server.notebook, role: "reader" };
    notebooks.receive(server.notebook);
    server.createsHeld = false;
    server.release();
  });
  await waitFor(() => expect(upload.failure).toMatchObject({ status: 403, code: "forbidden" }));
  await waitFor(() => expect(assets.uploads).toEqual([upload]));
  const retained = await within(section).findByRole("textbox", { name: "Title for taken.md" });
  const retry = within(retained.closest("li")!).getByRole("button", { name: "Add source" });
  expect(retained).toHaveProperty("disabled", true);
  expect(retained).toHaveProperty("value", "Recovered source");
  expect(retry).toHaveProperty("disabled", true);
  expect(upload.file).toBe(file);
  expect(upload.name).toBe("Recovered source");
  expect(upload.result).toBeUndefined();
  expect(upload.signal.aborted).toBe(false);
  expect(creates(server)).toEqual(["POST taken under Guide", "POST blocker under Guide"]);
  expect(server.createRequests).toHaveLength(2);
  expect(contentOf(server, "blocker")).toBe("already sent blocker");
  expect(server.nodes.some((node) => node.name === "Recovered source")).toBe(false);

  act(() => {
    server.notebook = { ...server.notebook, role: "editor" };
    notebooks.receive(server.notebook);
  });
  await waitFor(() => expect(retained).toHaveProperty("disabled", false));
  expect(retry).toHaveProperty("disabled", false);
  await user.click(retry);
  await waitFor(() => expect(assets.uploads).toHaveLength(0));
  expect(creates(server)).toEqual([
    "POST taken under Guide",
    "POST blocker under Guide",
    "POST Recovered source under Guide",
  ]);
  expect(server.createRequests).toHaveLength(3);
  expect(await server.createRequests[2]!.clone().json()).toEqual({
    parent_id: guide.id,
    title: "Recovered source",
    content: body,
  });
  expect(contentOf(server, "Recovered source")).toBe(body);
  expect(server.contents.get(existing.id)).toEqual({ content: "the existing page", revision: 1 });
  expect(upload.file).toBe(file);
  expect(upload.result).toMatchObject({ kind: "page", page: { name: "Recovered source" } });
});

test.each(["initial failure", "explicit retry"] as const)(
  "a Markdown source duplicate's %s links to its page without writing or returning focus to the upload dialog",
  async (attempt) => {
    const user = userEvent.setup();
    const known = pageNode(28, "Known", install);
    const body = "# Known source\r\n原始正文\n";
    const server = pageServer({ mode: "llm_wiki", sourceRootId: guide.id, nodes: [...base, known] });
    server.contents.set(known.id, { content: body, revision: 1 });
    const beforeNodes = [...server.nodes];
    const beforeContents = new Map(server.contents);
    const view = renderSources(server);
    const dialog = await openUpload(user);
    const file = new File([body], "evidence.md");
    choose(dialog, [file]);

    await within(dialog).findByRole("link", { name: "Guide/Install/Known.md" });
    const assets = view.store().assetsOf(server.notebook)!;
    expect(assets.uploads).toHaveLength(1);
    const upload = assets.uploads[0]!;
    const firstFailure = upload.failure;
    expect(firstFailure).toMatchObject({ code: "page.source_duplicate", problem: { duplicate_of: known.id } });
    expect(upload.file).toBe(file);
    expect(creates(server)).toEqual(["POST evidence under Guide"]);
    expect(await server.createRequests[0]!.clone().json()).toEqual({
      parent_id: guide.id,
      title: "evidence",
      content: body,
    });

    if (attempt === "explicit retry") {
      const title = within(dialog).getByRole("textbox", { name: "Title for evidence.md" });
      await user.clear(title);
      await user.type(title, "Another chosen title");
      await user.click(within(title.closest("li")!).getByRole("button", { name: "Add source" }));
      await waitFor(() => {
        expect(upload.failure).toMatchObject({ code: "page.source_duplicate", problem: { duplicate_of: known.id } });
        expect(upload.failure).not.toBe(firstFailure);
      });
      expect(creates(server)).toEqual(["POST evidence under Guide", "POST Another chosen title under Guide"]);
      expect(await server.createRequests[1]!.clone().json()).toEqual({
        parent_id: guide.id,
        title: "Another chosen title",
        content: body,
      });
    }

    expect(server.nodes).toEqual(beforeNodes);
    expect(server.contents).toEqual(beforeContents);
    expect(within(dialog).getByText("This operation was not written.")).toBeTruthy();
    await user.click(within(dialog).getByRole("link", { name: "Guide/Install/Known.md" }));
    await waitFor(() => expect(view.router.state.location.pathname).toBe(pagePath(known.id)));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    const heading = await screen.findByRole("heading", { level: 1, name: "Known" });
    await waitFor(() => expect(document.activeElement).toBe(heading));
    expect(view.router.state.location.state).toEqual({ arrived: true });
    expect(assets.uploads).toEqual([upload]);
    expect(upload.file).toBe(file);
    expect(server.createRequests).toHaveLength(attempt === "initial failure" ? 1 : 2);
    expect(uploads(server)).toEqual([]);
    expect(server.nodes).toEqual(beforeNodes);
    expect(server.contents).toEqual(beforeContents);
  }
);

test("source uploads keep their queue and explicit title after the modal closes and do not take a later route", async () => {
  const user = userEvent.setup();
  const server = pageServer({ mode: "llm_wiki", sourceRootId: guide.id, nodes: base });
  server.createsHeld = true;
  server.uploadsHeld = true;
  const view = renderSources(server);
  const dialog = await openUpload(user);
  choose(dialog, [new File(["kept page"], "kept.md"), new File(["kept asset"], "kept.png")]);
  await waitFor(() => expect(server.createRequests).toHaveLength(1));
  await waitFor(() => expect(transfers(server.app)).toHaveLength(1));
  const assets = view.store().assetsOf(server.notebook)!;
  const going = [...assets.uploads];
  expect(going).toHaveLength(2);
  expect(warnsBeforeLeaving()).toBe(true);

  await user.click(within(dialog).getByRole("button", { name: "Close" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(going.map((upload) => upload.signal.aborted)).toEqual([false, false]);
  expect(server.createRequests[0]!.signal.aborted).toBe(false);
  expect(transfers(server.app)[0]!.aborted).toBe(false);
  const reopened = await openUpload(user);
  expect(within(reopened).getByRole("list", { name: "Uploads" }).querySelectorAll(":scope > li")).toHaveLength(2);
  expect(assets.uploads).toEqual(going);
  await user.click(within(reopened).getByRole("button", { name: "Close" }));
  await act(() => view.router.navigate(pagePath(notes.id)));
  await screen.findByRole("heading", { name: "Notes", level: 1 });
  act(() => server.release());

  await waitFor(() => expect(assets.uploads).toHaveLength(0));
  expect(contentOf(server, "kept")).toBe("kept page");
  expect(server.nodes.find((node) => node.name === "kept.png")).toMatchObject({ kind: "asset", parent_id: guide.id });
  expect(creates(server)).toEqual(["POST kept under Guide"]);
  expect(uploads(server)).toEqual(["UPLOAD kept.png under Guide"]);
  expect(view.router.state.location.pathname).toBe(pagePath(notes.id));
  expect(warnsBeforeLeaving()).toBe(false);
});

test.each(["generation change", "logout"] as const)(
  "source uploads keep their queue and explicit title but %s aborts sent requests and Markdown still waiting in the tree queue",
  async (ending) => {
    const user = userEvent.setup();
    const logoutBodies: unknown[] = [];
    const server = pageServer({
      mode: "llm_wiki",
      sourceRootId: guide.id,
      nodes: base,
      answers: {
        "POST /api/v0/auth/logout": async (request) => {
          logoutBodies.push(await request.json());
          return new Response(null, { status: 204 });
        },
      },
    });
    server.createsHeld = true;
    server.uploadsHeld = true;
    const view = renderSources(server);
    const section = await attachments();
    choose(section, [new File(["first"], "first.md")]);
    await waitFor(() => expect(server.createRequests).toHaveLength(1));
    choose(section, [new File(["queued"], "queued.md"), new File(["image"], "going.png")]);
    await waitFor(() => expect(transfers(server.app)).toHaveLength(1));
    await waitFor(() => expect(within(section).getAllByText("Creating…")).toHaveLength(2));
    const assets = view.store().assetsOf(server.notebook)!;
    const going = [...assets.uploads];
    expect(going).toHaveLength(3);
    expect(creates(server)).toEqual(["POST first under Guide"]);
    expect(going.map((upload) => upload.signal.aborted)).toEqual([false, false, false]);
    expect(warnsBeforeLeaving()).toBe(true);

    if (ending === "logout") {
      await user.click(screen.getByRole("button", { name: "Ada" }));
      await user.click(await screen.findByRole("menuitem", { name: "Sign out" }));
      await screen.findByRole("heading", { name: "Sign in" });
      expect(server.app.session.tokens.state.status).toBe("signed-out");
      expect(logoutBodies).toEqual([{ refresh_token: "rt-1" }]);
    } else {
      const login = server.app.session.tokens.state.loginId;
      await act(() =>
        server.app.session.tokens.signIn({ ...tokensJSON, access_token: "at-next", refresh_token: "rt-next" })
      );
      expect(server.app.session.tokens.state.loginId).not.toBe(login);
      expect(server.app.session.tokens.state.status).toBe("signed-in");
      expect(logoutBodies).toEqual([]);
    }

    await waitFor(() => expect(going.map((upload) => upload.signal.aborted)).toEqual([true, true, true]));
    expect(server.createRequests[0]!.signal.aborted).toBe(true);
    expect(transfers(server.app)[0]!.aborted).toBe(true);
    act(() => server.release());
    await waitFor(() => expect(assets.uploads).toHaveLength(0));
    expect(server.createRequests).toHaveLength(1);
    expect(creates(server)).toEqual(["POST first under Guide"]);
    expect(server.nodes.map((node) => node.name)).toEqual(["Guide", "Install", "Notes"]);
    expect(warnsBeforeLeaving()).toBe(false);
  }
);

test("source uploads keep their queue and explicit title when a waiting Markdown row is cancelled while an independent attachment finishes", async () => {
  const user = userEvent.setup();
  const server = pageServer({ mode: "llm_wiki", sourceRootId: guide.id, nodes: base });
  server.createsHeld = true;
  const view = renderSources(server);
  const section = await attachments();
  choose(section, [new File(["first"], "first.md")]);
  await waitFor(() => expect(server.createRequests).toHaveLength(1));
  choose(section, [new File(["cancelled"], "queued.md"), new File(["independent"], "free.png")]);
  await waitFor(() => expect(within(section).getAllByText("Creating…")).toHaveLength(2));
  const assets = view.store().assetsOf(server.notebook)!;
  await waitFor(() => expect(assets.uploads).toHaveLength(2));
  expect(server.nodes.find((node) => node.name === "free.png")).toMatchObject({ kind: "asset", parent_id: guide.id });
  expect(uploads(server)).toEqual(["UPLOAD free.png under Guide"]);
  expect(server.nodes.some((node) => node.name === "first")).toBe(false);
  const queued = assets.uploads.find((upload) => upload.name === "queued")!;
  expect(queued).toBeDefined();

  await user.click(within(section).getByRole("button", { name: /Cancel.*upload of queued/ }));
  expect(queued.signal.aborted).toBe(true);
  expect(server.createRequests[0]!.signal.aborted).toBe(false);
  act(() => server.release());

  await waitFor(() => expect(assets.uploads).toHaveLength(0));
  expect(contentOf(server, "first")).toBe("first");
  expect(creates(server)).toEqual(["POST first under Guide"]);
  expect(server.createRequests).toHaveLength(1);
  expect(server.nodes.some((node) => node.name === "queued")).toBe(false);
});

test("source uploads keep their queue and explicit title while page success and refusal refresh the tree without asset pagination", async () => {
  const existing = Array.from({ length: 101 }, (_, i) =>
    assetNode(100 + i, `file-${i.toString().padStart(3, "0")}.png`, guide)
  );
  const server = pageServer({ mode: "llm_wiki", sourceRootId: guide.id, nodes: [...base, ...existing] });
  server.createsHeld = true;
  const view = renderSources(server);
  const section = await attachments();
  await waitFor(() => expect(rows(section)).toHaveLength(100));
  const assets = view.store().assetsOf(server.notebook)!;
  expect(assets.listOf(guide.id)?.next).toBe("100");
  expect(assetReads(server)).toEqual(["GET assets Guide"]);
  const treeReads = server.sent.filter((line) => line === "GET nodes").length;
  choose(section, [new File(["one source body"], "created.md")]);
  await waitFor(() => expect(server.createRequests).toHaveLength(1));
  const upload = assets.uploads[0]!;
  act(() => server.release());
  await waitFor(() => expect(assets.uploads).toHaveLength(0));
  const made = server.nodes.find((node) => node.name === "created")!;
  expect(upload.target).toBe("source-page");
  expect(upload.result).toMatchObject({ kind: "page", page: { id: made.id, parent_id: guide.id } });
  expect(view.store().pagesOf(server.notebook)!.byId(made.id)).toMatchObject({ name: "created", parent_id: guide.id });

  // The next creation reaches the fake's source hash rule and is refused; it too must never enter asset paging.
  server.createsHeld = false;
  choose(section, [new File(["one source body"], "duplicate.md")]);
  await within(section).findByRole("link", { name: "Guide/created.md" });
  expect(within(section).getByText("This operation was not written.")).toBeTruthy();
  expect(assets.uploads).toHaveLength(1);
  expect(assets.uploads[0]!.failure).toMatchObject({ code: "page.source_duplicate" });
  expect(creates(server)).toEqual(["POST created under Guide", "POST duplicate under Guide"]);
  expect(server.sent.filter((line) => line === "GET nodes").length).toBe(treeReads + 2);
  expect(assetReads(server)).toEqual(["GET assets Guide"]);
  expect(assets.listOf(guide.id)?.next).toBe("100");
  expect(rows(section)).toHaveLength(100);
  expect(server.nodes.some((node) => node.name === "duplicate")).toBe(false);
});

test.each(["outside", "inside"] as const)(
  "source uploads keep their queue and explicit title while the server decides after their parent moves %s the source area",
  async (destination) => {
    const user = userEvent.setup();
    const folder = pageNode(30, "Source folder", guide);
    const original = pageNode(31, "Original", guide);
    const server = pageServer({ mode: "llm_wiki", sourceRootId: guide.id, nodes: [...base, folder, original] });
    server.contents.set(original.id, { content: "same bytes", revision: 1 });
    server.createsHeld = true;
    const view = renderSources(server, install.id);
    const dialog = await openUpload(user, "Install");
    choose(dialog, [new File(["same bytes"], "new source.md")]);
    await waitFor(() => expect(server.createRequests).toHaveLength(1));
    const assets = view.store().assetsOf(server.notebook)!;
    const upload = assets.uploads[0]!;

    // A different tab's HTTP write uses the real fake handler while this tab's tree still has the old ancestry.
    const to = destination === "outside" ? notes : folder;
    const otherTab = new RootStore(server.app, server.app.session.tokens.state.loginId);
    const otherTree = otherTab.pagesOf(server.notebook)!;
    await otherTree.load();
    await otherTree.move(install.id, { parent_id: to.id });
    expect(server.nodes.find((node) => node.id === install.id)?.parent_id).toBe(to.id);
    expect(view.store().pagesOf(server.notebook)!.byId(install.id)?.parent_id).toBe(guide.id);
    act(() => server.release());

    if (destination === "outside") {
      await waitFor(() => expect(assets.uploads).toHaveLength(0));
      expect(contentOf(server, "new source")).toBe("same bytes");
      expect(server.nodes.find((node) => node.name === "new source")?.parent_id).toBe(install.id);
    } else {
      // Reading the moved tree unmounts the old tree row and its dialog; the page's lasting queue still shows it.
      await waitFor(() => expect(document.contains(dialog)).toBe(false));
      expect(screen.queryByRole("dialog")).toBeNull();
      await within(await attachments()).findByRole("link", { name: "Guide/Original.md" });
      expect(assets.uploads).toEqual([upload]);
      expect(upload.failure).toMatchObject({ code: "page.source_duplicate" });
      expect(server.nodes.some((node) => node.name === "new source")).toBe(false);
    }
    expect(creates(server)).toEqual(["POST new source under Install"]);
    expect(server.sent).toContain(`MOVE Install under ${to.name}, after last`);
    expect(server.contents.get(original.id)).toEqual({ content: "same bytes", revision: 1 });
    expect(view.store().pagesOf(server.notebook)!.byId(install.id)?.parent_id).toBe(to.id);
    expect(view.router.state.location.pathname).toBe(pagePath(install.id));
  }
);

test("source uploads keep their queue and explicit title while ordinary files retain three collision attempts and their own size limit", async () => {
  const names: string[] = [];
  const server = pageServer({
    mode: "llm_wiki",
    sourceRootId: guide.id,
    nodes: base,
    answers: {
      "GET /api/v0/instance": () => json({ ...instanceJSON, asset_max_bytes: 4 }),
      [assetPath]: (request) => {
        names.push(String(formOf(request)?.get("name")));
        return problem(409, "page.title_taken");
      },
    },
  });
  renderSources(server);
  const section = await attachments();
  choose(section, [
    new File(["1234"], "scan.png"),
    new File(["12345"], "too-big.png"),
    new File(["a page larger than four bytes"], "small.md"),
  ]);

  await within(section).findByText("Another page or attachment here has this name, and the names tried after it too.");
  await within(section).findByText("The file is larger than this server takes: 4 B at most.");
  await waitFor(() => expect(contentOf(server, "small")).toBe("a page larger than four bytes"));
  expect(names).toEqual(["scan.png", "scan 2.png", "scan 3.png"]);
  expect(creates(server)).toEqual(["POST small under Guide"]);
  expect(server.nodes.some((node) => node.name === "too-big.png")).toBe(false);
});

test("source uploads keep their queue and explicit title without admitting Markdown through the editor's attachment API", async () => {
  const user = userEvent.setup();
  const server = pageServer({ mode: "llm_wiki", sourceRootId: guide.id, nodes: base });
  renderApp(pagePath(guide.id), server.app, { editorExtensions });
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  const { view } = await pageEditor();
  const before = view.state.doc.toString();
  const event = new Event("paste", { bubbles: true, cancelable: true });
  Object.defineProperty(event, "clipboardData", {
    value: { ...dropped([new File(["must stay out"], "editor.MD")]), getData: () => "" },
  });
  act(() => void view.contentDOM.dispatchEvent(event));

  await screen.findByText("A Markdown file is a page: zip it and import it from the notebook's settings.");
  expect(creates(server)).toEqual([]);
  expect(uploads(server)).toEqual([]);
  expect(view.state.doc.toString()).toBe(before);
  expect(server.nodes).toHaveLength(base.length);
});
