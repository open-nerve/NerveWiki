import { readFileSync } from "node:fs";
import { setTimeout as sleep } from "node:timers/promises";
import { deflateSync } from "node:zlib";

import { createClient } from "@nervewiki/api-client";
import { test } from "@playwright/test";

import { login, signInContext } from "../fixtures/auth";
import { uploadAsset } from "../fixtures/assets";
import { watchPage } from "../fixtures/browser";
import { goto } from "../fixtures/navigation";
import { createNotebook } from "../fixtures/notebooks";
import { createPage, writeContent } from "../fixtures/pages";
import { record } from "./record";
import { perfEnv } from "./setup";
import { pngOf } from "./shapes";

// M4-performance items 11 and 13, only recorded (M12/P4 design 4.6), measured once P2/S4 is merged (decision 424):
// a page of 20 PNGs of 4,000 × 4,000 (a plain pattern: small files, each about 64 MB decoded), attachments of the
// page, in its reading view, where they load lazily and with their size. The page opened, then scrolled to its end
// 200 px at a time; every 100 ms each Chromium process's VmHWM (/proc, its pid from CDP's SystemInfo), the renderers'
// and the GPU's apart: a decoded image may live in either (design 4.6, review C's U2); and the JS heap at the end.
// Two rounds. Moved to P4/S3 while P2/S4 is not merged (M12/P4/S2's brief).

const round = process.env.NW_PERF_ROUND ?? "1";
const images = 20;
const side = 4_000;

test("large images in the reading view", async ({ browser }) => {
  const env = perfEnv();
  const api = createClient({ baseUrl: env.baseURL });
  const admin = await login(api, env.admin);
  const credential = admin.access_token;
  const notebook = await createNotebook(api, credential, env.workspace, `Images ${round}`, "editor");
  const page = await createPage(api, credential, notebook.id, "Images");
  let bytes = 0;
  for (let i = 1; i <= images; i++) {
    const png = pngOf(side, side, (x, y) => ((x >> 6) + (y >> 6) + i) & 0xff, deflateSync);
    bytes += png.length;
    // oxlint-disable-next-line no-await-in-loop -- one upload at a time
    await uploadAsset(
      api,
      credential,
      notebook.id,
      { name: `image-${i}.png`, bytes: new Uint8Array(png), type: "image/png" },
      page.id
    );
  }
  const content = Array.from({ length: images }, (_, i) => `![[image-${i + 1}.png]]`).join("\n\n");
  await writeContent(api, credential, page.id, { content: `${content}\n`, base_revision: page.revision });

  const cdp = await browser.newBrowserCDPSession();
  const peaks = new Map<number, { type: string; start: number; peak: number }>();
  const sample = async () => {
    const { processInfo } = (await cdp.send("SystemInfo.getProcessInfo")) as {
      processInfo: { type: string; id: number }[];
    };
    for (const { type, id } of processInfo) {
      const kb = highWater(id);
      if (kb === undefined) continue;
      const seen = peaks.get(id);
      if (seen === undefined) peaks.set(id, { type, start: kb, peak: kb });
      else seen.peak = Math.max(seen.peak, kb);
    }
  };
  const context = await browser.newContext({ baseURL: env.baseURL });
  const sampling = new AbortController();
  const sampler = (async () => {
    while (!sampling.signal.aborted) {
      // oxlint-disable-next-line no-await-in-loop -- every 100 ms
      await sample().catch(() => undefined);
      // oxlint-disable-next-line no-await-in-loop -- the same
      await sleep(100);
    }
  })();
  try {
    await signInContext(context, env.baseURL, admin);
    const tab = await context.newPage();
    await watchPage(tab);
    await goto(tab, `/${env.workspace}/notebooks/${notebook.id}/pages/${page.id}`);
    await tab.getByRole("main").getByRole("article").locator("img").first().waitFor();
    for (;;) {
      // oxlint-disable-next-line no-await-in-loop -- 200 px at a time, a pause for what came into view to load
      const end = await tab.evaluate(() => {
        scrollBy(0, 200);
        return scrollY + innerHeight >= document.documentElement.scrollHeight - 1;
      });
      // oxlint-disable-next-line no-await-in-loop -- the same
      await sleep(150);
      if (end) break;
    }
    await sleep(2_000);
    // Those loaded: an image that failed is complete too, but has no width (review C's M5 on M12/P4/S2). Loaded is not
    // decoded: Chromium decodes as it paints.
    const loaded = await tab.evaluate(
      () =>
        [...document.querySelectorAll("article img")].filter(
          (img) => (img as HTMLImageElement).complete && (img as HTMLImageElement).naturalWidth > 0
        ).length
    );
    const pageCdp = await context.newCDPSession(tab);
    await pageCdp.send("Performance.enable");
    const { metrics } = await pageCdp.send("Performance.getMetrics");
    sampling.abort();
    await sampler;
    record("images", {
      round,
      images,
      bytes,
      loaded,
      heap: metrics.find((metric) => metric.name === "JSHeapUsedSize")?.value,
      processes: [...peaks].map(([pid, seen]) => ({ pid, type: seen.type, start: seen.start, peak: seen.peak })),
    });
  } finally {
    sampling.abort();
    await context.close();
  }
});

/** The VmHWM of the process pid, in KiB; undefined when it is gone or cannot be read. */
function highWater(pid: number): number | undefined {
  try {
    const line = readFileSync(`/proc/${pid}/status`, "utf8")
      .split("\n")
      .find((each) => each.startsWith("VmHWM:"));
    return line === undefined ? undefined : Number(line.split(/\s+/)[1]);
  } catch {
    return undefined;
  }
}
