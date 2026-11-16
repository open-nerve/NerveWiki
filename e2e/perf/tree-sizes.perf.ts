import { createClient } from "@nervewiki/api-client";
import { test } from "@playwright/test";

import { login } from "../fixtures/auth";
import { watchPage } from "../fixtures/browser";
import { goto } from "../fixtures/navigation";
import { painted, probedContext, treeBytes } from "./probe";
import { record } from "./record";
import { perfEnv } from "./setup";

// The page tree at 50,000 and 100,000 pages, flat, only recorded (M12/P4 design 4.9): the tree's bytes, plain and
// compressed; the heap once the home is open; and T1 once, waited for 120 s at most. One round. A tab whose renderer
// crashes on the way is recorded so, with the bytes and how long after the address was asked for.

const round = process.env.NW_PERF_ROUND ?? "1";
/** How long the cold open is waited for: past it, "not painted within 120 s". */
const paintMs = 120_000;

for (const shape of ["flat50k", "flat100k"] as const) {
  test(`the page tree's size, ${shape}`, async ({ browser }) => {
    const env = perfEnv();
    const notebook = env.notebooks[shape];
    test.skip(notebook === undefined, `this round has no ${shape} notebook: NW_PERF_SHAPES names the shapes`);
    if (notebook === undefined) return;
    const api = createClient({ baseUrl: env.baseURL });
    const admin = await login(api, env.admin);
    const bytes = await treeBytes(env.baseURL, admin.access_token, notebook.id);
    const nav = `Pages of ${notebook.notebook}`;
    const context = await probedContext(browser, env.baseURL, admin, { kind: "tree", nav, roots: notebook.roots });
    try {
      const tab = await context.newPage();
      await watchPage(tab);
      const cdp = await context.newCDPSession(tab);
      let crashed: number | undefined;
      const asked = Date.now();
      tab.on("crash", () => {
        crashed = Date.now() - asked;
      });
      await cdp.send("Network.clearBrowserCache");
      await goto(tab, `/${env.workspace}/notebooks/${notebook.id}`);
      const seen = await painted(tab, "t1", paintMs).catch((error: unknown) => {
        if (String(error).includes("Timeout") || crashed !== undefined) return undefined;
        throw error;
      });
      let heap: number | undefined;
      if (crashed === undefined) {
        // The heap as it is then, painted or not.
        await cdp.send("HeapProfiler.collectGarbage");
        await cdp.send("Performance.enable");
        const { metrics } = await cdp.send("Performance.getMetrics");
        heap = metrics.find((metric) => metric.name === "JSHeapUsedSize")?.value;
      }
      record("tree-size", {
        shape,
        round,
        browser: browser.version(),
        ...bytes,
        t1: seen?.painted ?? null,
        paintedWithin:
          crashed === undefined
            ? seen === undefined
              ? `not painted within ${paintMs / 1_000} s`
              : "painted"
            : `the renderer crashed ${crashed} ms after the address was asked for`,
        rows: seen?.rows ?? null,
        heap: heap ?? null,
      });
      if (seen !== undefined) {
        await goto(tab, "about:blank");
      }
      await tab.close();
    } finally {
      await context.close();
    }
  });
}
