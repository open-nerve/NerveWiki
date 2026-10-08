import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { notebookJSON, problem } from "../../test/fakes";
import { assetNode, guide, install, linux, notes, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";
import { formOf } from "../../test/transfer";

// The attachments of a page and of a notebook's root (M7/P4 design 3.5, 3.6).

afterEach(() => {
  vi.useRealTimers();
});

const photo = assetNode(70, "photo.png", guide);
const manual = assetNode(71, "manual.pdf", guide);
const archive = assetNode(72, "data.zip", guide);
const readme = assetNode(73, "README", guide);
const nodes = [guide, install, linux, notes, photo, manual, archive, readme];

/** attachments waits for the section of the attachments shown. */
function attachments(): Promise<HTMLElement> {
  return screen.findByRole("region", { name: "Attachments" });
}

/** rows are the names of the attachments listed. */
function rows(section: HTMLElement): string[] {
  const list = within(section)
    .queryAllByRole("list")
    .find((each) => each.getAttribute("aria-label") === null);
  return list === undefined ? [] : [...list.querySelectorAll(":scope > li > a")].map((link) => link.textContent);
}

/** files is a drop's or a paste's transfer of files, and of folders, which only say what they are. */
function dropped(files: File[], folders: string[] = []) {
  return {
    types: ["Files"],
    files,
    items: [
      ...files.map((file) => ({
        kind: "file",
        getAsFile: () => file,
        webkitGetAsEntry: () => ({ isDirectory: false }),
      })),
      ...folders.map((name) => ({
        kind: "file",
        getAsFile: () => new File([], name),
        webkitGetAsEntry: () => ({ isDirectory: true }),
      })),
    ],
    dropEffect: "none",
  };
}

test("a page's attachments are listed by name, each opening as it is served, with its size", async () => {
  const server = pageServer({ nodes });
  renderApp(pagePath(guide.id), server.app);

  const section = await attachments();
  await waitFor(() => expect(rows(section)).toHaveLength(4));
  expect(rows(section)).toEqual([
    "photo.png (opens in a new tab)",
    "manual.pdf (opens in a new tab)",
    "data.zip",
    "README",
  ]);
  const opened = within(section).getByRole("link", { name: "photo.png (opens in a new tab)" });
  expect([opened.getAttribute("href"), opened.getAttribute("target"), opened.getAttribute("rel")]).toEqual([
    `/api/v0/assets/${photo.id}/content?sig=1`,
    "_blank",
    "noopener noreferrer",
  ]);
  const downloaded = within(section).getByRole("link", { name: "data.zip" });
  expect([
    downloaded.getAttribute("href"),
    downloaded.getAttribute("target"),
    downloaded.getAttribute("download"),
  ]).toEqual([`/api/v0/assets/${archive.id}/content?sig=1&download=1`, null, "data.zip"]);
  expect(section.textContent).toContain("1 KB");
  expect(server.sent).toContain(`GET assets Guide`);
});

test("the notebook's home lists the attachments at its root", async () => {
  const server = pageServer({ nodes: [...nodes, assetNode(74, "logo.png")] });
  renderApp(`/lab/notebooks/${notebookJSON.id}`, server.app);

  const section = await attachments();

  await waitFor(() => expect(rows(section)).toEqual(["logo.png (opens in a new tab)"]));
  expect(server.sent).toContain("GET assets root");
});

test("without attachments, a reader sees no section; a writer sees its title, Upload and where to drop files", async () => {
  const reader = pageServer({ role: "reader" });
  const { unmount } = renderApp(pagePath(guide.id), reader.app);
  await screen.findByRole("heading", { level: 1, name: "Guide" });
  await waitFor(() => expect(reader.sent).toContain("GET assets Guide"));
  expect(screen.queryByRole("region", { name: "Attachments" })).toBeNull();
  unmount();

  const writer = pageServer({ role: "editor" });
  renderApp(pagePath(guide.id), writer.app);
  const section = await attachments();
  expect(within(section).getByRole("button", { name: "Upload" })).toBeDefined();
  await within(section).findByText("Drop files here to upload them.");
});

test("a reader's menu opens, downloads and copies the embed; a writer's renames, moves and deletes too", async () => {
  const user = userEvent.setup();
  const reader = pageServer({ role: "reader", nodes });
  const { unmount } = renderApp(pagePath(guide.id), reader.app);
  let section = await attachments();
  await user.click(await within(section).findByRole("button", { name: "Actions for photo.png" }));
  expect(screen.getAllByRole("menuitem").map((item) => item.textContent)).toEqual([
    "Open (opens in a new tab)",
    "Download",
    "Copy embed",
  ]);
  await user.keyboard("{Escape}");
  // README has no extension: no link leads to it, and it has no embed.
  await user.click(within(section).getByRole("button", { name: "Actions for README" }));
  expect(screen.getAllByRole("menuitem").map((item) => item.textContent)).toEqual(["Open", "Download"]);
  await user.keyboard("{Escape}");
  expect(within(section).queryByRole("button", { name: "Upload" })).toBeNull();
  unmount();

  renderApp(pagePath(guide.id), pageServer({ nodes }).app);
  section = await attachments();
  await user.click(await within(section).findByRole("button", { name: "Actions for photo.png" }));
  expect(screen.getAllByRole("menuitem").map((item) => item.textContent)).toEqual([
    "Open (opens in a new tab)",
    "Download",
    "Copy embed",
    "Rename",
    "Move to…",
    "Delete",
  ]);
});

test("Copy embed copies the wikilink that embeds it, and says so", async () => {
  const user = userEvent.setup();
  renderApp(pagePath(guide.id), pageServer({ nodes }).app);
  const section = await attachments();

  await user.click(await within(section).findByRole("button", { name: "Actions for photo.png" }));
  await user.click(screen.getByRole("menuitem", { name: "Copy embed" }));

  await within(section).findByText("Embed copied: paste it into a page.");
  await expect(navigator.clipboard.readText()).resolves.toBe("![[photo.png]]");
});

test("a row drags as its embed, into the editor; one no link leads to does not drag", async () => {
  renderApp(pagePath(guide.id), pageServer({ nodes }).app);
  const section = await attachments();
  const row = (await within(section).findByRole("link", { name: "data.zip" })).closest("li") as HTMLElement;
  const data = new Map<string, string>();

  fireEvent.dragStart(row, { dataTransfer: { setData: (type: string, value: string) => data.set(type, value) } });

  expect(data.get("text/plain")).toBe("![[data.zip]]");
  expect(row.getAttribute("draggable")).toBe("true");
  expect(within(section).getByRole("link", { name: "README" }).closest("li")?.getAttribute("draggable")).toBeNull();
});

test("Rename renames the stem: the extension shows after the field and stays", async () => {
  const user = userEvent.setup();
  const server = pageServer({ nodes });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();

  await user.click(await within(section).findByRole("button", { name: "Actions for photo.png" }));
  await user.click(screen.getByRole("menuitem", { name: "Rename" }));
  const dialog = await screen.findByRole("dialog", { name: "Rename photo.png" });
  const field = within(dialog).getByRole("textbox", { name: "Name" });
  expect((field as HTMLInputElement).value).toBe("photo");
  expect(dialog.textContent).toContain("The extension .png stays.");
  await user.clear(field);
  await user.type(field, "cover{Enter}");

  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(server.sent).toContain("PATCH photo.png to cover.png");
  await waitFor(() => expect(rows(section)[0]).toBe("cover.png (opens in a new tab)"));
  expect(document.activeElement).toBe(within(section).getByRole("button", { name: "Actions for cover.png" }));
});

test("Move to… moves it last under another page, or to the top level; its own parent sends nothing", async () => {
  const user = userEvent.setup();
  const server = pageServer({ nodes });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();

  await user.click(await within(section).findByRole("button", { name: "Actions for photo.png" }));
  await user.click(screen.getByRole("menuitem", { name: "Move to…" }));
  let dialog = await screen.findByRole("dialog", { name: "Move photo.png" });
  const parent = within(dialog).getByRole("combobox", { name: "Parent page" });
  expect([...(parent as HTMLSelectElement).options].map((option) => option.textContent)).toEqual([
    "The notebook's top level",
    "Guide",
    "Guide / Install",
    "Guide / Install / Linux",
    "Notes",
  ]);
  expect((parent as HTMLSelectElement).value).toBe(guide.id);
  await user.click(within(dialog).getByRole("button", { name: "Move" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(server.sent.filter((line) => line.startsWith("MOVE"))).toEqual([]);

  await user.click(within(section).getByRole("button", { name: "Actions for photo.png" }));
  await user.click(screen.getByRole("menuitem", { name: "Move to…" }));
  dialog = await screen.findByRole("dialog", { name: "Move photo.png" });
  await user.selectOptions(within(dialog).getByRole("combobox", { name: "Parent page" }), "Notes");
  await user.click(within(dialog).getByRole("button", { name: "Move" }));

  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(server.sent).toContain("MOVE photo.png under Notes, after last");
  await waitFor(() => expect(rows(section)).not.toContain("photo.png (opens in a new tab)"));
  expect(document.activeElement).toBe(within(section).getByRole("heading", { name: "Attachments" }));
});

test("Delete asks first; once deleted, the focus goes to the section's title", async () => {
  const user = userEvent.setup();
  const server = pageServer({ nodes });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();

  await user.click(await within(section).findByRole("button", { name: "Actions for data.zip" }));
  await user.click(screen.getByRole("menuitem", { name: "Delete" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Delete data.zip?" });
  await user.click(within(dialog).getByRole("button", { name: "Delete" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(server.sent).toContain("DELETE data.zip");
  expect(rows(section)).not.toContain("data.zip");
  expect(document.activeElement).toBe(within(section).getByRole("heading", { name: "Attachments" }));
});

test("Upload sends each file chosen; each shows its progress and Cancel until its list has it", async () => {
  const user = userEvent.setup();
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  const picker = section.querySelector<HTMLInputElement>("input[type=file]");

  fireEvent.change(picker as HTMLInputElement, {
    target: { files: [new File(["abcd"], "photo.png"), new File(["ef"], "notes.txt")] },
  });

  const uploads = await within(section).findByRole("list", { name: "Uploads" });
  await waitFor(() =>
    expect(
      within(uploads)
        .getAllByRole("progressbar")
        .map((bar) => bar.getAttribute("aria-label"))
    ).toEqual(["Upload of photo 2.png", "Upload of notes.txt"])
  );
  expect(uploads.textContent).toContain("100%");
  expect(server.sent.filter((line) => line.startsWith("UPLOAD"))).toEqual([
    "UPLOAD photo 2.png under Guide",
    "UPLOAD notes.txt under Guide",
  ]);
  act(() => server.release());

  await waitFor(() => expect(within(section).queryByRole("list", { name: "Uploads" })).toBeNull());
  expect(rows(section)).toEqual(expect.arrayContaining(["photo 2.png (opens in a new tab)", "notes.txt"]));
  expect(await user.click(within(section).getByRole("button", { name: "Upload" }))).toBeUndefined();
});

test("Cancel stops an upload, and the focus goes back to Upload", async () => {
  const user = userEvent.setup();
  const server = pageServer({ nodes });
  server.uploadsHeld = true;
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();

  fireEvent.change(section.querySelector("input[type=file]") as HTMLInputElement, {
    target: { files: [new File(["abcd"], "big.png")] },
  });
  await user.click(await within(section).findByRole("button", { name: "Cancel the upload of big.png" }));

  expect(within(section).queryByRole("list", { name: "Uploads" })).toBeNull();
  expect(document.activeElement).toBe(within(section).getByRole("button", { name: "Upload" }));
});

test("a page's file, or a file larger than the server takes, is not sent: it says why until dismissed", async () => {
  const user = userEvent.setup();
  const server = pageServer({ nodes });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  const big = new File([], "huge.mov");
  Object.defineProperty(big, "size", { value: 60 * 1024 * 1024 });

  fireEvent.change(section.querySelector("input[type=file]") as HTMLInputElement, {
    target: { files: [new File(["# a"], "notes.md"), big] },
  });

  const uploads = await within(section).findByRole("list", { name: "Uploads" });
  await within(uploads).findByText("A Markdown file is a page: import it instead.");
  await within(uploads).findByText("The file is larger than this server takes: 50 MB at most.");
  expect(server.sent.filter((line) => line.startsWith("UPLOAD"))).toEqual([]);
  await user.click(within(uploads).getByRole("button", { name: "Dismiss the upload of notes.md" }));
  await user.click(within(uploads).getByRole("button", { name: "Dismiss the upload of huge.mov" }));

  expect(within(section).queryByRole("list", { name: "Uploads" })).toBeNull();
  expect(document.activeElement).toBe(within(section).getByRole("button", { name: "Upload" }));
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

  fireEvent.change(section.querySelector("input[type=file]") as HTMLInputElement, {
    target: { files: [new File(["a"], "x.png")] },
  });

  await waitFor(() => expect(rows(section)).toContain("x 4.png (opens in a new tab)"));
  expect(server.sent.filter((line) => line.startsWith("UPLOAD"))).toEqual([
    "UPLOAD x.png under Guide",
    "UPLOAD x 4.png under Guide",
  ]);
  cleanup();

  // Names the server compares otherwise than the page: each is taken.
  const names: string[] = [];
  const refusing = pageServer({
    nodes,
    answers: {
      [`POST /api/v0/notebooks/${notebookJSON.id}/assets`]: (request) => {
        names.push(String(formOf(request)?.get("name")));
        return problem(409, "page.title_taken");
      },
    },
  });
  renderApp(pagePath(guide.id), refusing.app);
  section = await attachments();
  fireEvent.change(section.querySelector("input[type=file]") as HTMLInputElement, {
    target: { files: [new File(["a"], "y.png")] },
  });

  await within(section).findByText("Another page or attachment here has this name, and the names tried after it too.");
  expect(names).toEqual(["y.png", "y 2.png", "y 3.png"]);
});

test("files dropped on the section, or on the page's reading view, upload to the page; a folder does not", async () => {
  const server = pageServer({ nodes });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();

  const over = fireEvent.dragOver(section, { dataTransfer: dropped([]) });
  expect(over).toBe(false);
  fireEvent.drop(section, { dataTransfer: dropped([new File(["a"], "a.png")], ["photos"]) });
  await within(section).findByText("Folders are not uploaded: import a folder of notes instead.");
  const view = (await screen.findByText("Guide", { selector: "p" })).closest("div[class]") as HTMLElement;
  fireEvent.drop(view, { dataTransfer: dropped([new File(["b"], "b.png")]) });

  await waitFor(() =>
    expect(server.sent.filter((line) => line.startsWith("UPLOAD"))).toEqual([
      "UPLOAD a.png under Guide",
      "UPLOAD b.png under Guide",
    ])
  );
});

test("a reader's section and reading view take no files", async () => {
  const server = pageServer({ nodes, role: "reader" });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();

  const transfer = dropped([]);
  fireEvent.dragOver(section, { dataTransfer: transfer });
  expect(transfer.dropEffect).toBe("none");
  fireEvent.drop(section, { dataTransfer: dropped([new File(["a"], "a.png")]) });

  expect(server.sent.filter((line) => line.startsWith("UPLOAD"))).toEqual([]);
});

test("a file dropped where nothing takes it stays out of the tab", async () => {
  renderApp(pagePath(guide.id), pageServer({ nodes }).app);
  await attachments();
  const transfer = dropped([]);

  expect(fireEvent.dragOver(document.body, { dataTransfer: transfer })).toBe(false);
  expect(transfer.dropEffect).toBe("none");
  expect(fireEvent.drop(document.body, { dataTransfer: transfer })).toBe(false);
  // Text dragged, a link, is the page's own: it goes as it would.
  expect(fireEvent.dragOver(document.body, { dataTransfer: { types: ["text/plain"] } })).toBe(true);
});

test("More attachments reads the next hundred; the last read, the focus goes to the first it added", async () => {
  const user = userEvent.setup();
  const server = pageServer({ nodes });
  server.assetPage = 2;
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  await waitFor(() => expect(rows(section)).toHaveLength(2));

  await user.click(within(section).getByRole("button", { name: "More attachments" }));

  await waitFor(() => expect(rows(section)).toHaveLength(4));
  expect(within(section).queryByRole("button", { name: "More attachments" })).toBeNull();
  expect(document.activeElement).toBe(within(section).getByRole("link", { name: "data.zip" }));
  expect(server.sent).toContain("GET assets Guide from 2");
});

test("the list is read again a minute before the first of its addresses expires", async () => {
  vi.useFakeTimers({ now: Date.parse("2026-10-09T08:00:00Z"), shouldAdvanceTime: true });
  const server = pageServer({ nodes });
  server.assetsExpireAt = "2026-10-09T09:00:00Z";
  renderApp(pagePath(guide.id), server.app);
  await attachments();
  await waitFor(() => expect(server.sent.filter((line) => line === "GET assets Guide")).toHaveLength(1));

  await act(() => vi.advanceTimersByTimeAsync(58 * 60_000));
  expect(server.sent.filter((line) => line === "GET assets Guide")).toHaveLength(1);
  server.assetsExpireAt = "2026-10-09T10:00:00Z";
  await act(() => vi.advanceTimersByTimeAsync(2 * 60_000));

  expect(server.sent.filter((line) => line === "GET assets Guide")).toHaveLength(2);
});

test("a page's deletion counts the attachments of its subtree, which go with it", async () => {
  const user = userEvent.setup();
  renderApp(pagePath(notes.id), pageServer({ nodes: [...nodes, assetNode(75, "x.png", linux)] }).app);
  const tree = await screen.findByRole("navigation", { name: "Pages of Plans" });

  await user.click(await within(tree).findByRole("button", { name: "Actions for Guide" }));
  await user.click(screen.getByRole("menuitem", { name: "Delete" }));

  const dialog = await screen.findByRole("alertdialog", { name: "Delete Guide?" });
  expect(dialog.textContent).toContain("Its subpages go with it: 2.");
  expect(dialog.textContent).toContain("The attachments under it go with it: 5.");
});
