import type { Asset } from "@nervewiki/api-client";
import type { Locator } from "@playwright/test";

import { expectUploaded } from "../../fixtures/assert/asset";
import { expectIndexedLinks } from "../../fixtures/assert/links";
import { listAssets, pngBytes, uploadAsset } from "../../fixtures/assets";
import { countAnswers } from "../../fixtures/browser";
import type { Database } from "../../fixtures/db";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, getView, readContent, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, saveEdit, startEditing, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// AS2, files pasted into the editor or dropped on it (M7 design 9; M7/P4 design 5): each uploads as the page's
// attachment and its embed goes where it was pasted or dropped; the reading view shows them, the index leads to them.
// The API version uploads them, then writes the page with their embeds: the same database's assertions.

/** The page's content before the files: its text, the cursor going after it. */
const before = "Notes of the day.\n";

/**
 * expectEmbedded is the database after the attachments were uploaded under page by creatorId through client, and
 * embedded in its content in order: each stored as uploaded, and the index leading each embed to its attachment.
 */
async function expectEmbedded(
  db: Database,
  storageDir: string,
  page: string,
  attachments: readonly Asset[],
  creatorId: string,
  client: string
): Promise<void> {
  for (const asset of attachments) {
    // oxlint-disable-next-line no-await-in-loop -- one at a time
    await expectUploaded(db, storageDir, asset, pngBytes, creatorId, client);
    expect(asset.parent_id).toBe(page);
  }
  await expectIndexedLinks(
    db,
    page,
    attachments.map((asset) => ({ kind: "embed", property: null, target: asset.link ?? "", resolved: asset.id }))
  );
}

/** shown waits for each image of article, the attachments' own, to load: their sources. */
async function shown(article: Locator, count: number): Promise<string[]> {
  const images = article.locator("img.nw-asset");
  await expect(images).toHaveCount(count);
  await expect
    .poll(() => images.evaluateAll((all) => all.every((image) => (image as HTMLImageElement).naturalWidth > 0)))
    .toBe(true);
  return images.evaluateAll((all) => all.map((image) => new URL((image as HTMLImageElement).src).pathname));
}

test("AS2 (API): images uploaded under a page, and the page written with their embeds, the view shows them; the index leads to them", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, adminId, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const guide = await createPage(api, pat, notebook.id, "Guide", null, before);
  const pasted = await uploadAsset(
    api,
    pat,
    notebook.id,
    { name: "Pasted image 20261009120000.png", bytes: pngBytes, type: "image/png" },
    guide.id
  );
  const dropped = await uploadAsset(
    api,
    pat,
    notebook.id,
    { name: "chart.png", bytes: pngBytes, type: "image/png" },
    guide.id
  );

  await writeContent(api, pat, guide.id, {
    content: `${before}![[${pasted.link}]]![[${dropped.link}]]`,
    base_revision: 1,
  });

  await expectEmbedded(db, nervewiki.storageDir, guide.id, [pasted, dropped], adminId, "api");
  const html = (await getView(api, pat, guide.id)).data?.html ?? "";
  expect([...html.matchAll(/<img class="nw-asset" src="([^"?]*)/g)].map(([, src]) => src)).toEqual([
    `/api/v0/assets/${pasted.id}/content`,
    `/api/v0/assets/${dropped.id}/content`,
  ]);
});

test("AS2 (page): an image pasted into the editor and one dropped on it upload as the page's attachments, their embeds where they went; the reading view shows them", async ({
  api,
  db,
  nervewiki,
  signedInPage,
}, testInfo) => {
  const { tokens, adminId, pat, workspace } = await newOnboardedTeam(api, testInfo);
  const page = await signedInPage(tokens);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const guide = await createPage(api, pat, notebook.id, "Guide", null, before);
  const uploads = countAnswers(page, "POST", `/api/v0/notebooks/${notebook.id}/assets`);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, guide.id));
  await expect(pageHeading(page, "Guide")).toBeVisible();
  const content = await startEditing(page);
  await page.keyboard.press("ControlOrMeta+End");

  // An image the clipboard holds without a name of its own, as a screenshot copied: named as Obsidian names it.
  await content.evaluate(
    (element, bytes) => {
      const data = new DataTransfer();
      data.items.add(new File([new Uint8Array(bytes)], "image.png", { type: "image/png" }));
      element.dispatchEvent(new ClipboardEvent("paste", { bubbles: true, cancelable: true, clipboardData: data }));
    },
    [...pngBytes]
  );
  await expect.poll(uploads).toBe(1);
  await expect(content).toContainText(/!\[\[Pasted image \d{14}\.png\]\]/);

  // A file dragged from outside may drop on the text, at the end of its last line.
  const over = await content.evaluate(
    (element, bytes) => {
      const data = new DataTransfer();
      data.items.add(new File([new Uint8Array(bytes)], "chart.png", { type: "image/png" }));
      // A transfer made by script is no drag's: the browser keeps no drop effect set on it. What is set is kept here.
      let effect = "none";
      Object.defineProperty(data, "dropEffect", {
        get: () => effect,
        set: (value: string) => {
          effect = value;
        },
      });
      const last = [...element.querySelectorAll(".cm-line")].at(-1)?.getBoundingClientRect();
      const at = { clientX: (last?.right ?? 0) + 2, clientY: ((last?.top ?? 0) + (last?.bottom ?? 0)) / 2 };
      const dragover = new DragEvent("dragover", { bubbles: true, cancelable: true, dataTransfer: data, ...at });
      element.dispatchEvent(dragover);
      const accepted = { prevented: dragover.defaultPrevented, effect };
      element.dispatchEvent(new DragEvent("drop", { bubbles: true, cancelable: true, dataTransfer: data, ...at }));
      return accepted;
    },
    [...pngBytes]
  );
  expect(over).toEqual({ prevented: true, effect: "copy" });
  await expect.poll(uploads).toBe(2);
  await expect(content).toContainText("![[chart.png]]");

  await saveEdit(page);
  const listed = await listAssets(api, pat, notebook.id, guide.id);
  const pasted = listed.find((asset) => asset.name.startsWith("Pasted image "));
  const dropped = listed.find((asset) => asset.name === "chart.png");
  expect(pasted?.name).toMatch(/^Pasted image \d{14}\.png$/);
  expect((await readContent(api, pat, guide.id)).content).toBe(`${before}![[${pasted?.link}]]![[chart.png]]`);
  await expectEmbedded(
    db,
    nervewiki.storageDir,
    guide.id,
    [pasted, dropped].filter((asset) => asset !== undefined),
    adminId,
    "web"
  );

  await page.getByRole("main").getByRole("button", { name: "Done", exact: true }).click();
  const article = page.getByRole("article", { name: "Guide" });
  expect(await shown(article, 2)).toEqual([
    `/api/v0/assets/${pasted?.id}/content`,
    `/api/v0/assets/${dropped?.id}/content`,
  ]);
});
