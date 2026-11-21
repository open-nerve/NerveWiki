import { setTimeout as sleep } from "node:timers/promises";

import { createClient, type ApiClient } from "@nervewiki/api-client";
import { expect, test, type Browser } from "@playwright/test";

import { bearer, createToken, login, signInContext } from "../fixtures/auth";
import { watchPage } from "../fixtures/browser";
import { goto } from "../fixtures/navigation";
import { notebookPath } from "../fixtures/notebook-pages";
import { createNotebook } from "../fixtures/notebooks";
import { createPage } from "../fixtures/pages";
import { cancelJob, startExport, startImport } from "../fixtures/transfer";
import { record } from "./record";
import { perfEnv, rootPageId, type PerfEnv } from "./setup";
import { sha256, shapes, type Archive } from "./shapes";

// M7-transfer item 4, only recorded (M12/P4 design 4.8): the notebook's jobs, read again every second while one is
// queued or running (W/pages/notebook/transfer-page.tsx), every page of them held. For each N, 1 and 3 tabs, a
// notebook of its own: a page, 60 small exports of it ended (with the import, two pages of 50 jobs), then an import of
// 50,000 pages under one folder, running through the window. Since M12/P4/S6 an import makes 45,000 nodes at most:
// the folder and its first 5,000 pages are imported before the exports, and the window's import is the last 45,000
// under it, the same tree. Not the export of 50,000 pages the design names: here it
// ends within a second, and their tree, in each tab's column, holds its main thread 19 s; the import runs minutes and
// leaves the root one row. N tabs of one sign-in at the transfer page, each having loaded the second page; for 60
// seconds each tab's reads of the list, its other requests and its 429s. Two rounds.

const round = process.env.NW_PERF_ROUND ?? "1";
const windowMs = 60_000;
const small = 60;

test("the jobs' polling", async ({ browser }) => {
  const env = perfEnv();
  const api = createClient({ baseUrl: env.baseURL });
  const admin = await login(api, env.admin);
  // A token of the test's own for its calls: the tabs' sign-in is another credential, its bucket theirs alone.
  const pat = (await createToken(api, admin.access_token, { name: `jobs ${round}` })).token;
  const archives = shapes.folder50k.archives();
  const [folder, archive] = archives;
  if (folder === undefined || archive?.under !== "Folder" || archives.length !== 2)
    throw new Error("not the folder's two");
  // Their bytes and sha256, as setup.ts records each shape's (review A's N2 on M12/P4/S2), as "archive": no import has
  // run yet, each N imports them anew, and an "import" line has the time and state of one (its check C1-N1).
  for (const [i, each] of archives.entries()) {
    record("archive", {
      shape: "folder50k",
      archive: i,
      under: each.under ?? null,
      bytes: each.zip.length,
      sha256: sha256(each.zip),
    });
  }
  for (const tabs of [1, 3]) {
    // oxlint-disable-next-line no-await-in-loop -- one N at a time, each with its own notebook and import
    record("jobs-poll", { round, tabs, ...(await polling(browser, env, api, pat, folder, archive, tabs)) });
  }
});

/** The job id once done says its state does, polled every half second within ms. */
async function stateOf(
  api: ApiClient,
  credential: string,
  id: string,
  done: (state: string) => boolean,
  ms: number
): Promise<{ state: string; at: number }> {
  const deadline = Date.now() + ms;
  for (;;) {
    // oxlint-disable-next-line no-await-in-loop -- polled
    const { data } = await api.GET("/api/v0/transfer-jobs/{job_id}", {
      params: { path: { job_id: id } },
      headers: bearer(credential),
    });
    if (data !== undefined && done(data.state)) return { state: data.state, at: Date.now() };
    if (Date.now() > deadline) throw new Error(`the job ${id} is still ${data?.state} after ${ms} ms`);
    // oxlint-disable-next-line no-await-in-loop -- the same
    await sleep(500);
  }
}

const ended = (state: string) => state !== "queued" && state !== "running";

interface Tab {
  reads: number[];
  requests: { at: number; path: string }[];
  refused: { at: number; path: string }[];
}

/**
 * One N: its notebook and jobs, its tabs, its window; the import cancelled and ended after it. The folder's first
 * archive is imported, and ended, before the small exports; the window's import is archive, under the folder.
 */
async function polling(
  browser: Browser,
  env: PerfEnv,
  api: ApiClient,
  pat: string,
  folder: Archive,
  archive: Archive,
  tabs: number
) {
  const notebook = await createNotebook(api, pat, env.workspace, `Jobs ${round}-${tabs}`, "editor");
  const folderStarted = Date.now();
  const first = await startImport(api, pat, notebook.id, new Uint8Array(folder.zip));
  const folderState = (await stateOf(api, pat, first.id, ended, 600_000)).state;
  if (folderState !== "succeeded") throw new Error(`the folder's import ended ${folderState}`);
  const folderMs = Date.now() - folderStarted;
  const parent = await rootPageId(api, pat, notebook.id, archive.under ?? "");
  const exported = await createPage(api, pat, notebook.id, "Exported", null, "Exported.\n");
  const smallStarted = Date.now();
  for (let i = 1; i <= small; i++) {
    // oxlint-disable-next-line no-await-in-loop -- one at a time: the jobs queued are bounded (transfer.max_queued)
    const job = await startExport(api, pat, notebook.id, exported.id);
    // oxlint-disable-next-line no-await-in-loop -- the same
    await stateOf(api, pat, job.id, ended, 60_000);
  }
  const smallMs = Date.now() - smallStarted;
  const job = await startImport(api, pat, notebook.id, new Uint8Array(archive.zip), parent);
  const running = await stateOf(api, pat, job.id, (state) => state !== "queued", 120_000);
  const listPath = `/api/v0/notebooks/${notebook.id}/transfer-jobs`;
  const context = await browser.newContext({ baseURL: env.baseURL });
  try {
    await signInContext(context, env.baseURL, await login(api, env.admin));
    const seen: Tab[] = [];
    for (let i = 0; i < tabs; i++) {
      // oxlint-disable-next-line no-await-in-loop -- one tab after another
      const page = await context.newPage();
      // oxlint-disable-next-line no-await-in-loop -- the same
      await watchPage(page);
      const tab: Tab = { reads: [], requests: [], refused: [] };
      page.on("request", (request) => {
        const path = new URL(request.url()).pathname;
        if (!path.startsWith("/api/")) return;
        tab.requests.push({ at: Date.now(), path });
        if (path === listPath) tab.reads.push(Date.now());
      });
      page.on("response", (response) => {
        if (response.status() === 429) tab.refused.push({ at: Date.now(), path: new URL(response.url()).pathname });
      });
      // oxlint-disable-next-line no-await-in-loop -- the same
      await goto(page, notebookPath(env.workspace, notebook.id, "transfer"));
      const more = page.getByRole("button", { name: "Load more", exact: true });
      // oxlint-disable-next-line no-await-in-loop -- the same
      await more.click();
      // oxlint-disable-next-line no-await-in-loop -- the same
      await expect(more).toHaveCount(0);
      seen.push(tab);
    }
    const from = Date.now();
    await sleep(windowMs);
    const to = Date.now();
    const { data } = await api.GET("/api/v0/transfer-jobs/{job_id}", {
      params: { path: { job_id: job.id } },
      headers: bearer(pat),
    });
    const finished = data?.finished_at;
    const until = finished === null || finished === undefined ? to : Math.min(to, Date.parse(finished));
    const inWindow = (at: number) => at >= from && at <= until;
    const result = {
      folderMs,
      smallMs,
      importState: data?.state,
      importProgress: data?.progress,
      /** From the import's running to the window's start: the tabs' opening. */
      runningBefore: from - running.at,
      windowMs: until - from,
      perTab: seen.map((tab) => {
        const reads = tab.reads.filter(inWindow).length;
        const requests = tab.requests.filter((each) => inWindow(each.at));
        const byPath: Record<string, number> = {};
        for (const each of requests) {
          const key = each.path.replaceAll(/[0-9a-f]{8}-[0-9a-f-]{27}/g, "{id}");
          byPath[key] = (byPath[key] ?? 0) + 1;
        }
        return {
          reads,
          readsPerMinute: Math.round((reads * 60_000) / (until - from)),
          requests: requests.length,
          byPath,
          refused: tab.refused.filter((each) => inWindow(each.at)).length,
          refusedPaths: [...new Set(tab.refused.map((each) => each.path))],
        };
      }),
    };
    if (data !== undefined && !ended(data.state)) {
      await cancelJob(api, pat, job.id);
      // The next N's import is not queued behind this one.
      await stateOf(api, pat, job.id, ended, 180_000);
    }
    return result;
  } finally {
    await context.close();
  }
}
