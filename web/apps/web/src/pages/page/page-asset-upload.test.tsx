import { EditorView } from "@codemirror/view";
import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, onTestFinished, test, vi } from "vitest";

import { idleLimit } from "../../editor/idle-exit";
import { editorExtensions } from "../../editor/registry";
import { attachments, dropped, nodes, picker } from "../../test/attachments";
import { instanceJSON, json } from "../../test/fakes";
import { pageEditor } from "../../test/page-editor";
import { guide, pagePath, pageServer } from "../../test/page-server";
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
  expect(screen.queryByText("Leaving once the uploads finish…")).toBeNull();
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

  await rest(idleLimit + 60_000);
  expect(screen.getByRole("textbox", { name: "Page content" })).toBeTruthy();
  expect(screen.queryByText("Leaving once the uploads finish…")).toBeNull();
  act(() => server.release());
  await waitFor(() => expect(view.state.doc.toString()).toContain("![[chart.png]]"));
  await rest(idleLimit);
  expect(await screen.findByText("Editing ended after 30 minutes without input.")).toBeTruthy();
  expect(puts(server).at(-1)).toMatch(/!\[\[chart\.png\]\]/);
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
