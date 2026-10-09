import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, onTestFinished, test, vi } from "vitest";

import { attachments, dropped, nodes, picker, rows, transfers } from "../../test/attachments";
import { json, notebookJSON, problem } from "../../test/fakes";
import { assetJSON, assetNode, guide, notes, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";
import { AssetStore } from "../../stores/asset.store";
import { formOf } from "../../test/transfer";

// The uploads of a page's attachments' section, chosen or dropped, and the drops the page keeps out of the tab
// (M7/P4 design 3.3–3.6).

const uploadsPath = `POST /api/v0/notebooks/${notebookJSON.id}/assets`;

/** uploaded is what the server was sent to upload. */
const uploaded = (sent: readonly string[]) => sent.filter((line) => line.startsWith("UPLOAD"));

/** notice is what section says, unseen. */
const notice = (section: HTMLElement) => section.querySelector("[aria-live=polite]")?.textContent;

/** choose chooses files in section's file input, as its picker would. */
function choose(section: HTMLElement, files: File[]): void {
  fireEvent.change(picker(section), { target: { files } });
}

test("Upload opens the picker of files, several at once", async () => {
  const user = userEvent.setup();
  renderApp(pagePath(guide.id), pageServer({ nodes }).app);
  const section = await attachments();
  const opened: HTMLInputElement[] = [];
  const click = vi.spyOn(HTMLInputElement.prototype, "click").mockImplementation(function (this: HTMLInputElement) {
    opened.push(this);
  });
  onTestFinished(() => click.mockRestore());

  await user.click(within(section).getByRole("button", { name: "Upload" }));

  expect(opened).toEqual([picker(section)]);
  expect([picker(section).type, picker(section).multiple]).toEqual(["file", true]);
});

test("each file chosen goes up with its progress, said as it begins and as it is uploaded, until its list has it", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  const said = () => notice(section);

  choose(section, [new File(["abcd"], "photo.png"), new File(["ef"], "notes.txt")]);

  const uploads = await within(section).findByRole("list", { name: "Uploads" });
  await waitFor(() =>
    expect(
      within(uploads)
        .getAllByRole("progressbar")
        .map((bar) => bar.getAttribute("aria-label"))
    ).toEqual(["Upload of photo 2.png", "Upload of notes.txt"])
  );
  expect(uploads.textContent).toContain("0%");
  // Said in the turn they begin in.
  await waitFor(() => expect(said()).toBe("Uploading files: 2."));
  expect(uploaded(server.sent)).toEqual(["UPLOAD photo 2.png under Guide", "UPLOAD notes.txt under Guide"]);
  act(() => server.release());

  await waitFor(() => expect(within(section).queryByRole("list", { name: "Uploads" })).toBeNull());
  expect(rows(section)).toEqual(expect.arrayContaining(["photo 2.png (opens in a new tab)", "notes.txt"]));
  await waitFor(() =>
    expect(said()).toMatch(
      /^Uploaded: (photo 2\.png and notes\.txt|notes\.txt and photo 2\.png|notes\.txt|photo 2\.png)\.$/
    )
  );
});

test.each<[string, "en" | "zh-CN", RegExp, RegExp]>([
  ["English", "en", /^Uploaded: (.+)\.$/, /, and | and |, /],
  ["Chinese", "zh-CN", /^已上传：(.+)。$/, /、|和/],
])(
  "uploads that leave together are said in one sentence, their names joined as the language does (%s)",
  async (_, locale, sentence, joins) => {
    const server = pageServer({ nodes });
    server.uploadsHeld = true;
    renderApp(pagePath(guide.id), server.app);
    const section = await attachments();
    act(() => server.app.preferences.setLocale(locale));
    const region = section.querySelector("[aria-live=polite]") as HTMLElement;
    const said: string[] = [];
    const watching = new MutationObserver(() => said.push((region.textContent ?? "").trim()));
    watching.observe(region, { characterData: true, childList: true, subtree: true });
    onTestFinished(() => watching.disconnect());

    choose(section, [new File(["a"], "a.png"), new File(["b"], "b.png"), new File(["c"], "c.png")]);
    const uploads = locale === "en" ? "Uploads" : "上传队列";
    await within(section).findByRole("list", { name: uploads });
    act(() => server.release());
    await waitFor(() => expect(within(section).queryByRole("list", { name: uploads })).toBeNull());
    await act(() => new Promise((resolve) => setTimeout(resolve, 20)));

    const lists = said.flatMap((text) => sentence.exec(text)?.[1] ?? []);
    expect(lists.flatMap((names) => names.split(joins)).toSorted()).toEqual(["a.png", "b.png", "c.png"]);
    expect(lists.some((names) => joins.test(names))).toBe(true);
  }
);

test("uploads that leave in turns apart are said apart, each naming its own", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();

  for (const name of ["a.png", "b.png"]) {
    choose(section, [new File(["x"], name)]);
    // oxlint-disable-next-line no-await-in-loop -- one upload after the other
    await within(section).findByRole("list", { name: "Uploads" });
    act(() => server.release());
    // oxlint-disable-next-line no-await-in-loop -- one upload after the other
    await waitFor(() => expect(notice(section)).toBe(`Uploaded: ${name}.`));
  }
});

test("an upload that leaves as others begin, in one turn, is said with them", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  // As a.png leaves, b.png and c.png are chosen: in its turn.
  const dismiss = AssetStore.prototype.dismiss;
  let section: HTMLElement | undefined;
  const leaving = vi.spyOn(AssetStore.prototype, "dismiss").mockImplementation(function (this: AssetStore, upload) {
    dismiss.call(this, upload);
    if (upload.name === "a.png" && section !== undefined) {
      choose(section, [new File(["b"], "b.png"), new File(["c"], "c.png")]);
    }
  });
  onTestFinished(() => leaving.mockRestore());
  renderApp(pagePath(guide.id), server.app);
  section = await attachments();

  choose(section, [new File(["a"], "a.png")]);
  await within(section).findByRole("list", { name: "Uploads" });
  act(() => server.release());

  await waitFor(() => expect(notice(section)).toBe("Uploaded: a.png. Uploading files: 2."));
  act(() => server.release());
});

test("uploads that begin as one leaves, in one turn, are said with it, the one that began before too", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();

  choose(section, [new File(["a"], "a.png")]);
  await waitFor(() => expect(server.held).toHaveLength(1));
  await waitFor(() => expect(notice(section)).toBe("Uploading files: 1."));
  act(() => {
    server.release();
    choose(section, [new File(["b"], "b.png"), new File(["c"], "c.png")]);
  });

  await waitFor(() => expect(notice(section)).toBe("Uploaded: a.png. Uploading files: 2."));
  act(() => server.release());
});

test("Cancel stops the upload: its request is aborted, nothing is made, nothing said uploaded, the focus goes back to Upload", async () => {
  const user = userEvent.setup();
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();

  choose(section, [new File(["abcd"], "big.png")]);
  await user.click(await within(section).findByRole("button", { name: "Cancel the upload of big.png" }));

  await waitFor(() => expect(within(section).queryByRole("list", { name: "Uploads" })).toBeNull());
  expect(transfers(server.app).map((transfer) => transfer.aborted)).toEqual([true]);
  expect(document.activeElement).toBe(within(section).getByRole("button", { name: "Upload" }));
  act(() => server.release());
  await act(() => new Promise((resolve) => setTimeout(resolve, 20)));
  expect(server.nodes.map((node) => node.name)).not.toContain("big.png");
  expect(notice(section)).toBe("Uploading files: 1.");
});

test("an upload answered is finishing as its list is read again: it can no longer be cancelled", async () => {
  const user = userEvent.setup();
  let holding = false;
  let release: (() => void) | undefined;
  const server = pageServer({
    nodes,
    answers: {
      [`GET /api/v0/notebooks/${notebookJSON.id}/assets`]: async () => {
        if (holding) {
          await new Promise<void>((resolve) => (release = resolve));
        }
        return json({
          data: server.nodes
            .filter((node) => node.kind === "asset" && node.parent_id === guide.id)
            .map((node) => assetJSON(node)),
          next_cursor: null,
        });
      },
    },
  });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  await waitFor(() => expect(rows(section)).toHaveLength(4));
  holding = true;

  choose(section, [new File(["a"], "a.png")]);

  const finishing = await within(section).findByRole("button", { name: "Finishing the upload of a.png" });
  expect(finishing.getAttribute("aria-disabled")).toBe("true");
  await user.click(finishing);
  expect(transfers(server.app).map((transfer) => transfer.aborted)).toEqual([false]);
  act(() => release?.());
  await waitFor(() => expect(within(section).queryByRole("list", { name: "Uploads" })).toBeNull());
  expect(rows(section)).toContain("a.png (opens in a new tab)");
});

test("an upload whose bytes have all gone is finishing as the server answers: it can no longer be cancelled", async () => {
  const user = userEvent.setup();
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();

  choose(section, [new File(["abcd"], "big.png")]);
  await within(section).findByRole("button", { name: "Cancel the upload of big.png" });
  act(() => transfers(server.app)[0]?.progress(4, 4));

  const finishing = await within(section).findByRole("button", { name: "Finishing the upload of big.png" });
  expect(finishing.getAttribute("aria-disabled")).toBe("true");
  expect(within(section).getByText("100%")).toBeTruthy();
  await user.click(finishing);
  expect(transfers(server.app).map((transfer) => transfer.aborted)).toEqual([false]);
  act(() => server.release());
  await waitFor(() => expect(within(section).queryByRole("list", { name: "Uploads" })).toBeNull());
  expect(rows(section)).toContain("big.png (opens in a new tab)");
});

test("the focus on a row's button stays as its upload fails: Cancel becomes Dismiss", async () => {
  const user = userEvent.setup();
  let refuse: (() => void) | undefined;
  const server = pageServer({
    nodes,
    answers: {
      [uploadsPath]: () => new Promise<Response>((resolve) => (refuse = () => resolve(problem(507, "storage_full")))),
    },
  });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();

  choose(section, [new File(["a"], "a.png")]);
  const cancel = await within(section).findByRole("button", { name: "Cancel the upload of a.png" });
  cancel.focus();
  act(() => refuse?.());

  const dismiss = await within(section).findByRole("button", { name: "Dismiss the upload of a.png" });
  expect(dismiss).toBe(cancel);
  expect(document.activeElement).toBe(dismiss);
  expect(within(section).getByRole("alert").textContent).toBe(
    "The server has no room for more files. Ask its administrator."
  );
  await user.click(dismiss);
  expect(document.activeElement).toBe(within(section).getByRole("button", { name: "Upload" }));
});

test("the focus on a row's button as its upload goes through goes back to Upload", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();

  choose(section, [new File(["a"], "a.png")]);
  (await within(section).findByRole("button", { name: "Cancel the upload of a.png" })).focus();
  act(() => server.release());

  await waitFor(() => expect(within(section).queryByRole("list", { name: "Uploads" })).toBeNull());
  expect(document.activeElement).toBe(within(section).getByRole("button", { name: "Upload" }));
});

test("a page's file, or a file larger than the server takes, is not sent: it says why until dismissed", async () => {
  const user = userEvent.setup();
  const server = pageServer({ nodes });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  const big = new File([], "huge.mov");
  Object.defineProperty(big, "size", { value: 60 * 1024 * 1024 });

  choose(section, [new File(["# a"], "notes.md"), big]);

  const uploads = await within(section).findByRole("list", { name: "Uploads" });
  await act(() => new Promise((resolve) => setTimeout(resolve, 20)));
  // Nothing goes: nothing is said to.
  expect(notice(section)).toBe("");
  await within(uploads).findByText("A Markdown file is a page: import it instead.");
  await within(uploads).findByText("The file is larger than this server takes: 50 MB at most.");
  expect(uploaded(server.sent)).toEqual([]);
  await user.click(within(uploads).getByRole("button", { name: "Dismiss the upload of notes.md" }));
  await user.click(within(uploads).getByRole("button", { name: "Dismiss the upload of huge.mov" }));

  expect(within(section).queryByRole("list", { name: "Uploads" })).toBeNull();
  expect(document.activeElement).toBe(within(section).getByRole("button", { name: "Upload" }));
});

test("a file the server finds too large says how large a file it takes", async () => {
  const server = pageServer({ nodes, answers: { [uploadsPath]: () => problem(413, "payload_too_large") } });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();

  choose(section, [new File(["a"], "a.png")]);

  await within(section).findByText("The file is larger than this server takes: 50 MB at most.");
});

test("a name taken meanwhile tries the next free one, of the tree read again; refused three times, it says so", async () => {
  const server = pageServer({ nodes });
  renderApp(pagePath(guide.id), server.app);
  let section = await attachments();
  // Other tabs took the names since the tree was read.
  server.nodes = [
    ...server.nodes,
    assetNode(81, "x.png", guide),
    assetNode(82, "x 2.png", guide),
    assetNode(83, "x 3.png", guide),
  ];

  choose(section, [new File(["a"], "x.png")]);

  await waitFor(() => expect(rows(section)).toContain("x 4.png (opens in a new tab)"));
  expect(uploaded(server.sent)).toEqual(["UPLOAD x.png under Guide", "UPLOAD x 4.png under Guide"]);
  cleanup();

  // Names the server compares otherwise than the page: each is taken.
  const names: string[] = [];
  const refusing = pageServer({
    nodes,
    answers: {
      [uploadsPath]: (request) => {
        names.push(String(formOf(request)?.get("name")));
        return problem(409, "page.title_taken");
      },
    },
  });
  renderApp(pagePath(guide.id), refusing.app);
  section = await attachments();
  choose(section, [new File(["a"], "y.png")]);

  await within(section).findByText("Another page or attachment here has this name, and the names tried after it too.");
  expect(names).toEqual(["y.png", "y 2.png", "y 3.png"]);
});

test("a section shows the uploads under its own page, not another's", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  const { router } = renderApp(pagePath(guide.id), server.app);
  let section = await attachments();
  choose(section, [new File(["a"], "a.png")]);
  await within(section).findByRole("list", { name: "Uploads" });

  await act(() => router.navigate(pagePath(notes.id)));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  section = await attachments();

  expect(within(section).queryByRole("list", { name: "Uploads" })).toBeNull();
  act(() => server.release());
  await waitFor(() => expect(server.nodes.some((node) => node.name === "a.png")).toBe(true));
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  // Guide's upload is not Notes' to say.
  expect(notice(section)).toBe("");
});

test("a drag of files over the section, or the reading view, may drop there; dropped, they upload to the page, said; a folder does not", async () => {
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  const view = await screen.findByRole("article", { name: "Guide" });

  for (const zone of [section, view]) {
    const over = dropped([]);
    expect(fireEvent.dragOver(zone, { dataTransfer: over })).toBe(false);
    expect(over.dropEffect).toBe("copy");
  }
  fireEvent.drop(section, { dataTransfer: dropped([new File(["a"], "a.png")], ["photos"]) });
  await within(section).findByText("Folders are not uploaded: import a folder of notes instead.");
  await waitFor(() => expect(notice(section)).toBe("Uploading files: 1."));
  fireEvent.drop(view, { dataTransfer: dropped([new File(["b"], "b.png"), new File(["c"], "c.png")]) });

  await waitFor(() =>
    expect(uploaded(server.sent)).toEqual([
      "UPLOAD a.png under Guide",
      "UPLOAD b.png under Guide",
      "UPLOAD c.png under Guide",
    ])
  );
  // The section says what the reading view took.
  await waitFor(() => expect(notice(section)).toBe("Uploading files: 2."));
  act(() => server.release());
});

test("a file dropped on the reading view larger than the server takes is not sent", async () => {
  const server = pageServer({ nodes });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  const big = new File([], "huge.mov");
  Object.defineProperty(big, "size", { value: 60 * 1024 * 1024 });

  fireEvent.drop(await screen.findByRole("article", { name: "Guide" }), { dataTransfer: dropped([big]) });

  await within(section).findByText("The file is larger than this server takes: 50 MB at most.");
  expect(uploaded(server.sent)).toEqual([]);
});

test("a reader's section and reading view take no files", async () => {
  const server = pageServer({ nodes, role: "reader" });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  const view = await screen.findByRole("article", { name: "Guide" });

  for (const zone of [section, view]) {
    const over = dropped([]);
    fireEvent.dragOver(zone, { dataTransfer: over });
    expect(over.dropEffect).toBe("none");
    fireEvent.drop(zone, { dataTransfer: dropped([new File(["a"], "a.png")]) });
  }
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));

  expect(within(section).queryByRole("list", { name: "Uploads" })).toBeNull();
  expect(uploaded(server.sent)).toEqual([]);
});

test("files dropped on a dialog the section holds are not uploaded: a dialog is no place to drop", async () => {
  const user = userEvent.setup();
  const server = pageServer({ nodes });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  await user.click(await within(section).findByRole("button", { name: "Actions for data.zip" }));
  await user.click(screen.getByRole("menuitem", { name: "Delete" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Delete data.zip?" });

  const over = dropped([]);
  fireEvent.dragOver(dialog, { dataTransfer: over });
  fireEvent.drop(dialog, { dataTransfer: dropped([new File(["a"], "a.png")]) });
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));

  expect(over.dropEffect).toBe("none");
  expect(uploaded(server.sent)).toEqual([]);
});

test("a drag that started in the page, an image of it, is no file to upload; its end, a drop, a press or the pointer moved with no button down ends it", async () => {
  const server = pageServer({ nodes });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  const view = await screen.findByRole("article", { name: "Guide" });
  // The tree's drag and drop says jsdom's drags are none of a browser's.
  vi.mocked(console.warn).mockImplementation(() => undefined);

  // The image's drag carries its file, as Chromium's does: not the section's; the page keeps it out of the tab.
  const drag = dropped([new File(["a"], "image.png")]);
  fireEvent.dragStart(view, { dataTransfer: drag });
  fireEvent.dragOver(section, { dataTransfer: drag });
  expect(drag.dropEffect).toBe("none");
  expect(fireEvent.drop(section, { dataTransfer: drag })).toBe(false);
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  expect(uploaded(server.sent)).toEqual([]);

  // Another, cancelled as its source left the document: neither its end nor a drop reaches the document. The
  // pointer moved with its button down (Firefox, as a drag begins) is the drag still; with none, it is over.
  fireEvent.dragStart(view, { dataTransfer: dropped([]) });
  fireEvent.pointerMove(document.body, { buttons: 1 });
  const still = dropped([]);
  fireEvent.dragOver(section, { dataTransfer: still });
  expect(still.dropEffect).toBe("none");
  fireEvent.pointerMove(document.body, { buttons: 0 });
  const outside = dropped([]);
  expect(fireEvent.dragOver(section, { dataTransfer: outside })).toBe(false);
  expect(outside.dropEffect).toBe("copy");

  // Its end, a press, or a drop anywhere end it as well.
  for (const end of [
    () => fireEvent.dragEnd(view),
    () => fireEvent.pointerDown(document.body),
    () => fireEvent.drop(document.body, { dataTransfer: dropped([]) }),
  ]) {
    fireEvent.dragStart(view, { dataTransfer: dropped([]) });
    end();
    const next = dropped([]);
    fireEvent.dragOver(section, { dataTransfer: next });
    expect(next.dropEffect).toBe("copy");
  }
});

test("a file dropped where nothing takes it stays out of the tab; in an editor, the editor takes it", async () => {
  renderApp(pagePath(guide.id), pageServer({ nodes }).app);
  await attachments();
  const transfer = dropped([]);

  expect(fireEvent.dragOver(document.body, { dataTransfer: transfer })).toBe(false);
  expect(transfer.dropEffect).toBe("none");
  expect(fireEvent.drop(document.body, { dataTransfer: transfer })).toBe(false);
  // Text dragged, a link, is the page's own: it goes as it would.
  expect(fireEvent.dragOver(document.body, { dataTransfer: { types: ["text/plain"] } })).toBe(true);

  const editor = document.createElement("div");
  editor.setAttribute("contenteditable", "true");
  document.body.append(editor);
  const into = dropped([]);
  expect(fireEvent.dragOver(editor, { dataTransfer: into })).toBe(true);
  expect(into.dropEffect).toBe("move");
  expect(fireEvent.drop(editor, { dataTransfer: into })).toBe(true);
  // One over its text, a node in it, as some browsers have it.
  editor.textContent = "words";
  expect(fireEvent.dragOver(editor.firstChild as Text, { dataTransfer: dropped([]) })).toBe(true);
  editor.remove();
});
