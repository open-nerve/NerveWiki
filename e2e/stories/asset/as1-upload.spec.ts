import { randomBytes } from "node:crypto";
import { readFileSync } from "node:fs";

import type { Asset } from "@nervewiki/api-client";
import type { Locator, Page } from "@playwright/test";

import { expectAssetsDeletedWithNodes, expectUploaded, storedBlobs } from "../../fixtures/assert/asset";
import { download, getAsset, listAssets, pngBytes, uploadAsset, utf8 } from "../../fixtures/assets";
import { countAnswers } from "../../fixtures/browser";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// AS1, attachments uploaded, listed, read and downloaded (M7 design 9; M7/P2
// design 3.13): each file streamed to the store, its row beside its node,
// and the same bytes at the addresses the server signed, without a token.
// The page version (M7/P4 design 3.5, 3.6): the attachments' section of a
// page and of the notebook's home, files chosen or dropped on the reading
// view, each going with its progress until its list has it.

test("AS1 (API): files uploaded under a page and at the root are listed, read and downloaded at their signed addresses, the same bytes", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, adminId, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const guide = await createPage(api, pat, notebook.id, "Guide");
  const notesBytes = utf8("Notes beside the guide.\n");
  const picture = await uploadAsset(
    api,
    pat,
    notebook.id,
    { name: "diagram.png", bytes: pngBytes, type: "image/png" },
    guide.id
  );
  const notes = await uploadAsset(api, pat, notebook.id, { name: "notes.txt", bytes: notesBytes, type: "text/plain" });

  expect([picture.parent_id, picture.mime, picture.width, picture.height]).toEqual([guide.id, "image/png", 2, 3]);
  expect([notes.parent_id, notes.mime, notes.width, notes.height]).toEqual([
    null,
    "application/octet-stream",
    null,
    null,
  ]);
  await expectUploaded(db, nervewiki.storageDir, picture, pngBytes, adminId, "api");
  await expectUploaded(db, nervewiki.storageDir, notes, notesBytes, adminId, "api");

  // The addresses are signed anew by each read: the attachment is the same.
  const unsigned = ({ content_url, download_url, expires_at, ...rest }: typeof picture) => rest;
  expect((await listAssets(api, pat, notebook.id, guide.id)).map(unsigned)).toEqual([unsigned(picture)]);
  expect((await listAssets(api, pat, notebook.id)).map(unsigned)).toEqual([unsigned(notes)]);
  const read = await getAsset(api, pat, picture.id);
  expect(read.response.status).toBe(200);
  expect(read.data && unsigned(read.data)).toEqual(unsigned(picture));

  for (const [asset, bytes] of [
    [picture, pngBytes],
    [notes, notesBytes],
  ] as const) {
    for (const address of [asset.content_url, asset.download_url]) {
      // oxlint-disable-next-line no-await-in-loop -- one download at a time, each checked
      const answer = await download(nervewiki.baseURL, address);
      expect(answer.status, `${asset.name} at ${address}`).toBe(200);
      // oxlint-disable-next-line no-await-in-loop -- its body
      expect(Buffer.compare(Buffer.from(await answer.arrayBuffer()), Buffer.from(bytes))).toBe(0);
    }
  }
});

/** attachments is the section of the attachments shown. */
const attachments = (page: Page) => page.getByRole("region", { name: "Attachments" });

/** uploadsPath is the path an upload into the notebook notebookId goes to. */
const uploadsPath = (notebookId: string) => `/api/v0/notebooks/${notebookId}/assets`;

/**
 * dragFileOver drags a file over target, then drops it there, as a script can: whether the dragover's default was
 * prevented and the drop effect it set, and whether the drop's default was prevented.
 */
function dragFileOver(target: Locator) {
  return target.evaluate((element) => {
    const data = new DataTransfer();
    data.items.add(new File(["x"], "notes.txt", { type: "text/plain" }));
    // A transfer made by script is no drag's: the browser keeps no drop effect set on it. What is set is kept here.
    let effect = "copy";
    Object.defineProperty(data, "dropEffect", {
      get: () => effect,
      set: (value: string) => {
        effect = value;
      },
    });
    const over = new DragEvent("dragover", { bubbles: true, cancelable: true, dataTransfer: data });
    element.dispatchEvent(over);
    const drop = new DragEvent("drop", { bubbles: true, cancelable: true, dataTransfer: data });
    element.dispatchEvent(drop);
    return { prevented: over.defaultPrevented, effect, dropped: drop.defaultPrevented };
  });
}

test("AS1 (page): files chosen, or dropped on the reading view, upload to the page, and at the root on the notebook's home; each is listed by name and size, opens or downloads as the same bytes, and is renamed and deleted from its menu", async ({
  api,
  db,
  nervewiki,
  signedInPage,
}, testInfo) => {
  const { tokens, adminId, pat, workspace } = await newOnboardedTeam(api, testInfo);
  const page = await signedInPage(tokens);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const guide = await createPage(api, pat, notebook.id, "Guide");
  const notesBytes = utf8("Notes beside the guide.\n");
  const uploads = countAnswers(page, "POST", uploadsPath(notebook.id));
  await page.goto(wikiPagePath(workspace.slug, notebook.id, guide.id));
  await expect(pageHeading(page, "Guide")).toBeVisible();
  const section = attachments(page);
  await expect(section.getByText("Drop files here to upload them.")).toBeVisible();
  // Over the page's heading nothing takes files: the page keeps a file dropped there from opening in the tab.
  expect(await dragFileOver(pageHeading(page, "Guide"))).toEqual({ prevented: true, effect: "none", dropped: true });

  // Upload opens the browser's picker, of several files.
  const choosing = page.waitForEvent("filechooser");
  await section.getByRole("button", { name: "Upload" }).click();
  const chooser = await choosing;
  expect(chooser.isMultiple()).toBe(true);
  await chooser.setFiles([
    { name: "diagram.png", mimeType: "image/png", buffer: Buffer.from(pngBytes) },
    { name: "notes.txt", mimeType: "text/plain", buffer: Buffer.from(notesBytes) },
  ]);
  await expect.poll(uploads).toBe(2);
  await expect(section.getByRole("list", { name: "Uploads" })).toBeHidden();
  await expect(section.getByRole("link", { name: "diagram.png (opens in a new tab)" })).toBeVisible();
  await expect(section.getByRole("link", { name: "notes.txt", exact: true })).toBeVisible();
  await expect(section.getByText("24 B")).toBeVisible();

  // One dragged over the reading view may drop there; dropped, its name taken, it goes as the next free one.
  const view = page.getByRole("article", { name: "Guide" });
  const over = await view.evaluate(
    (article, bytes) => {
      const data = new DataTransfer();
      data.items.add(new File([new Uint8Array(bytes)], "notes.txt", { type: "text/plain" }));
      // A transfer made by script is no drag's: the browser keeps no drop effect set on it. What is set is kept here.
      let effect = "none";
      Object.defineProperty(data, "dropEffect", {
        get: () => effect,
        set: (value: string) => {
          effect = value;
        },
      });
      const dragover = new DragEvent("dragover", { bubbles: true, cancelable: true, dataTransfer: data });
      article.dispatchEvent(dragover);
      const accepted = { prevented: dragover.defaultPrevented, effect };
      article.dispatchEvent(new DragEvent("drop", { bubbles: true, cancelable: true, dataTransfer: data }));
      return accepted;
    },
    [...notesBytes]
  );
  expect(over).toEqual({ prevented: true, effect: "copy" });
  await expect.poll(uploads).toBe(3);
  await expect(section.getByRole("link", { name: "notes 2.txt", exact: true })).toBeVisible();

  const listed = await listAssets(api, pat, notebook.id, guide.id);
  expect(listed.map((asset) => asset.name)).toEqual(["diagram.png", "notes 2.txt", "notes.txt"]);
  for (const asset of listed) {
    // oxlint-disable-next-line no-await-in-loop -- one at a time
    await expectUploaded(
      db,
      nervewiki.storageDir,
      asset,
      asset.name === "diagram.png" ? pngBytes : notesBytes,
      adminId,
      "web"
    );
  }

  // The name opens what the browser shows, the menu's Download downloads: the same bytes at each address.
  const name = section.getByRole("link", { name: "diagram.png (opens in a new tab)" });
  expect(await name.getAttribute("target")).toBe("_blank");
  const fetched = await download(nervewiki.baseURL, (await name.getAttribute("href")) ?? "");
  expect(Buffer.compare(Buffer.from(await fetched.arrayBuffer()), Buffer.from(pngBytes))).toBe(0);
  await section.getByRole("button", { name: "Actions for notes.txt" }).click();
  const downloading = page.waitForEvent("download");
  await page.getByRole("menuitem", { name: "Download" }).click();
  const downloaded = await downloading;
  expect(downloaded.suggestedFilename()).toBe("notes.txt");
  expect(Buffer.compare(readFileSync(await downloaded.path()), Buffer.from(notesBytes))).toBe(0);

  // Renamed by its stem, the extension staying; deleted once confirmed.
  const notes2 = listed.find((asset) => asset.name === "notes 2.txt");
  await section.getByRole("button", { name: "Actions for notes 2.txt" }).click();
  await page.getByRole("menuitem", { name: "Rename" }).click();
  const rename = page.getByRole("dialog", { name: "Rename notes 2.txt" });
  await rename.getByRole("textbox", { name: "Name" }).fill("draft");
  await rename.getByRole("button", { name: "Save" }).click();
  // The dialog's title is the new name as soon as the list has it: the dialog, whatever its title.
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(section.getByRole("link", { name: "draft.txt", exact: true })).toBeVisible();
  await expect(section.getByRole("button", { name: "Actions for draft.txt" })).toBeFocused();
  await section.getByRole("button", { name: "Actions for draft.txt" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  await page.getByRole("alertdialog", { name: "Delete draft.txt?" }).getByRole("button", { name: "Delete" }).click();
  await expect(section.getByRole("link", { name: "draft.txt", exact: true })).toBeHidden();
  await expect(section.getByRole("heading", { name: "Attachments" })).toBeFocused();
  expect((await listAssets(api, pat, notebook.id, guide.id)).map((asset) => asset.name)).toEqual([
    "diagram.png",
    "notes.txt",
  ]);
  await expectAssetsDeletedWithNodes(db, [notes2?.id ?? ""]);

  // On the notebook's home, the root's.
  await page.goto(`/${workspace.slug}/notebooks/${notebook.id}`);
  const root = attachments(page);
  await expect(root.getByRole("button", { name: "Upload" })).toBeVisible();
  await root
    .locator("input[type=file]")
    .setInputFiles([{ name: "logo.png", mimeType: "image/png", buffer: Buffer.from(pngBytes) }]);
  await expect(root.getByRole("link", { name: "logo.png (opens in a new tab)" })).toBeVisible();
  const atRoot = await listAssets(api, pat, notebook.id);
  expect(atRoot.map((asset) => [asset.name, asset.parent_id])).toEqual([["logo.png", null]]);
  await expectUploaded(db, nervewiki.storageDir, atRoot[0] as Asset, pngBytes, adminId, "web");
});

test("AS1 (page): an upload going shows its progress, of the whole request; cancelled, it stops, and the server keeps nothing of it", async ({
  api,
  db,
  nervewiki,
  signedInPage,
}, testInfo) => {
  const { tokens, pat, workspace } = await newOnboardedTeam(api, testInfo);
  const page = await signedInPage(tokens);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const guide = await createPage(api, pat, notebook.id, "Guide");
  // The browser sends about 100 KB a second, as CDP throttles it from before the page loads (set
  // after, it holds the progress back but not the body). The socket's buffers let some megabytes
  // ahead of it: 16 MiB take far longer than the cancel, which comes as the first bytes go.
  const cdp = await page.context().newCDPSession(page);
  await cdp.send("Network.enable");
  await cdp.send("Network.emulateNetworkConditions", {
    offline: false,
    latency: 0,
    downloadThroughput: -1,
    uploadThroughput: 100_000,
  });
  await page.goto(wikiPagePath(workspace.slug, notebook.id, guide.id));
  const section = attachments(page);
  await expect(section.getByRole("button", { name: "Upload" })).toBeVisible();
  const before = storedBlobs(nervewiki.storageDir);
  const size = 16 << 20;

  await section
    .locator("input[type=file]")
    .setInputFiles([{ name: "large.bin", mimeType: "application/octet-stream", buffer: randomBytes(size) }]);
  const progress = section.getByRole("progressbar", { name: "Upload of large.bin" });
  await expect.poll(async () => Number(await progress.getAttribute("value")), { intervals: [10] }).toBeGreaterThan(0);
  const [sent, total] = await progress.evaluate((bar: HTMLProgressElement) => [bar.value, bar.max]);
  // Of the request as the browser sends it: the form's bytes beside the file's.
  expect(total).toBeGreaterThan(size);
  expect(sent).toBeLessThan(total);
  await section.getByRole("button", { name: "Cancel the upload of large.bin" }).click();

  await expect(section.getByRole("list", { name: "Uploads" })).toBeHidden();
  await cdp.send("Network.emulateNetworkConditions", {
    offline: false,
    latency: 0,
    downloadThroughput: -1,
    uploadThroughput: -1,
  });
  expect(await listAssets(api, pat, notebook.id, guide.id)).toEqual([]);
  expect(await db.query("SELECT id FROM asset_blobs WHERE notebook_id = $1", [notebook.id])).toEqual([]);
  await expect.poll(() => storedBlobs(nervewiki.storageDir), { message: "the store's blobs/" }).toEqual(before);
});

test("AS1 (page): a Markdown file is not sent: the section says to import it, until dismissed", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { tokens, pat, workspace } = await newOnboardedTeam(api, testInfo);
  const page = await signedInPage(tokens);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const guide = await createPage(api, pat, notebook.id, "Guide");
  let sent = 0;
  page.on("request", (request) => {
    if (request.method() === "POST" && new URL(request.url()).pathname === uploadsPath(notebook.id)) {
      sent += 1;
    }
  });
  await page.goto(wikiPagePath(workspace.slug, notebook.id, guide.id));
  const section = attachments(page);
  await expect(section.getByRole("button", { name: "Upload" })).toBeVisible();

  await section
    .locator("input[type=file]")
    .setInputFiles([{ name: "notes.md", mimeType: "text/markdown", buffer: Buffer.from("# Notes\n") }]);

  await expect(
    section.getByText("A Markdown file is a page: zip it and import it from the notebook's settings.")
  ).toBeVisible();
  expect(sent).toBe(0);
  await section.getByRole("button", { name: "Dismiss the upload of notes.md" }).click();
  await expect(section.getByRole("list", { name: "Uploads" })).toBeHidden();
  await expect(section.getByRole("button", { name: "Upload" })).toBeFocused();
});
