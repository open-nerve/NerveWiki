import { readFileSync } from "node:fs";

import type { ApiClient } from "@nervewiki/api-client";
import type { Locator, Page } from "@playwright/test";

import { expectIndexedLinks } from "../../fixtures/assert/links";
import { download, oggOpus, pngBytes, uploadAsset, utf8 } from "../../fixtures/assets";
import { getPageProperties } from "../../fixtures/links";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, getView, readContent, renameNode } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// AS3, attachments shown where links lead to them (M7 design 9; M7/P3
// design 5.5, 5.6; M7/P4 design 4): an embed of an image, an audio or a
// video is its element, at its content's address, signed; any other
// attachment's, and a link to one, a link, which downloads one the browser
// does not show (download) and opens one it does in a tab of its own; an
// embed of no attachment is unresolved. A renamed attachment's embeds are
// written again, still leading to it; the index has the attachments' ids.
// In the page, the media load from their addresses, and one playing goes on
// as the view is read again.

/** The page that shows them: a caption and a width, an audio, a video, a PDF, a link, and an embed of none. */
const content =
  '---\ncover: "[[cover.png]]"\nfile: "[[data.zip]]"\n---\n' +
  "![[cover.png|封面|300]]\n\n![[sound.ogg]] ![[clip.webm]]\n\n![[doc.pdf]] [[data.zip|the data]] ![[missing.png]]\n";

const pdfBytes = utf8("%PDF-1.4\n% a document\n");
const zipBytes = Uint8Array.from([0x50, 0x4b, 0x03, 0x04, ...utf8("an archive")]);
/** WebM's first bytes, which the server sniffs; the page version records a video the browser plays. */
const webmHead = Uint8Array.from([0x1a, 0x45, 0xdf, 0xa3, 0x9f, 0x42, 0x86, 0x81, 0x01]);

/** upload uploads the story's attachments, at the notebook's root, its video's bytes video. */
async function upload(api: ApiClient, pat: string, notebook: string, video: Uint8Array<ArrayBuffer>) {
  const [cover, sound, clip, doc, data] = await Promise.all([
    uploadAsset(api, pat, notebook, { name: "cover.png", bytes: pngBytes }),
    uploadAsset(api, pat, notebook, { name: "sound.ogg", bytes: oggOpus(30) }),
    uploadAsset(api, pat, notebook, { name: "clip.webm", bytes: video }),
    uploadAsset(api, pat, notebook, { name: "doc.pdf", bytes: pdfBytes }),
    uploadAsset(api, pat, notebook, { name: "data.zip", bytes: zipBytes }),
  ]);
  return { cover, sound, clip, doc, data };
}

/** path is the path of the attachment id's content. */
const path = (id: string) => `/api/v0/assets/${id}/content`;

/** attrsOf is the attributes of each element of html named element whose class has nw-asset, in order. */
function attrsOf(html: string, element: string): Record<string, string>[] {
  return [...html.matchAll(new RegExp(`<${element}\\b([^>]*)>`, "g"))]
    .map((match) =>
      Object.fromEntries(
        [...(match[1] ?? "").matchAll(/([\w-]+)="([^"]*)"/g)].map(([, name, value]) => [
          name,
          value?.replaceAll("&amp;", "&"),
        ])
      )
    )
    .filter((attrs) => (attrs.class ?? "").split(" ").includes("nw-asset"));
}

test("AS3 (API): the view has an attachment's image, audio and video at their signed addresses, a link to any other, downloading one the browser does not show; a rename writes the embeds again; the index has the attachments' ids", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const { cover, sound, clip, doc, data } = await upload(api, pat, notebook.id, webmHead);
  expect([cover.mime, sound.mime, clip.mime, doc.mime, data.mime]).toEqual([
    "image/png",
    "audio/ogg",
    "video/webm",
    "application/pdf",
    "application/octet-stream",
  ]);
  const guide = await createPage(api, pat, notebook.id, "Guide", null, content);

  const view = (await getView(api, pat, guide.id)).data;
  const html = view?.html ?? "";
  const [img] = attrsOf(html, "img");
  expect([img?.alt, img?.width, img?.src?.split("?")[0]]).toEqual(["封面", "300", path(cover.id)]);
  expect(attrsOf(html, "audio").map((a) => a.src?.split("?")[0])).toEqual([path(sound.id)]);
  expect(attrsOf(html, "video").map((v) => v.src?.split("?")[0])).toEqual([path(clip.id)]);
  // A link carries download when the browser would not show what it leads to.
  expect(attrsOf(html, "a").map((a) => [a.href?.split("?")[0], "download" in a])).toEqual([
    [path(cover.id), false],
    [path(data.id), true],
    [path(doc.id), false],
    [path(data.id), true],
  ]);
  expect(html).toContain('data-nw-target="missing.png"');
  // The view expires as its addresses do.
  const expiry = new URL(img?.src ?? "", nervewiki.baseURL).searchParams.get("e");
  expect(view?.assets_expire_at && Date.parse(view.assets_expire_at) / 1000).toBe(Number(expiry));
  const archive = attrsOf(html, "a").find((a) => "download" in a && a.href?.startsWith(path(data.id)));
  for (const [address, bytes] of [
    [img?.src, pngBytes],
    [archive?.href, zipBytes],
  ] as const) {
    // oxlint-disable-next-line no-await-in-loop -- one download at a time, each checked
    const answer = await download(nervewiki.baseURL, address ?? "");
    // oxlint-disable-next-line no-await-in-loop -- its body
    expect(Buffer.compare(Buffer.from(await answer.arrayBuffer()), Buffer.from(bytes))).toBe(0);
  }

  // The properties' links say whether the browser shows what they lead to, and when their addresses expire.
  const properties = await getPageProperties(api, pat, guide.id);
  expect(properties.response.status).toBe(200);
  const links = properties.data?.links ?? [];
  expect(links.map((l) => [l.key, l.node_id, l.inline, l.url?.split("?")[0]])).toEqual([
    ["cover", cover.id, true, path(cover.id)],
    ["file", data.id, false, path(data.id)],
  ]);
  const linkExpiry = new URL(links[0]?.url ?? "", nervewiki.baseURL).searchParams.get("e");
  expect(properties.data?.assets_expire_at && Date.parse(properties.data.assets_expire_at) / 1000).toBe(
    Number(linkExpiry)
  );

  await expectIndexedLinks(db, guide.id, [
    { kind: "wikilink", property: "cover", target: "cover.png", resolved: cover.id },
    { kind: "wikilink", property: "file", target: "data.zip", resolved: data.id },
    { kind: "embed", property: null, target: "cover.png", resolved: cover.id },
    { kind: "embed", property: null, target: "sound.ogg", resolved: sound.id },
    { kind: "embed", property: null, target: "clip.webm", resolved: clip.id },
    { kind: "embed", property: null, target: "doc.pdf", resolved: doc.id },
    { kind: "wikilink", property: null, target: "data.zip", resolved: data.id },
    { kind: "embed", property: null, target: "missing.png", resolved: null },
  ]);

  // Renamed, its embeds and its property's link are written again, and lead to it still.
  expect((await renameNode(api, pat, cover.id, "front.png")).response.status).toBe(200);
  expect((await readContent(api, pat, guide.id)).content).toBe(content.replaceAll("[[cover.png", "[[front.png"));
  const again = (await getView(api, pat, guide.id)).data?.html ?? "";
  expect(attrsOf(again, "img").map((i) => i.src?.split("?")[0])).toEqual([path(cover.id)]);
  await expectIndexedLinks(db, guide.id, [
    { kind: "wikilink", property: "cover", target: "front.png", resolved: cover.id },
    { kind: "wikilink", property: "file", target: "data.zip", resolved: data.id },
    { kind: "embed", property: null, target: "front.png", resolved: cover.id },
    { kind: "embed", property: null, target: "sound.ogg", resolved: sound.id },
    { kind: "embed", property: null, target: "clip.webm", resolved: clip.id },
    { kind: "embed", property: null, target: "doc.pdf", resolved: doc.id },
    { kind: "wikilink", property: null, target: "data.zip", resolved: data.id },
    { kind: "embed", property: null, target: "missing.png", resolved: null },
  ]);
});

/** recordWebm records, in page, a short video of a canvas as WebM: Playwright's Chromium plays VP8, not H.264. */
async function recordWebm(page: Page): Promise<Uint8Array<ArrayBuffer>> {
  const bytes = await page.evaluate(async () => {
    const canvas = document.createElement("canvas");
    canvas.width = 16;
    canvas.height = 8;
    const drawing = canvas.getContext("2d");
    const recorder = new MediaRecorder(canvas.captureStream(10), { mimeType: "video/webm;codecs=vp8" });
    const chunks: Blob[] = [];
    recorder.addEventListener("dataavailable", (event) => chunks.push(event.data));
    const stopped = new Promise((resolve) => recorder.addEventListener("stop", resolve));
    recorder.start();
    for (let frame = 0; frame < 6; frame++) {
      if (drawing) {
        drawing.fillStyle = frame % 2 === 0 ? "red" : "blue";
        drawing.fillRect(0, 0, 16, 8);
      }
      // oxlint-disable-next-line no-await-in-loop -- a frame at a time
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    recorder.stop();
    await stopped;
    return [...new Uint8Array(await new Blob(chunks).arrayBuffer())];
  });
  return Uint8Array.from(bytes);
}

/** loaded tells, once media is asked to load its metadata (the view writes preload="none"), whether it has. */
function loaded(media: Locator): Promise<boolean> {
  return media.evaluate(
    (element) =>
      new Promise<boolean>((resolve) => {
        if (!(element instanceof HTMLMediaElement)) {
          resolve(false);
          return;
        }
        element.addEventListener("loadedmetadata", () => resolve(true), { once: true });
        element.addEventListener("error", () => resolve(false), { once: true });
        element.preload = "metadata";
        element.load();
      })
  );
}

test("AS3 (page): the image, the audio and the video load from their addresses; a PDF opens in a tab of its own, an archive downloads, each with its size, the properties' too; an embed of none is unresolved; an audio playing goes on as a rename reads the view again", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { tokens, pat, workspace } = await newOnboardedTeam(api, testInfo);
  const page = await signedInPage(tokens);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  await page.goto(`/${workspace.slug}/notebooks/${notebook.id}`);
  const { cover, sound, clip, doc, data } = await upload(api, pat, notebook.id, await recordWebm(page));
  const guide = await createPage(api, pat, notebook.id, "Guide", null, content);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, guide.id));
  await expect(pageHeading(page, "Guide")).toBeVisible();
  const view = page.getByRole("article", { name: "Guide" });

  const image = view.getByRole("img", { name: "封面" });
  await expect(image).toHaveAttribute("width", "300");
  await expect.poll(() => image.evaluate((img) => (img as HTMLImageElement).naturalWidth)).toBe(2);
  const audio = view.locator("audio.nw-asset");
  const video = view.locator("video.nw-asset");
  await expect(audio).toHaveAttribute("src", new RegExp(`^/api/v0/assets/${sound.id}/content\\?`));
  expect(await loaded(audio)).toBe(true);
  expect(await loaded(video)).toBe(true);
  expect(await video.evaluate((element) => (element as HTMLVideoElement).videoWidth)).toBe(16);

  // A PDF opens in a tab of its own, leaving the page; an archive downloads, the same bytes.
  const pdf = view.getByRole("link", { name: "doc.pdf (opens in a new tab)" });
  await expect(pdf).toHaveAttribute("target", "_blank");
  expect(await pdf.getAttribute("download")).toBeNull();
  // The headless browser has no viewer of PDFs: the tab asks for the document, and shows nothing.
  const opening = page.waitForEvent("popup");
  const asked = page
    .context()
    .waitForEvent("request", (request) => new URL(request.url()).pathname === `/api/v0/assets/${doc.id}/content`);
  await pdf.click();
  const tab = await opening;
  expect((await asked).frame().page()).toBe(tab);
  await tab.close();
  const archive = view.getByRole("link", { name: "the data", exact: true });
  await expect(archive).toHaveAttribute("download");
  const downloading = page.waitForEvent("download");
  await archive.click();
  const downloaded = await downloading;
  expect(downloaded.suggestedFilename()).toBe("data.zip");
  expect(Buffer.compare(readFileSync(await downloaded.path()), Buffer.from(zipBytes))).toBe(0);
  await expect(view.getByText("missing.png")).toHaveClass(/nw-unresolved/);
  expect(page.url()).toContain(guide.id);
  // Each link says its size, in the reader's language.
  await expect(view.locator("p").filter({ hasText: "doc.pdf" })).toContainText(
    `doc.pdf (opens in a new tab) (${pdfBytes.length.toString()} B) the data (${zipBytes.length.toString()} B) missing.png`
  );

  // The properties' links: the image opens in a tab of its own, the archive downloads.
  const properties = page.getByRole("complementary", { name: "About this page" });
  const coverLink = properties.getByRole("link", { name: "cover.png (opens in a new tab)" });
  await expect(coverLink).toHaveAttribute("target", "_blank");
  await expect(coverLink).toHaveAttribute("href", new RegExp(`^/api/v0/assets/${cover.id}/content\\?`));
  const fileLink = properties.getByRole("link", { name: "data.zip", exact: true });
  await expect(fileLink).toHaveAttribute("download");
  const fetching = page.waitForEvent("download");
  await fileLink.click();
  expect(Buffer.compare(readFileSync(await (await fetching).path()), Buffer.from(zipBytes))).toBe(0);

  // Playing, the audio goes on as the view is read again: the rename writes the page's embeds again.
  await audio.evaluate((element) => {
    const media = element as HTMLMediaElement & { kept?: boolean };
    media.kept = true;
    media.muted = true;
    return media.play();
  });
  await expect.poll(() => audio.evaluate((element) => (element as HTMLMediaElement).currentTime)).toBeGreaterThan(0.3);
  await image.evaluate((element) => {
    (element as HTMLImageElement & { old?: boolean }).old = true;
  });
  expect((await renameNode(api, pat, cover.id, "front.png")).response.status).toBe(200);
  await expect
    .poll(() =>
      view.locator("img.nw-asset").evaluate((element) => (element as HTMLImageElement & { old?: boolean }).old)
    )
    .toBeUndefined();
  const playing = await audio.evaluate((element) => {
    const media = element as HTMLMediaElement & { kept?: boolean };
    return { kept: media.kept, paused: media.paused, at: media.currentTime };
  });
  expect([playing.kept, playing.paused]).toEqual([true, false]);
  await expect
    .poll(() => audio.evaluate((element) => (element as HTMLMediaElement).currentTime))
    .toBeGreaterThan(playing.at);
  await expect(view.getByRole("img", { name: "封面" })).toHaveAttribute(
    "src",
    new RegExp(`^/api/v0/assets/${cover.id}/content\\?`)
  );
  await expectIndexedLinks(db, guide.id, [
    { kind: "wikilink", property: "cover", target: "front.png", resolved: cover.id },
    { kind: "wikilink", property: "file", target: "data.zip", resolved: data.id },
    { kind: "embed", property: null, target: "front.png", resolved: cover.id },
    { kind: "embed", property: null, target: "sound.ogg", resolved: sound.id },
    { kind: "embed", property: null, target: "clip.webm", resolved: clip.id },
    { kind: "embed", property: null, target: "doc.pdf", resolved: doc.id },
    { kind: "wikilink", property: null, target: "data.zip", resolved: data.id },
    { kind: "embed", property: null, target: "missing.png", resolved: null },
  ]);
});
