import type { Locator } from "@playwright/test";

import { expectEmbedded } from "../../fixtures/assert/asset";
import { expectChangesetOf } from "../../fixtures/assert/page";
import { download, listAssets, pngBytes, uploadAsset } from "../../fixtures/assets";
import { countAnswers } from "../../fixtures/browser";
import { getPageProperties, listTags } from "../../fixtures/links";
import { mcpClient, readPage, resultOf, writtenOf } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, getView, readContent, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { contentWrites, pageHeading, saveEdit, startEditing, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// AS2, files pasted into the editor or dropped on it (M7 design 9; M7/P4 design 5): each uploads as the page's
// attachment and its embed goes where it was pasted or dropped; the reading view shows them, the index leads to them.
// The API version uploads them, then writes the page with their embeds: the same database's assertions. The MCP
// version's agent reads the images uploaded, and writes the page with their embeds by the links read gives (M9/P5
// design 3.5).

/** The origin the server writes its addresses with (fixtures/server.ts). */
const publicOrigin = "http://wiki.example";

/** A write's answer, as much as AS2 reads of the tool's structuredContent. */
interface Written {
  node_id: string;
  version: string;
  changeset_id: string | null;
  at: string | null;
}

/** The page's content before the files: its text, the cursor going after it. */
const before = "Notes of the day.\n";

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

  const content = `Notes of![[${dropped.link}]] the day.\n![[${pasted.link}]] and a chart:`;
  const written = await writeContent(api, pat, guide.id, { content, base_revision: 1 });

  await expectEmbedded(
    db,
    nervewiki.storageDir,
    { written, content, attachments: [dropped, pasted] },
    pngBytes,
    adminId,
    "api"
  );
  const html = (await getView(api, pat, guide.id)).data?.html ?? "";
  expect([...html.matchAll(/<img class="nw-asset" src="([^"?]*)/g)].map(([, src]) => src)).toEqual([
    `/api/v0/assets/${dropped.id}/content`,
    `/api/v0/assets/${pasted.id}/content`,
  ]);
});

test("AS2 (MCP): an agent reads the page's uploaded images, each as itself with how to link it and its address, and writes the page with their embeds; the index leads them to the attachments", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, adminId, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const guide = await createPage(api, pat, notebook.id, "Guide", null, before);
  // Another chart.png, uploaded first elsewhere: Guide's, uploaded after it, is linked by its path, not its name.
  const other = await createPage(api, pat, notebook.id, "Other");
  await uploadAsset(api, pat, notebook.id, { name: "chart.png", bytes: pngBytes, type: "image/png" }, other.id);
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
  expect([dropped.link, pasted.link]).toEqual(["Guide/chart.png", "Pasted image 20261009120000.png"]);
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });
  const { version } = (await readPage(agent, { path: "Guide.md" })).meta;

  // Each image as itself, how to link it, and its signed address, which downloads its bytes.
  const links: string[] = [];
  for (const asset of [dropped, pasted]) {
    // oxlint-disable-next-line no-await-in-loop -- one attachment at a time
    const answer = await agent.call("read", { path: `Guide/${asset.name}` });
    expect(answer.isError, answer.text.join(" ")).toBe(false);
    const meta = JSON.parse(answer.text[0] ?? "{}") as { link: string | null; url: string | null };
    expect(meta.link, asset.name).toBe(asset.link);
    const address = new URL(meta.url ?? "");
    expect([address.origin, address.pathname]).toEqual([publicOrigin, `/api/v0/assets/${asset.id}/content`]);
    // oxlint-disable-next-line no-await-in-loop -- its download, at the server's own address
    const got = await download(nervewiki.baseURL, address.pathname + address.search);
    expect(got.status).toBe(200);
    // oxlint-disable-next-line no-await-in-loop -- its body
    expect(Buffer.compare(Buffer.from(await got.arrayBuffer()), Buffer.from(pngBytes))).toBe(0);
    const image = answer.content[1];
    expect(image?.type === "image" && Buffer.compare(Buffer.from(image.data, "base64"), Buffer.from(pngBytes))).toBe(0);
    links.push(meta.link ?? "");
  }

  // The page written with the embeds by the links read gave; the upload's client is REST's, the write the agent's.
  const content = `Notes of![[${links[0]}]] the day.\n![[${links[1]}]] and a chart:`;
  const written = resultOf<Written>(
    await agent.call("write", { path: "Guide.md", content, base_version: version }),
    "write Guide"
  );
  await expectEmbedded(
    db,
    nervewiki.storageDir,
    { written: writtenOf(written, "write Guide"), content, attachments: [dropped, pasted] },
    pngBytes,
    adminId,
    "api"
  );
  // A write of content alone moves no node: no item.
  await expectChangesetOf(db, String(written.changeset_id), {
    client: "mcp:e2e-agent",
    byId: adminId,
    items: [],
    versions: [guide.id],
  });
  const html = (await getView(api, pat, guide.id)).data?.html ?? "";
  expect([...html.matchAll(/<img class="nw-asset" src="([^"?]*)/g)].map(([, src]) => src)).toEqual([
    `/api/v0/assets/${dropped.id}/content`,
    `/api/v0/assets/${pasted.id}/content`,
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
  const writes = contentWrites(page, guide.id);
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
  // The cursor went after the embed, as after a paste: what is typed next follows it.
  await page.keyboard.type(" and a chart:");
  await expect(content).toContainText(/!\[\[Pasted image \d{14}\.png\]\] and a chart:$/);

  // Short as the text is, the content fills the editor: a file may drop below it too.
  const below = await content.evaluate((element) => {
    const editor = element.closest(".cm-editor");
    editor?.scrollIntoView({ block: "end" });
    const box = editor?.getBoundingClientRect();
    return element.contains(document.elementFromPoint((box?.right ?? 0) - 24, (box?.bottom ?? 0) - 24));
  });
  expect(below).toBe(true);

  // A file dragged from outside drops where it is let go: in the first line, after "Notes of".
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
      const text = element.querySelector(".cm-line")?.firstChild ?? element;
      const letter = document.createRange();
      letter.setStart(text, "Notes o".length);
      letter.setEnd(text, "Notes of".length);
      const box = letter.getBoundingClientRect();
      const at = { clientX: box.right - 1, clientY: (box.top + box.bottom) / 2 };
      // What is there, as the browser would send the drag's events to it.
      const target = document.elementFromPoint(at.clientX, at.clientY) ?? document.body;
      const dragover = new DragEvent("dragover", { bubbles: true, cancelable: true, dataTransfer: data, ...at });
      target.dispatchEvent(dragover);
      const accepted = { content: element.contains(target), prevented: dragover.defaultPrevented, effect };
      target.dispatchEvent(new DragEvent("drop", { bubbles: true, cancelable: true, dataTransfer: data, ...at }));
      return accepted;
    },
    [...pngBytes]
  );
  expect(over).toEqual({ content: true, prevented: true, effect: "copy" });
  await expect.poll(uploads).toBe(2);
  await expect(content).toContainText("![[chart.png]]");

  await saveEdit(page);
  const listed = await listAssets(api, pat, notebook.id, guide.id);
  const pasted = listed.find((asset) => asset.name.startsWith("Pasted image "));
  const dropped = listed.find((asset) => asset.name === "chart.png");
  if (pasted === undefined || dropped === undefined) {
    throw new Error(`the attachments listed are ${JSON.stringify(listed.map((asset) => asset.name))}`);
  }
  expect(pasted.name).toMatch(/^Pasted image \d{14}\.png$/);
  const text = `Notes of![[chart.png]] the day.\n![[${pasted.link}]] and a chart:`;
  expect((await readContent(api, pat, guide.id)).content).toBe(text);
  await expectEmbedded(
    db,
    nervewiki.storageDir,
    { written: await writes.saved(), content: text, attachments: [dropped, pasted] },
    pngBytes,
    adminId,
    "web"
  );

  await page.getByRole("main").getByRole("button", { name: "Done", exact: true }).click();
  const article = page.getByRole("article", { name: "Guide" });
  expect(await shown(article, 2)).toEqual([
    `/api/v0/assets/${dropped.id}/content`,
    `/api/v0/assets/${pasted.id}/content`,
  ]);
});

test("AS2 (page, frontmatter): a file pasted in a property's quotes goes in as the property's YAML string writes it; saved, the page keeps its properties, its tags and its property links (Codex M7 review R1)", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { tokens, pat, workspace } = await newOnboardedTeam(api, testInfo);
  const page = await signedInPage(tokens);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const guide = await createPage(api, pat, notebook.id, "Guide", null, "");
  const board = await createPage(
    api,
    pat,
    notebook.id,
    "Board",
    null,
    "---\ntags: [retained]\nimage: ''\nref: '[[Guide]]'\n---\nBody\n"
  );
  const uploads = countAnswers(page, "POST", `/api/v0/notebooks/${notebook.id}/assets`);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, board.id));
  await expect(pageHeading(page, "Board")).toBeVisible();
  const content = await startEditing(page);
  // Between the quotes of image's value.
  await page.keyboard.press("ControlOrMeta+Home");
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("End");
  await page.keyboard.press("ArrowLeft");
  await content.evaluate(
    (element, bytes) => {
      const data = new DataTransfer();
      data.items.add(new File([new Uint8Array(bytes)], "Bob's.png", { type: "image/png" }));
      element.dispatchEvent(new ClipboardEvent("paste", { bubbles: true, cancelable: true, clipboardData: data }));
    },
    [...pngBytes]
  );
  await expect.poll(uploads).toBe(1);
  await expect(content).toContainText("image: '![[Bob''s.png]]'");
  await saveEdit(page);

  expect((await readContent(api, pat, board.id)).content).toBe(
    "---\ntags: [retained]\nimage: '![[Bob''s.png]]'\nref: '[[Guide]]'\n---\nBody\n"
  );
  const properties = (await getPageProperties(api, pat, board.id)).data;
  expect(properties?.valid).toBe(true);
  expect(properties?.properties).toEqual([
    { key: "tags", value: ["retained"] },
    { key: "image", value: "![[Bob's.png]]" },
    { key: "ref", value: "[[Guide]]" },
  ]);
  expect(properties?.links).toContainEqual({ key: "ref", node_id: guide.id, kind: "page", url: null, inline: null });
  expect((await listTags(api, pat, notebook.id)).data?.data).toEqual([{ tag: "retained", count: 1 }]);
});
