import type { EditorView } from "@codemirror/view";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { editorExtensions } from "../../editor/registry";
import { attachments, dropped, nodes } from "../../test/attachments";
import { pageEditor } from "../../test/page-editor";
import { guide, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// Files pasted into the page's editor upload as its attachments, their embeds inserted, through the composition
// root's registry (M7/P4 design 5; the M4 handoff of attachments' extensions, items 2 and 3): the app's
// editorExtensions, loaded as the editor opens.

/** Guide edited by Ada, with the app's extensions, over server. */
async function editing(server: ReturnType<typeof pageServer>) {
  const user = userEvent.setup();
  renderApp(pagePath(guide.id), server.app, { editorExtensions });
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  return pageEditor();
}

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
  const before = view.state.doc.toString();

  paste(view, [new File(["text"], "LICENSE")]);

  const said = "LICENSE is in the page's attachments, not inserted: a name without an extension cannot be embedded.";
  // Shown by the editor, and announced in its own region.
  await waitFor(() =>
    expect(screen.getAllByText(said).filter((element) => element.closest(".cm-editor") === null)).toHaveLength(1)
  );
  expect(content.closest(".cm-editor")?.querySelector(".cm-announced")?.textContent).toBe(said);
  expect(view.state.doc.toString()).toBe(before);
});
