import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, onTestFinished, test, vi } from "vitest";

import type { Asset } from "../../services/asset.service";
import { archive, attachments, nodes, photo, rows } from "../../test/attachments";
import { json, notebookJSON, problem } from "../../test/fakes";
import { readAgain } from "../../test/page-panel";
import { assetJSON, assetNode, guide, linux, notes, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The attachments of a page and of a notebook's root, listed, and what their rows' menus do (M7/P4 design 3.5);
// their uploads are attachment-uploads.test.tsx's.

afterEach(() => {
  vi.useRealTimers();
});

const assetsPath = `GET /api/v0/notebooks/${notebookJSON.id}/assets`;

const actionsOf = (section: HTMLElement, name: string) =>
  within(section).findByRole("button", { name: `Actions for ${name}` });

test("a page's attachments are listed by name, each opening as it is served, with its size", async () => {
  const server = pageServer({ nodes });
  renderApp(pagePath(guide.id), server.app);

  const section = await attachments();
  await waitFor(() =>
    expect(rows(section)).toEqual([
      "data.zip",
      "manual.pdf (opens in a new tab)",
      "photo.png (opens in a new tab)",
      "README",
    ])
  );
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

test("a list that cannot be read says why, and is read on Try again; read again, it shows what changed elsewhere", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  let down = true;
  let listed: Asset[] = [];
  const server = pageServer({
    nodes,
    answers: {
      [assetsPath]: () => (down ? Promise.reject(new TypeError("offline")) : json({ data: listed, next_cursor: null })),
    },
  });
  renderApp(pagePath(guide.id), server.app);

  const section = await attachments();
  await within(section).findByText("Cannot reach the server. Check the connection and try again.");
  down = false;
  await user.click(within(section).getByRole("button", { name: "Try again" }));
  await within(section).findByText("Drop files here to upload them.");

  // Another tab's upload, shown as the list is read again.
  listed = [assetJSON(assetNode(76, "other.png", guide))];
  await readAgain();

  await waitFor(() => expect(rows(section)).toEqual(["other.png (opens in a new tab)"]));
});

test("a reader's menu opens, downloads and copies the embed; a writer's renames, moves and deletes too", async () => {
  const user = userEvent.setup();
  const reader = pageServer({ role: "reader", nodes });
  const { unmount } = renderApp(pagePath(guide.id), reader.app);
  let section = await attachments();
  await user.click(await actionsOf(section, "photo.png"));
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
  await user.click(await actionsOf(section, "photo.png"));
  expect(screen.getAllByRole("menuitem").map((item) => item.textContent)).toEqual([
    "Open (opens in a new tab)",
    "Download",
    "Copy embed",
    "Rename",
    "Move to…",
    "Delete",
  ]);
});

test("Copy embed copies the wikilink the server writes for it, its path where another attachment has its name, and says so", async () => {
  const user = userEvent.setup();
  renderApp(pagePath(guide.id), pageServer({ nodes: [...nodes, assetNode(77, "Photo.png", notes)] }).app);
  const section = await attachments();

  await user.click(await actionsOf(section, "photo.png"));
  await user.click(screen.getByRole("menuitem", { name: "Copy embed" }));

  await waitFor(() =>
    expect(section.querySelector("[aria-live=polite]")?.textContent).toBe("Embed copied: paste it into a page.")
  );
  expect(within(section).getAllByText("Embed copied: paste it into a page.")).toHaveLength(2);
  await expect(navigator.clipboard.readText()).resolves.toBe("![[Guide/photo.png]]");
});

test("where the page has no clipboard, Copy embed shows the embed in a field, focused, to copy by hand", async () => {
  const user = userEvent.setup();
  renderApp(pagePath(guide.id), pageServer({ nodes }).app);
  const section = await attachments();
  const writing = vi
    .spyOn(navigator.clipboard, "writeText")
    .mockRejectedValue(new DOMException("denied", "NotAllowedError"));
  onTestFinished(() => writing.mockRestore());

  await user.click(await actionsOf(section, "data.zip"));
  await user.click(screen.getByRole("menuitem", { name: "Copy embed" }));

  const field = await within(section).findByRole("textbox", { name: "Embed of data.zip" });
  expect((field as HTMLInputElement).value).toBe("![[data.zip]]");
  expect((field as HTMLInputElement).readOnly).toBe(true);
  await waitFor(() => expect(document.activeElement).toBe(field));
  expect(section.textContent).toContain("The embed could not be copied: select it here and copy it yourself.");
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

  await user.click(await actionsOf(section, "photo.png"));
  await user.click(screen.getByRole("menuitem", { name: "Rename" }));
  const dialog = await screen.findByRole("dialog", { name: "Rename photo.png" });
  const field = within(dialog).getByRole("textbox", { name: "Name" });
  expect((field as HTMLInputElement).value).toBe("photo");
  expect(dialog.textContent).toContain("The extension .png stays.");
  await user.clear(field);
  await user.type(field, "cover{Enter}");

  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(server.sent).toContain("PATCH photo.png to cover.png");
  await waitFor(() => expect(rows(section)).toContain("cover.png (opens in a new tab)"));
  expect(document.activeElement).toBe(within(section).getByRole("button", { name: "Actions for cover.png" }));
});

test("a rename refused stays in the dialog: an empty stem unsent, its extension said still; a name taken; pages being edited", async () => {
  const user = userEvent.setup();
  let refusal = problem(409, "page.title_taken");
  let patches = 0;
  const server = pageServer({
    nodes,
    answers: {
      "PATCH /api/v0/nodes/*": () => {
        patches += 1;
        return refusal;
      },
    },
  });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  await user.click(await actionsOf(section, "photo.png"));
  await user.click(screen.getByRole("menuitem", { name: "Rename" }));
  const dialog = await screen.findByRole("dialog", { name: "Rename photo.png" });
  const field = within(dialog).getByRole("textbox", { name: "Name" });

  await user.clear(field);
  await user.type(field, "{Enter}");
  expect(await within(dialog).findByText("Required.")).toBeTruthy();
  expect(field.getAttribute("aria-describedby")?.split(" ")).toHaveLength(2);
  expect(dialog.textContent).toContain("The extension .png stays.");
  expect(patches).toBe(0);

  await user.type(field, "cover{Enter}");
  await within(dialog).findByText(
    "A page under the same parent already has this title (titles differ in more than case)."
  );
  refusal = problem(409, "linking.pages_locked", {
    locks: [{ page_id: linux.id, user_id: "u-bob", display_name: "Bob" }],
  });
  await user.click(within(dialog).getByRole("button", { name: "Save" }));

  await waitFor(() => expect(dialog.textContent).toContain("Bob"));
  expect(dialog.textContent).toContain("Linux");
  expect(patches).toBe(2);
  expect(screen.getByRole("dialog", { name: "Rename photo.png" })).toBe(dialog);
});

test("Move to… offers every page and the top level; its own parent sends nothing; a refusal stays in the dialog", async () => {
  const user = userEvent.setup();
  const moves: string[] = [];
  const server = pageServer({
    nodes,
    answers: {
      "POST /api/v0/nodes/*/move": () => {
        moves.push("MOVE");
        return problem(409, "page.title_taken");
      },
    },
  });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();

  await user.click(await actionsOf(section, "photo.png"));
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
  expect(moves).toEqual([]);

  await user.click(within(section).getByRole("button", { name: "Actions for photo.png" }));
  await user.click(screen.getByRole("menuitem", { name: "Move to…" }));
  dialog = await screen.findByRole("dialog", { name: "Move photo.png" });
  await user.selectOptions(within(dialog).getByRole("combobox", { name: "Parent page" }), "Notes");
  await user.click(within(dialog).getByRole("button", { name: "Move" }));

  await within(dialog).findByText(
    "A page under the same parent already has this title (titles differ in more than case)."
  );
  expect(moves).toEqual(["MOVE"]);
  expect(screen.getByRole("dialog", { name: "Move photo.png" })).toBe(dialog);
});

test("a move sent takes it out of the list, the focus to the section's title", async () => {
  const user = userEvent.setup();
  const server = pageServer({ nodes });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();

  await user.click(await actionsOf(section, "photo.png"));
  await user.click(screen.getByRole("menuitem", { name: "Move to…" }));
  const dialog = await screen.findByRole("dialog", { name: "Move photo.png" });
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

  await user.click(await actionsOf(section, "data.zip"));
  await user.click(screen.getByRole("menuitem", { name: "Delete" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Delete data.zip?" });
  await user.click(within(dialog).getByRole("button", { name: "Delete" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(server.sent).toContain("DELETE data.zip");
  expect(rows(section)).not.toContain("data.zip");
  expect(document.activeElement).toBe(within(section).getByRole("heading", { name: "Attachments" }));
});

/** many is a hundred and two attachments under Guide, named so that they list in their order. */
const many = Array.from({ length: 102 }, (_, i) => assetNode(200 + i, `f${String(i).padStart(3, "0")}.bin`, guide));

test("More attachments reads the next hundred; the last read, the focus goes to the first it added", async () => {
  const user = userEvent.setup();
  const server = pageServer({ nodes: [guide, ...many] });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  await waitFor(() => expect(rows(section)).toHaveLength(100));

  await user.click(within(section).getByRole("button", { name: "More attachments" }));

  await waitFor(() => expect(rows(section)).toHaveLength(102));
  expect(within(section).queryByRole("button", { name: "More attachments" })).toBeNull();
  expect(document.activeElement).toBe(within(section).getByRole("link", { name: "f100.bin" }));
  expect(server.sent).toContain("GET assets Guide from 100");
});

test("More read as the reader does something else leaves the focus where they put it", async () => {
  const user = userEvent.setup();
  let release: (() => void) | undefined;
  const server = pageServer({
    nodes: [guide, ...many],
    answers: {
      [assetsPath]: async (request) => {
        const cursor = new URL(request.url).searchParams.get("cursor");
        if (cursor !== null) {
          await new Promise<void>((resolve) => (release = resolve));
        }
        const from = Number(cursor ?? "0");
        return json({
          data: many.slice(from, from + 100).map((node) => assetJSON(node)),
          next_cursor: from + 100 < many.length ? String(from + 100) : null,
        });
      },
    },
  });
  renderApp(pagePath(guide.id), server.app);
  const section = await attachments();
  await waitFor(() => expect(rows(section)).toHaveLength(100));

  await user.click(within(section).getByRole("button", { name: "More attachments" }));
  await waitFor(() => expect(release).toBeDefined());
  await user.click(screen.getByRole("heading", { level: 1, name: "Guide" }));
  const heading = screen.getByRole("heading", { level: 1, name: "Guide" });
  heading.focus();
  act(() => release?.());

  await waitFor(() => expect(rows(section)).toHaveLength(102));
  expect(document.activeElement).toBe(heading);
});

test("the list is read again a minute before the first of its addresses expires, half a minute after a read at the soonest", async () => {
  vi.useFakeTimers({ now: Date.parse("2026-10-09T08:00:00Z"), shouldAdvanceTime: true });
  const server = pageServer({ nodes });
  server.assetsExpireAt = "2026-10-09T09:00:00Z";
  renderApp(pagePath(guide.id), server.app);
  await attachments();
  const reads = () => server.sent.filter((line) => line === "GET assets Guide").length;
  await waitFor(() => expect(reads()).toBe(1));

  await act(() => vi.advanceTimersByTimeAsync(58 * 60_000));
  expect(reads()).toBe(1);
  // Expired already, by this clock: read again half a minute after each read, not at once.
  server.assetsExpireAt = "2026-10-09T07:00:00Z";
  await act(() => vi.advanceTimersByTimeAsync(60_000 + 5_000));
  expect(reads()).toBe(2);
  await act(() => vi.advanceTimersByTimeAsync(20_000));
  expect(reads()).toBe(2);
  await act(() => vi.advanceTimersByTimeAsync(15_000));
  expect(reads()).toBe(3);
});

test("addresses that expire far off, by this clock, are read again before an hour from the read is over", async () => {
  vi.useFakeTimers({ now: Date.parse("2026-10-09T08:00:00Z"), shouldAdvanceTime: true });
  const server = pageServer({ nodes });
  server.assetsExpireAt = "2026-10-09T12:00:00Z";
  renderApp(pagePath(guide.id), server.app);
  await attachments();
  const reads = () => server.sent.filter((line) => line === "GET assets Guide").length;
  await waitFor(() => expect(reads()).toBe(1));

  await act(() => vi.advanceTimersByTimeAsync(58 * 60_000));
  expect(reads()).toBe(1);
  await act(() => vi.advanceTimersByTimeAsync(2 * 60_000));

  expect(reads()).toBe(2);
});

test("a page's deletion counts the attachments of its subtree, which go with it", async () => {
  const user = userEvent.setup();
  const others = [assetNode(75, "x.png", linux), assetNode(78, "y.png", notes), assetNode(79, "z.png")];
  renderApp(pagePath(linux.id), pageServer({ nodes: [...nodes, ...others] }).app);
  const tree = await screen.findByRole("navigation", { name: "Pages of Plans" });

  await user.click(await within(tree).findByRole("button", { name: "Actions for Guide" }));
  await user.click(screen.getByRole("menuitem", { name: "Delete" }));
  let dialog = await screen.findByRole("alertdialog", { name: "Delete Guide?" });
  expect(dialog.textContent).toContain(
    "Everyone who sees this notebook loses the page. Its subpages go with it: 2. The attachments under it go with it: 5."
  );
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));

  await user.click(await within(tree).findByRole("button", { name: "Actions for Install" }));
  await user.click(screen.getByRole("menuitem", { name: "Delete" }));
  dialog = await screen.findByRole("alertdialog", { name: "Delete Install?" });
  expect(dialog.textContent).toContain("The attachments under it go with it: 1.");
});
