import { EditorView } from "@codemirror/view";
import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, onTestFinished, test, vi } from "vitest";

import { idleLimit } from "../../editor/idle-exit";
import { editorExtensions } from "../../editor/registry";
import { attachments, dropped, nodes, picker, transfers } from "../../test/attachments";
import { instanceJSON, json, notebookJSON } from "../../test/fakes";
import { pageEditor } from "../../test/page-editor";
import { assetJSON, guide, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// Files pasted into the page's editor upload as its attachments, their embeds inserted, through the composition
// root's registry (M7/P4 design 5; the M4 handoff of attachments' extensions, items 2 and 3): the app's
// editorExtensions, loaded as the editor opens.

/** Guide edited by Ada, with the app's extensions, over server: its editor, and the user. */
async function editing(server: ReturnType<typeof pageServer>, user = userEvent.setup()) {
  renderApp(pagePath(guide.id), server.app, { editorExtensions });
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  return { user, ...(await pageEditor()) };
}

/** Time goes by for ms, the timers due run. */
const rest = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));

/** The contents the page was written with. */
const puts = (server: ReturnType<typeof pageServer>) => server.sent.filter((line) => line.startsWith("PUT"));

/** paste pastes files into view's content, as the clipboard has them: files only. */
function paste(view: EditorView, files: File[]) {
  const event = new Event("paste", { bubbles: true, cancelable: true });
  Object.defineProperty(event, "clipboardData", { value: { ...dropped(files), getData: () => "" } });
  act(() => void view.contentDOM.dispatchEvent(event));
}

/** uploadsBy is the uploads' list shown by the editor: before the editor, not in the attachments' section. */
function uploadsBy(content: HTMLElement): HTMLElement | undefined {
  return screen
    .queryAllByRole("list", { name: "Uploads" })
    .find((list) => list.compareDocumentPosition(content) === Node.DOCUMENT_POSITION_FOLLOWING);
}

test("a file pasted uploads as the page's attachment, its row by the editor, and its embed goes where the cursor was", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { view, content } = await editing(server);
  view.dispatch({ selection: { anchor: 0 } });

  paste(view, [new File(["png"], "image.png", { type: "image/png" })]);

  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  expect(within(await attachments()).queryByRole("list", { name: "Uploads" })).toBeNull();
  expect(server.sent.filter((line) => line.startsWith("UPLOAD"))).toEqual([
    expect.stringMatching(/^UPLOAD Pasted image \d{14}\.png under Guide$/),
  ]);
  act(() => server.release());
  await waitFor(() => expect(view.state.doc.toString()).toMatch(/^!\[\[Pasted image \d{14}\.png\]\]Guide/));
  await waitFor(() => expect(uploadsBy(content)).toBeUndefined());
});

test("a Markdown file pasted over a selection keeps it: its row by the editor says why", async () => {
  const server = pageServer({ nodes });
  const { view, content } = await editing(server);
  const before = view.state.doc.toString();
  view.dispatch({ selection: { anchor: 0, head: before.length } });

  paste(view, [new File(["# n"], "notes.md")]);

  await waitFor(() =>
    expect(uploadsBy(content)?.textContent).toContain(
      "A Markdown file is a page: zip it and import it from the notebook's settings."
    )
  );
  expect(view.state.doc.toString()).toBe(before);
  expect(server.sent.filter((line) => line.startsWith("UPLOAD"))).toEqual([]);
});

/** tree is the left column's tree of the notebook's pages, named in English or in Chinese. */
const tree = () => screen.getByRole("navigation", { name: /^(Pages of Plans|Plans的页面)$/u });

test.each<[string, "en" | "zh-CN", { title: string; description: string; stay: string; leave: string }]>([
  [
    "English",
    "en",
    {
      title: "Leave before the files are inserted?",
      description:
        "Files you pasted or dropped are still uploading. They will be added to this page's attachments, but not inserted into it.",
      stay: "Stay",
      leave: "Leave",
    },
  ],
  [
    "Chinese",
    "zh-CN",
    {
      title: "在文件插入之前离开？",
      description: "粘贴或拖入的文件还在上传。它们会照样加进这一页的附件，但不会插入页面。",
      stay: "留下",
      leave: "离开",
    },
  ],
])(
  "a file pasted uploading, going to another page asks first: Stay keeps the edit; Leave goes, the upload goes on, its embed not inserted (%s)",
  async (_, locale, said) => {
    const server = pageServer({ nodes });
    server.uploadsHeld = true;
    const { user, view, content } = await editing(server);
    act(() => server.app.preferences.setLocale(locale));
    view.dispatch({ selection: { anchor: view.state.doc.length } });
    paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
    await waitFor(() => expect(server.sent).toContain("UPLOAD chart.png under Guide"));

    await user.click(within(tree()).getByRole("link", { name: "Notes" }));
    const ask = await screen.findByRole("alertdialog", { name: said.title });
    expect(ask.getAttribute("aria-describedby")).not.toBeNull();
    expect(document.getElementById(ask.getAttribute("aria-describedby") ?? "")?.textContent).toBe(said.description);
    await user.click(within(ask).getByRole("button", { name: said.stay }));
    expect(screen.getByRole("heading", { level: 1, name: "Guide" })).toBeTruthy();
    await waitFor(() => expect(document.activeElement).toBe(content));

    await user.click(within(tree()).getByRole("link", { name: "Notes" }));
    await user.click(
      within(await screen.findByRole("alertdialog", { name: said.title })).getByRole("button", { name: said.leave })
    );
    await screen.findByRole("heading", { level: 1, name: "Notes" });
    act(() => server.release());
    await waitFor(() => expect(server.nodes.some((node) => node.name === "chart.png")).toBe(true));
    await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
    expect(puts(server)).toEqual([]);
  }
);

test("unsaved and uploading, leaving says both", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view } = await editing(server);
  view.dispatch({ changes: { from: view.state.doc.length, insert: "draft" }, selection: { anchor: 0 } });
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(server.sent).toContain("UPLOAD chart.png under Guide"));

  await user.click(within(tree()).getByRole("link", { name: "Notes" }));

  const ask = await screen.findByRole("alertdialog", { name: "Leave without saving?" });
  expect(document.getElementById(ask.getAttribute("aria-describedby") ?? "")?.textContent).toBe(
    "Your changes to this page are not saved. Files you pasted or dropped are still uploading. They will be added to this page's attachments, but not inserted into it."
  );
  act(() => server.release());
});

test("an upload answered, its embed in, asks nothing for itself as its list is read again: leaving says only what is unsaved", async () => {
  let holding = false;
  let release: (() => void) | undefined;
  const server = pageServer({
    nodes,
    answers: {
      [`GET /api/v0/notebooks/${notebookJSON.id}/assets`]: async (request) => {
        if (holding) {
          await new Promise<void>((resolve) => (release = resolve));
        }
        const parent = new URL(request.url).searchParams.get("parent_id");
        return json({
          data: server.nodes
            .filter((node) => node.kind === "asset" && node.parent_id === parent)
            .map((node) => assetJSON(node)),
          next_cursor: null,
        });
      },
    },
  });
  const { user, view } = await editing(server);
  view.dispatch({ selection: { anchor: view.state.doc.length } });
  holding = true;
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  // Answered: its embed goes in as its list is read again, held.
  await waitFor(() => expect(view.state.doc.toString()).toContain("![[chart.png]]"));
  expect(release).toBeDefined();

  await user.click(within(tree()).getByRole("link", { name: "Notes" }));

  const ask = await screen.findByRole("alertdialog", { name: "Leave without saving?" });
  expect(document.getElementById(ask.getAttribute("aria-describedby") ?? "")?.textContent).toBe(
    "Your changes to this page are not saved."
  );
  holding = false;
  act(() => release?.());
});

test("signing out as a pasted file uploads asks nothing", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view } = await editing(server);
  view.dispatch({ selection: { anchor: 0 } });
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(server.sent).toContain("UPLOAD chart.png under Guide"));

  await user.click(screen.getByRole("button", { name: "Ada" }));
  await user.click(await screen.findByRole("menuitem", { name: "Sign out" }));

  expect(await screen.findByRole("heading", { name: "Sign in" })).toBeTruthy();
  expect(screen.queryByRole("alertdialog")).toBeNull();
});

test("a file without an extension uploads, and is not inserted: the editor says so, shown and unseen", async () => {
  const server = pageServer({ nodes });
  const { view, content } = await editing(server);
  const announce = vi.spyOn(EditorView.announce, "of");
  onTestFinished(() => announce.mockRestore());
  const before = view.state.doc.toString();

  paste(view, [new File(["text"], "LICENSE")]);

  const said = "LICENSE uploaded, not inserted: a name without an extension cannot be embedded.";
  // Shown by the editor, and announced in its own region.
  await waitFor(() =>
    expect(screen.getAllByText(said).filter((element) => element.closest(".cm-editor") === null)).toHaveLength(1)
  );
  expect(announce).toHaveBeenCalledWith(said);
  expect(view.state.doc.toString()).toBe(before);
  expect(content.closest(".cm-editor")).toBeTruthy();
});

test("Done as a file pasted uploads waits for its embed, the bar saying so; then it is saved with the rest, and the edit left", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view } = await editing(server);
  view.dispatch({ selection: { anchor: view.state.doc.length } });
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(server.sent).toContain("UPLOAD chart.png under Guide"));

  await user.click(screen.getByRole("button", { name: "Done" }));
  expect((await screen.findByText("Leaving once the uploads finish…")).tagName).toBe("OUTPUT");
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
  expect(puts(server)).toEqual([]);
  expect(screen.getByRole("button", { name: "Done" }).getAttribute("aria-busy")).toBe("true");
  expect(screen.getByRole("button", { name: "Done" }).getAttribute("aria-disabled")).toBe("true");
  act(() => server.release());
  expect(await screen.findByRole("button", { name: "Edit" })).toBeTruthy();
  expect(puts(server)).toEqual([expect.stringMatching(/^PUT Guide ".*!\[\[chart\.png\]\]" on 1 in session-1$/)]);
});

test("Done waits too for a composition begun as the uploads went: half a word is never saved", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content } = await editing(server);
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  await user.click(screen.getByRole("button", { name: "Done" }));
  await screen.findByText("Leaving once the uploads finish…");

  const composing = vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);
  onTestFinished(() => composing.mockRestore());
  view.dispatch({ changes: { from: view.state.doc.length, insert: "ni" } });
  // Cancelled, the upload settles as the composition goes on.
  await user.click(screen.getByRole("button", { name: "Cancel the upload of chart.png" }));
  await waitFor(() => expect(uploadsBy(content)).toBeUndefined());
  await act(() => new Promise((resolve) => void setTimeout(resolve, 100)));
  expect(puts(server)).toEqual([]);
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();

  composing.mockReturnValue(false);
  view.dispatch({ changes: { from: view.state.doc.length - 2, to: view.state.doc.length, insert: "你" } });
  act(() => void view.contentDOM.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true })));
  expect(await screen.findByRole("button", { name: "Edit" })).toBeTruthy();
  expect(puts(server)).toEqual([expect.stringMatching(/^PUT Guide "Guide.*你" on 1 in session-1$/)]);
});

test("an edit lost as Done waits for its uploads stays, its banner deciding: Back to reading goes at once", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content } = await editing(server);
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  await user.click(screen.getByRole("button", { name: "Done" }));
  await screen.findByText("Leaving once the uploads finish…");

  server.takeOver(guide.id);
  act(() => void document.dispatchEvent(new Event("visibilitychange")));
  const banner = await screen.findByRole("alert");
  await waitFor(() => expect(document.activeElement).toBe(banner));
  await user.click(within(banner).getByRole("button", { name: "Back to reading" }));
  expect(await screen.findByRole("button", { name: "Edit" })).toBeTruthy();
  expect(puts(server)).toEqual([]);
});

test("what the editor tells as Done waits for its uploads is said on the reading view, describing Edit", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content } = await editing(server);
  paste(view, [new File(["text"], "LICENSE")]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  await user.click(screen.getByRole("button", { name: "Done" }));
  await screen.findByText("Leaving once the uploads finish…");

  act(() => server.release());
  const edit = await screen.findByRole("button", { name: "Edit" });
  expect(document.getElementById(edit.getAttribute("aria-describedby") ?? "")?.textContent).toBe(
    "LICENSE uploaded, not inserted: a name without an extension cannot be embedded."
  );
});

test("Done with a conflict open waits for no upload: the focus goes to the conflict's heading at once", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content, type } = await editing(server);
  server.contents.set(guide.id, { content: "Guide\ntheirs\n", revision: 2 });
  type(" mine");
  await user.click(screen.getByRole("button", { name: "Save" }));
  const region = await screen.findByRole("region", { name: "This page changed while you edited it" });
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());

  await user.click(screen.getByRole("button", { name: "Done" }));
  await waitFor(() => expect(document.activeElement).toBe(within(region).getByRole("heading")));
  await act(async () => {});
  expect(screen.getByRole("button", { name: "Done" }).getAttribute("aria-busy")).toBeNull();
});

test("a conflict met as Done waits keeps the edit, the conflict's panel deciding: mine kept, the uploads in, it is not left", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content, type } = await editing(server);
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  await user.click(screen.getByRole("button", { name: "Done" }));
  await screen.findByText("Leaving once the uploads finish…");

  server.contents.set(guide.id, { content: "Guide\ntheirs\n", revision: 2 });
  type(" mine");
  await user.click(screen.getByRole("button", { name: "Save" }));
  const region = await screen.findByRole("region", { name: "This page changed while you edited it" });
  // The edit no longer waits to be left.
  await act(async () => {});
  expect(screen.getByRole("button", { name: "Done" }).getAttribute("aria-busy")).toBeNull();
  await user.click(within(region).getByRole("button", { name: "Keep mine" }));
  await waitFor(() =>
    expect(screen.queryByRole("region", { name: "This page changed while you edited it" })).toBeNull()
  );
  act(() => server.release());
  await waitFor(() => expect(view.state.doc.toString()).toContain("![[chart.png]]"));
  await act(() => new Promise((resolve) => void setTimeout(resolve, 100)));
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Edit" })).toBeNull();
});

test("a conflict the leave's save runs into once the uploads are in leaves the focus where the user went meanwhile", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content, type } = await editing(server);
  type(" one");
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  await user.click(screen.getByRole("button", { name: "Done" }));
  await screen.findByText("Leaving once the uploads finish…");

  server.contents.set(guide.id, { content: "Guide\ntheirs\n", revision: 2 });
  const heading = screen.getByRole("heading", { level: 1, name: "Guide" });
  act(() => heading.focus());
  act(() => server.release());
  await screen.findByRole("region", { name: "This page changed while you edited it" });
  await act(() => new Promise((resolve) => void setTimeout(resolve, 100)));
  expect(document.activeElement).toBe(heading);
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
});

test("a save the user asks for as Done waits that fails says so", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content, type } = await editing(server);
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  await user.click(screen.getByRole("button", { name: "Done" }));
  const status = await screen.findByText("Leaving once the uploads finish…");

  server.writesDown = true;
  type(" one");
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(status.className).toContain("text-destructive"));
  expect(status.textContent).not.toBe("Leaving once the uploads finish…");
  server.writesDown = false;
  act(() => server.release());
  expect(await screen.findByRole("button", { name: "Edit" })).toBeTruthy();
  expect(puts(server).at(-1)).toMatch(/one.*!\[\[chart\.png\]\]|!\[\[chart\.png\]\].*one/);
});

test("as Done waits, the bar says so over a save that failed; the leave that fails then leaves the focus where the user went", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content, type } = await editing(server);
  server.writesDown = true;
  type(" one");
  await user.click(screen.getByRole("button", { name: "Save" }));
  const status = (await screen.findAllByRole("status")).find((each) => each.tagName === "OUTPUT") as HTMLElement;
  await waitFor(() => expect(status.className).toContain("text-destructive"));
  const failed = status.textContent;

  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  await user.click(screen.getByRole("button", { name: "Done" }));
  await waitFor(() => expect(status.textContent).toBe("Leaving once the uploads finish…"));
  expect(status.className).not.toContain("text-destructive");
  const heading = screen.getByRole("heading", { level: 1, name: "Guide" });
  act(() => heading.focus());

  const tried = puts(server).length;
  act(() => server.release());
  await waitFor(() => expect(status.textContent).toBe(failed));
  expect(puts(server).length).toBeGreaterThan(tried);
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
  expect(document.activeElement).toBe(heading);
});

test("an edit whose upload goes is not idle: the idle exit leaves it once the embed is in and the time has gone again", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  onTestFinished(() => void vi.useRealTimers());
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { view } = await editing(server, userEvent.setup({ advanceTimers: vi.advanceTimersByTime }));
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(server.sent).toContain("UPLOAD chart.png under Guide"));

  // The body goes on slowly all the while: an upload that makes progress is not given up (M12/P2/S5, 16.3).
  for (let step = 1; step * 30_000 <= idleLimit + 60_000; step++) {
    act(() => transfers(server.app)[0]?.progress(step, 1_000));
    // oxlint-disable-next-line no-await-in-loop -- the time goes by step by step
    await rest(30_000);
  }
  expect(transfers(server.app)[0]?.aborted).toBe(false);
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
  expect(screen.queryByText("Leaving once the uploads finish…")).toBeNull();
  act(() => server.release());
  await waitFor(() => expect(view.state.doc.toString()).toContain("![[chart.png]]"));
  await rest(idleLimit);
  expect(await screen.findByText("Editing ended after 30 minutes without input.")).toBeTruthy();
  expect(puts(server).at(-1)).toMatch(/!\[\[chart\.png\]\]/);
});

test("a file pasted whose upload has no answer within three minutes inserts nothing; Done waiting for it leaves", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  onTestFinished(() => void vi.useRealTimers());
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view } = await editing(server, userEvent.setup({ advanceTimers: vi.advanceTimersByTime }));
  view.dispatch({ selection: { anchor: view.state.doc.length } });
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(server.sent).toContain("UPLOAD chart.png under Guide"));
  await user.click(screen.getByRole("button", { name: "Done" }));
  await screen.findByText("Leaving once the uploads finish…");

  act(() => transfers(server.app)[0]?.progress(3, 3));
  await rest(180_000);

  expect(await screen.findByRole("button", { name: "Edit" })).toBeTruthy();
  expect(transfers(server.app)[0]?.aborted).toBe(true);
  expect(puts(server).filter((line) => line.includes("chart.png"))).toEqual([]);
  act(() => server.release());
});

test("as the page is edited, the section's uploads show in it, the editor's by the editor; once it is read, one of the editor's that failed shows in the section", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content } = await editing(server);
  const section = await attachments();
  fireEvent.change(picker(section), { target: { files: [new File(["pdf"], "spec.pdf")] } });
  // A page's file is refused: its row says why until dismissed.
  paste(view, [new File(["# n"], "notes.md")]);

  await waitFor(() => expect(uploadsBy(content)?.textContent).toContain("notes.md"));
  expect(uploadsBy(content)?.textContent).not.toContain("spec.pdf");
  const own = within(section).getByRole("list", { name: "Uploads" });
  expect(own.textContent).toContain("spec.pdf");
  expect(own.textContent).not.toContain("notes.md");
  act(() => server.release());
  await waitFor(() => expect(within(section).queryByRole("list", { name: "Uploads" })).toBeNull());

  await user.click(screen.getByRole("button", { name: "Done" }));
  await screen.findByRole("button", { name: "Edit" });
  expect(within(await attachments()).getByRole("list", { name: "Uploads" }).textContent).toContain("notes.md");
});

test("an embed whose upload is answered as the input method composes waits for the composition's end", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { view } = await editing(server);
  view.dispatch({ selection: { anchor: 0 } });
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(server.sent).toContain("UPLOAD chart.png under Guide"));

  const composing = vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);
  onTestFinished(() => composing.mockRestore());
  act(() => server.release());
  await waitFor(() => expect(uploadsBy(view.contentDOM)).toBeUndefined());
  expect(view.state.doc.toString()).not.toContain("![[chart.png]]");
  composing.mockReturnValue(false);
  act(() => void view.contentDOM.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true })));
  await waitFor(() => expect(view.state.doc.toString()).toMatch(/^!\[\[chart\.png\]\]/));
});

test("a file pasted larger than the server takes is not sent: its row by the editor says why; Dismiss gives the focus back to the editor", async () => {
  const server = pageServer({
    nodes,
    answers: { "GET /api/v0/instance": () => json({ ...instanceJSON, asset_max_bytes: 4 }) },
  });
  const { user, view, content } = await editing(server);
  const before = view.state.doc.toString();
  await attachments();

  paste(view, [new File(["12345"], "chart.png", { type: "image/png" })]);
  const rows = await waitFor(() => {
    const found = uploadsBy(content);
    expect(found).toBeDefined();
    return found as HTMLElement;
  });
  expect(within(rows).getByText(/larger than this server takes/)).toBeTruthy();
  expect(server.sent.filter((line) => line.startsWith("UPLOAD"))).toEqual([]);
  expect(view.state.doc.toString()).toBe(before);
  await user.click(within(rows).getByRole("button", { name: /Dismiss/ }));
  await waitFor(() => expect(document.activeElement).toBe(content));
});

test("an upload by the editor cancelled from its row inserts nothing; the focus goes back to the editor", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content } = await editing(server);
  const before = view.state.doc.toString();
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());

  await user.click(screen.getByRole("button", { name: "Cancel the upload of chart.png" }));
  await waitFor(() => expect(uploadsBy(content)).toBeUndefined());
  expect(document.activeElement).toBe(content);
  act(() => server.release());
  await act(async () => {});
  expect(view.state.doc.toString()).toBe(before);
});

test("Done waits again for an upload begun as the composition it waited for ends", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content } = await editing(server);
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  await user.click(screen.getByRole("button", { name: "Done" }));
  await screen.findByText("Leaving once the uploads finish…");

  const composing = vi.spyOn(EditorView.prototype, "composing", "get").mockReturnValue(true);
  onTestFinished(() => composing.mockRestore());
  await user.click(screen.getByRole("button", { name: "Cancel the upload of chart.png" }));
  await waitFor(() => expect(uploadsBy(content)).toBeUndefined());
  // Pasted as the composition ends: its end waits a moment for its text.
  composing.mockReturnValue(false);
  act(() => void view.contentDOM.dispatchEvent(new CompositionEvent("compositionend", { bubbles: true })));
  paste(view, [new File(["png"], "plan.png", { type: "image/png" })]);
  await act(() => new Promise((resolve) => void setTimeout(resolve, 100)));
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
  expect(puts(server)).toEqual([]);

  act(() => server.release());
  expect(await screen.findByRole("button", { name: "Edit" })).toBeTruthy();
  expect(puts(server)).toEqual([expect.stringMatching(/^PUT Guide ".*!\[\[plan\.png\]\].*" on 1 in session-1$/)]);
});

test("what the editor told before Done is not said on the reading view", async () => {
  const server = pageServer({ nodes });
  const { user, view } = await editing(server);
  paste(view, [new File(["text"], "NOTICE")]);
  await screen.findAllByText("NOTICE uploaded, not inserted: a name without an extension cannot be embedded.");
  await user.click(screen.getByRole("button", { name: "Done" }));
  expect((await screen.findByRole("button", { name: "Edit" })).getAttribute("aria-describedby")).toBeNull();
});

test("what the editor told as a leave that failed waited is not said once a later Done leaves", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  server.writesDown = true;
  const { user, view, content } = await editing(server);
  paste(view, [new File(["text"], "LICENSE"), new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  await user.click(screen.getByRole("button", { name: "Done" }));
  await screen.findByText("Leaving once the uploads finish…");
  act(() => server.release());
  await waitFor(() => expect(puts(server)).toHaveLength(1));
  await waitFor(() => expect(screen.queryByText("Leaving once the uploads finish…")).toBeNull());
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();

  server.writesDown = false;
  await user.click(screen.getByRole("button", { name: "Done" }));
  expect((await screen.findByRole("button", { name: "Edit" })).getAttribute("aria-describedby")).toBeNull();
});

test("as Done waits, the bar says so over a save asked for meanwhile", async () => {
  const server = pageServer({
    nodes,
    answers: { "PUT /api/v0/pages/*/content": () => new Promise<Response>(() => undefined) },
  });
  server.uploadsHeld = true;
  const { user, view, content, type } = await editing(server);
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  await user.click(screen.getByRole("button", { name: "Done" }));
  const status = await screen.findByText("Leaving once the uploads finish…");
  type(" one");
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(status.textContent).toBe("Leaving once the uploads finish…");
});

test("an edit left once its uploads are in leaves the focus where the user went meanwhile", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content } = await editing(server);
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  await user.click(screen.getByRole("button", { name: "Done" }));
  await screen.findByText("Leaving once the uploads finish…");

  const heading = screen.getByRole("heading", { level: 1, name: "Guide" });
  act(() => heading.focus());
  act(() => server.release());
  expect(await screen.findByRole("button", { name: "Edit" })).toBeTruthy();
  expect(document.activeElement).toBe(heading);
});

test("what the editor told as a leave waited that it gave up on is not said once a later Done leaves", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content, type } = await editing(server);
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  await user.click(screen.getByRole("button", { name: "Done" }));
  await screen.findByText("Leaving once the uploads finish…");

  server.contents.set(guide.id, { content: "Guide\ntheirs\n", revision: 2 });
  type(" mine");
  await user.click(screen.getByRole("button", { name: "Save" }));
  const region = await screen.findByRole("region", { name: "This page changed while you edited it" });
  await user.click(within(region).getByRole("button", { name: "Discard mine" }));
  act(() => server.release());
  await screen.findAllByText("chart.png uploaded, not inserted: the text was replaced or can no longer be changed.");

  await user.click(screen.getByRole("button", { name: "Done" }));
  expect((await screen.findByRole("button", { name: "Edit" })).getAttribute("aria-describedby")).toBeNull();
});

test("a conflict the leave's save runs into, the focus fallen to the page meanwhile, takes it to the conflict's heading", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content } = await editing(server);
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  await user.click(screen.getByRole("button", { name: "Done" }));
  await screen.findByText("Leaving once the uploads finish…");

  server.contents.set(guide.id, { content: "Guide\ntheirs\n", revision: 2 });
  act(() => (document.activeElement as HTMLElement | null)?.blur());
  expect(document.activeElement).toBe(document.body);
  act(() => server.release());
  const region = await screen.findByRole("region", { name: "This page changed while you edited it" });
  await waitFor(() => expect(document.activeElement).toBe(within(region).getByRole("heading")));
});

test("an edit left once an upload cancelled from its row is gone gives the focus back to Edit: it fell with the edit", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content } = await editing(server);
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  await user.click(screen.getByRole("button", { name: "Done" }));
  await screen.findByText("Leaving once the uploads finish…");

  await user.click(screen.getByRole("button", { name: "Cancel the upload of chart.png" }));
  const edit = await screen.findByRole("button", { name: "Edit" });
  await waitFor(() => expect(document.activeElement).toBe(edit));
});

test("an edit left with Mod+E, then Done pressed as it waits for its uploads, gives the focus back to Edit", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content } = await editing(server);
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  content.focus();
  fireEvent.keyDown(content, { key: "e", ctrlKey: true });
  await screen.findByText("Leaving once the uploads finish…");

  await user.click(screen.getByRole("button", { name: "Done" }));
  act(() => server.release());
  const edit = await screen.findByRole("button", { name: "Edit" });
  await waitFor(() => expect(document.activeElement).toBe(edit));
});

test("a conflict the leave's save runs into, the focus moved within the edit meanwhile, takes it to the conflict's heading", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content } = await editing(server);
  paste(view, [
    new File(["png"], "chart.png", { type: "image/png" }),
    new File(["png"], "plan.png", { type: "image/png" }),
  ]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  await user.click(screen.getByRole("button", { name: "Done" }));
  await screen.findByText("Leaving once the uploads finish…");

  // An upload cancelled from its row, the focus going to the editor as the row goes: within the edit.
  await user.click(screen.getByRole("button", { name: "Cancel the upload of chart.png" }));
  await waitFor(() => expect(document.activeElement).toBe(content));
  server.contents.set(guide.id, { content: "Guide\ntheirs\n", revision: 2 });
  act(() => server.release());
  const region = await screen.findByRole("region", { name: "This page changed while you edited it" });
  await waitFor(() => expect(document.activeElement).toBe(within(region).getByRole("heading")));
});

test("a leave that fails once the uploads are in, Done pressed as it waited after Mod+E, gives the focus back to the editor", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { user, view, content } = await editing(server);
  paste(view, [new File(["png"], "chart.png", { type: "image/png" })]);
  await waitFor(() => expect(uploadsBy(content)).toBeDefined());
  content.focus();
  fireEvent.keyDown(content, { key: "e", ctrlKey: true });
  await screen.findByText("Leaving once the uploads finish…");

  await user.click(screen.getByRole("button", { name: "Done" }));
  server.writesDown = true;
  const tried = puts(server).length;
  act(() => server.release());
  await waitFor(() => expect(puts(server).length).toBeGreaterThan(tried));
  await waitFor(() => expect(document.activeElement).toBe(content));
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
});
