import { setTimeout as sleep } from "node:timers/promises";

import { createClient } from "@nervewiki/api-client";
import { expect, test, type Page } from "@playwright/test";

import { bearer, login, signInContext } from "../fixtures/auth";
import { watchPage } from "../fixtures/browser";
import { goto } from "../fixtures/navigation";
import { createPage } from "../fixtures/pages";
import { editorContent } from "../fixtures/wiki-pages";
import { median, record } from "./record";
import { perfEnv } from "./setup";

// M4-performance item 7, only recorded (M12/P4 design 4.5): what a link's completion reads, the notebook's link
// targets, on the flat shapes of 10,000 and 100,000 pages. From Node, the read's bytes and time, five times; in the
// browser, in a page's editor, from the keydown of the second "[" to the frame after next of the completion's list,
// five times, each on a line of its own, so that each "[[" reads the targets again (a read serves its "[[" for 10 s,
// W/editor/loaded/link-completion.ts). Two rounds.

const round = process.env.NW_PERF_ROUND ?? "1";
const times = 5;
/** How long the editor may take to open on a page whose notebook's tree is this large. */
const openMs = 180_000;

for (const shape of ["flat", "flat100k"] as const) {
  test(`link targets, ${shape}`, async ({ browser }) => {
    const env = perfEnv();
    const notebook = env.notebooks[shape];
    test.skip(notebook === undefined, `this round has no ${shape} notebook: NW_PERF_SHAPES names the shapes`);
    if (notebook === undefined) return;
    const api = createClient({ baseUrl: env.baseURL });
    const admin = await login(api, env.admin);
    const reads: { ms: number; bytes: number }[] = [];
    for (let i = 1; i <= times; i++) {
      const started = performance.now();
      // oxlint-disable-next-line no-await-in-loop -- one read at a time
      const answer = await fetch(new URL(`/api/v0/notebooks/${notebook.id}/link-targets`, env.baseURL), {
        headers: bearer(admin.access_token),
      });
      // oxlint-disable-next-line no-await-in-loop -- the same
      const bytes = (await answer.arrayBuffer()).byteLength;
      if (answer.status !== 200) throw new Error(`link targets: ${answer.status}`);
      reads.push({ ms: performance.now() - started, bytes });
    }
    record("link-targets", { shape, round, reads, median: median(reads.map((read) => read.ms)) });

    const created = await createPage(api, admin.access_token, notebook.id, `Links ${round}`, null, "Links.\n");
    const context = await browser.newContext({ baseURL: env.baseURL });
    try {
      await signInContext(context, env.baseURL, admin);
      const page = await context.newPage();
      await watchPage(page);
      const opened = performance.now();
      let completions: { ms: number; options: number }[] = [];
      let note = "";
      try {
        await goto(page, `/${env.workspace}/notebooks/${notebook.id}/pages/${created.id}`, { timeout: openMs });
        // startEditing's, with the time a large tree takes the page.
        await page.getByRole("main").getByRole("button", { name: "Edit", exact: true }).click({ timeout: openMs });
        const content = editorContent(page);
        await expect(content).toBeFocused({ timeout: openMs });
        note = `the editor opened ${Math.round(performance.now() - opened)} ms after the page was asked for`;
        await content.press("ControlOrMeta+End");
        completions = [];
        for (let i = 1; i <= times; i++) {
          // oxlint-disable-next-line no-await-in-loop -- one completion at a time
          completions.push(await completeOnce(page));
        }
      } catch (error) {
        note = `not measured: ${String(error).split("\n")[0]}`;
      }
      record("link-completion", {
        shape,
        round,
        note,
        completions,
        median: completions.length > 0 ? median(completions.map((completion) => completion.ms)) : null,
      });
    } finally {
      await context.close();
    }
  });
}

/**
 * One completion: a new line, "[" typed, a second's rest, then the second "[": from its keydown to two frames after
 * the completion's list first has an option. Escape closes it.
 */
async function completeOnce(page: Page): Promise<{ ms: number; options: number }> {
  await page.keyboard.press("Enter");
  await page.keyboard.type("see [");
  await sleep(1_000);
  await page.evaluate(() => {
    const w = window as unknown as { nwCompletion?: { ms: number; options: number } };
    w.nwCompletion = undefined;
    addEventListener(
      "keydown",
      (event) => {
        const at = event.timeStamp;
        // oxlint-disable-next-line unicorn/consistent-function-scoping -- serialised whole: it stays in the listener
        const options = () => document.querySelectorAll(".cm-tooltip-autocomplete li").length;
        const painted = () =>
          requestAnimationFrame(() =>
            requestAnimationFrame(() => {
              w.nwCompletion = { ms: performance.now() - at, options: options() };
            })
          );
        if (options() > 0) {
          painted();
          return;
        }
        const watcher = new MutationObserver(() => {
          if (options() > 0) {
            watcher.disconnect();
            painted();
          }
        });
        watcher.observe(document.body, { subtree: true, childList: true });
      },
      { capture: true, once: true }
    );
  });
  await page.keyboard.press("[");
  await page.waitForFunction(
    () => (window as unknown as { nwCompletion?: unknown }).nwCompletion !== undefined,
    undefined,
    { timeout: 60_000 }
  );
  const result = await page.evaluate(
    () => (window as unknown as { nwCompletion: { ms: number; options: number } }).nwCompletion
  );
  await page.keyboard.press("Escape");
  return result;
}
