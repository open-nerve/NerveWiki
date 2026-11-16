import { setTimeout as sleep } from "node:timers/promises";

import { createClient, type ApiClient, type AuthTokens } from "@nervewiki/api-client";
import { test, type Browser, type Page } from "@playwright/test";

import { bearer, createToken, login } from "../fixtures/auth";
import { postAsset } from "../fixtures/assets";
import { watchPage } from "../fixtures/browser";
import { goto } from "../fixtures/navigation";
import { readContent } from "../fixtures/pages";
import { startEditing } from "../fixtures/wiki-pages";
import { painted, probedContext } from "./probe";
import { record } from "./record";
import { importNotebook, perfEnv, type PerfEnv } from "./setup";
import { pngsOf } from "./shapes";

// M4-performance item 12, the attachment list's rereads, as the hand-off measured it once (M12/P4 design 4.7; the
// judgement of I2-D10). A notebook of 3,000 pages at its root, imported anew for each repetition. Tab A at its home
// (its tree of 3,000 rows, its attachments section reading the root's); tab B of the same sign-in in the editor of
// the first page, typing a character and resting 3 s, so that each rest saves (autosave.ts saves after 2 s); the
// other account's three tokens upload 1,200 PNGs of 1 KiB to the root in 20 s. Recorded: A's reads of the
// attachments and its 429s, its long tasks' share of the window; each of B's saves, its status and when; the 429s of
// both tabs, one credential's. A repetition counts when at least 5 of B's saves fall in the window; a failed save is
// a 429 with no save that succeeded within 10 s after it, the typed text missing from the page once done, or the
// editor showing a failure once its last save answered. Five that count a round; two rounds.

const round = process.env.NW_PERF_ROUND ?? "1";
const counted = 5;
const attempts = 10;
const uploaders = 3;
const uploads = 1_200;
const uploadMs = 20_000;
const restMs = 3_000;

test("the attachment list's rereads, 3,000 pages", async ({ browser }) => {
  const env = perfEnv();
  const api = createClient({ baseUrl: env.baseURL });
  const admin = await login(api, env.admin);
  const other = await login(api, env.other);
  const pat = (await createToken(api, admin.access_token, { name: `uploads ${round}` })).token;
  const tokens: string[] = [];
  for (let i = 1; i <= uploaders; i++) {
    // oxlint-disable-next-line no-await-in-loop -- one token at a time
    tokens.push((await createToken(api, other.access_token, { name: `uploader ${round}-${i}` })).token);
  }
  const files = pngsOf(`uploads ${round}`, uploads);
  const reps: Repetition[] = [];
  let voided = 0;
  for (let attempt = 1; reps.length < counted && attempt <= attempts; attempt++) {
    // oxlint-disable-next-line no-await-in-loop -- one repetition at a time, each on a notebook of its own
    const one = await repetition(browser, env, api, pat, tokens, files, `${round}-${attempt}`);
    record("attachments-rep", { round, attempt, ...one });
    if (one.savesInWindow >= 5) {
      reps.push(one);
    } else {
      voided += 1;
    }
  }
  const failed = reps.filter((one) => one.failed).length;
  record("attachments", {
    round,
    counted: reps.length,
    voided,
    failed,
    stable: failed >= 3,
    saves: reps.map((one) => one.savesInWindow),
    refusedSaves: reps.map((one) => one.refusedSaves),
    aReads: reps.map((one) => one.aReads),
    aRefused: reps.map((one) => one.aRefused),
    aBusy: reps.map((one) => one.aBusy),
  });
});

interface Repetition {
  notebook: string;
  window: { from: number; to: number };
  uploads: Record<string, number>;
  aReads: number;
  aRefused: number;
  bRefused: number;
  /** The share of the window A's main thread spent in long tasks. */
  aBusy: number;
  saves: { sent: number; answered?: number; status?: number; inWindow: boolean }[];
  savesInWindow: number;
  refusedSaves: number;
  typed: string;
  failures: string[];
  failed: boolean;
}

async function repetition(
  browser: Browser,
  env: PerfEnv,
  api: ApiClient,
  pat: string,
  tokens: string[],
  files: Buffer[],
  label: string
): Promise<Repetition> {
  const notebook = await importNotebook(api, pat, env.workspace, "flat3k", `Uploads ${label}`);
  const tree = await api.GET("/api/v0/notebooks/{notebook_id}/nodes", {
    params: { path: { notebook_id: notebook.id } },
    headers: bearer(pat),
  });
  const first = tree.data?.data.find((node) => node.kind === "page" && node.parent_id === null);
  if (first === undefined) throw new Error("no page at the root");
  const nav = `Pages of ${notebook.notebook}`;
  const admin: AuthTokens = await login(api, env.admin);
  const context = await probedContext(browser, env.baseURL, admin, { kind: "tree", nav, roots: notebook.roots });
  try {
    const a = await context.newPage();
    await watchPage(a);
    const aReads: number[] = [];
    const refused: { tab: "A" | "B"; at: number; path: string }[] = [];
    a.on("request", (request) => {
      if (request.method() === "GET" && new URL(request.url()).pathname === `/api/v0/notebooks/${notebook.id}/assets`) {
        aReads.push(Date.now());
      }
    });
    a.on("response", (response) => {
      if (response.status() === 429) refused.push({ tab: "A", at: Date.now(), path: new URL(response.url()).pathname });
    });
    await goto(a, `/${env.workspace}/notebooks/${notebook.id}`);
    await painted(a, "t1", 60_000);

    const b = await context.newPage();
    await watchPage(b);
    const savePath = `/api/v0/pages/${first.id}/content`;
    const saves: { sent: number; answered?: number; status?: number }[] = [];
    const out = new Map<object, (typeof saves)[number]>();
    b.on("request", (request) => {
      if (request.method() === "PUT" && new URL(request.url()).pathname === savePath) {
        const save = { sent: Date.now() };
        saves.push(save);
        out.set(request, save);
      }
    });
    b.on("response", (response) => {
      const save = out.get(response.request());
      if (save !== undefined) {
        save.answered = Date.now();
        save.status = response.status();
      }
      if (response.status() === 429) refused.push({ tab: "B", at: Date.now(), path: new URL(response.url()).pathname });
    });
    await goto(b, `/${env.workspace}/notebooks/${notebook.id}/pages/${first.id}`);
    const content = await startEditing(b);
    await content.press("ControlOrMeta+End");

    // B types from a rest before the uploads to one after them; the uploads run 20 s.
    const from = Date.now() + restMs;
    const typed: string[] = [];
    const typing = (async () => {
      const letters = "abcdefghijklmnopqrstuvwxyz";
      for (let i = 0; Date.now() < from + uploadMs + restMs; i++) {
        const letter = letters[i % letters.length] ?? "a";
        // oxlint-disable-next-line no-await-in-loop -- a character, then a rest
        await b.keyboard.press(letter);
        typed.push(letter);
        // oxlint-disable-next-line no-await-in-loop -- the same
        await sleep(restMs);
      }
    })();
    await sleep(from - Date.now());
    const statuses = await upload(api, tokens, notebook.id, files, from);
    const to = Math.max(Date.now(), from + uploadMs);
    await typing;
    // The last rest's save, answered; then 10 s more, past a 429's retry window.
    await waitForSaves(b, saves);
    await sleep(10_000);
    const failing = await b.getByRole("main").locator("output.text-destructive").count();
    const page = await readContent(api, pat, first.id);
    const longTasks = await a.evaluate(() => ({ origin: performance.timeOrigin, tasks: window.nwPerf.longTasks() }));
    const busy = longTasks.tasks
      .map((task) => ({ start: longTasks.origin + task.start, end: longTasks.origin + task.start + task.duration }))
      .map((task) => Math.max(0, Math.min(task.end, to) - Math.max(task.start, from)))
      .reduce((sum, ms) => sum + ms, 0);

    const failures: string[] = [];
    for (const save of saves) {
      if (save.status !== 429 || save.answered === undefined) continue;
      const at = save.answered;
      const later = saves.some(
        (each) =>
          each.status !== undefined &&
          each.status < 300 &&
          each.answered !== undefined &&
          each.answered > at &&
          each.answered <= at + 10_000
      );
      if (!later) failures.push(`a 429 at ${at - from} ms with no save that succeeded within 10 s`);
    }
    if (!page.content.includes(typed.join(""))) failures.push("the page lacks the typed text");
    if (failing > 0) failures.push("the editor shows a failure after its last save");
    const inWindow = (save: { sent: number }) => save.sent >= from && save.sent <= to;
    return {
      notebook: notebook.id,
      window: { from: 0, to: to - from },
      uploads: statuses,
      aReads: aReads.filter((at) => at >= from && at <= to).length,
      aRefused: refused.filter((each) => each.tab === "A" && each.at >= from && each.at <= to).length,
      bRefused: refused.filter((each) => each.tab === "B" && each.at >= from).length,
      aBusy: busy / (to - from),
      saves: saves.map((save) => ({
        sent: save.sent - from,
        answered: save.answered === undefined ? undefined : save.answered - from,
        status: save.status,
        inWindow: inWindow(save),
      })),
      savesInWindow: saves.filter(inWindow).length,
      refusedSaves: saves.filter((save) => save.status === 429).length,
      typed: typed.join(""),
      failures,
      failed: failures.length > 0,
    };
  } finally {
    await context.close();
  }
}

/**
 * The uploads: files.length of them, the tokens taking turns, sent evenly from from over uploadMs, none waiting for
 * another; their statuses counted once all answered.
 */
async function upload(
  api: ApiClient,
  tokens: string[],
  notebookId: string,
  files: Buffer[],
  from: number
): Promise<Record<string, number>> {
  const sent: Promise<number>[] = [];
  const every = uploadMs / files.length;
  for (const [i, bytes] of files.entries()) {
    const wait = from + i * every - Date.now();
    // oxlint-disable-next-line no-await-in-loop -- paced, not waited for
    if (wait > 0) await sleep(wait);
    const token = tokens[i % tokens.length] ?? "";
    sent.push(
      postAsset(api, token, notebookId, {
        name: `upload-${i}.png`,
        bytes: new Uint8Array(bytes),
        type: "image/png",
      }).then(
        ({ response }) => response.status,
        () => 0
      )
    );
  }
  const statuses: Record<string, number> = {};
  for (const status of await Promise.all(sent)) statuses[status] = (statuses[status] ?? 0) + 1;
  return statuses;
}

/** Waits until B has no save out and has rested past its autosave: every save sent has its answer. */
async function waitForSaves(page: Page, saves: { answered?: number }[]): Promise<void> {
  await sleep(2_500);
  const deadline = Date.now() + 30_000;
  while (saves.some((save) => save.answered === undefined) && Date.now() < deadline) {
    // oxlint-disable-next-line no-await-in-loop -- polled
    await sleep(100);
  }
  await page.waitForTimeout(0);
}
