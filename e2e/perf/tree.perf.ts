import { setTimeout as sleep } from "node:timers/promises";

import { createClient, type ApiClient } from "@nervewiki/api-client";
import { expect, test, type BrowserContext, type Page, type Request } from "@playwright/test";

import { login } from "../fixtures/auth";
import { watchPage } from "../fixtures/browser";
import { goto } from "../fixtures/navigation";
import { postPage } from "../fixtures/pages";
import { coldOpen, painted, probedContext, treeBytes, watchFor, type ColdOpen, type LongTask } from "./probe";
import { median, record } from "./record";
import { perfEnv, type PerfEnv } from "./setup";
import type { ShapeName } from "./shapes";

// The page tree's three bounds, measured on each of its three shapes (M12/P4 design 4.3; 00-M12 4.7.2, decision 329):
// T1, a cold open of the notebook's home, ≤ 1 s; T2, another account's new page painted first at the root, ≤ 1.5 s
// from its answer, its longest long task ≤ 200 ms; T3, the folder of 1,000 pages opened, ≤ 500 ms. Five of each a
// round; a round is one run, NW_PERF_ROUND naming it. Besides: the tree's bytes, the heap after the fifth T1, and in
// the deep shape the "Move to…" dialog of the big folder. NW_PERF_CONTROLS=1 runs the controls of the measures
// instead (design 4.3, the six of them).
//
// T3 comes before T2, in the tab as it opened: "000 big" the first root, its children laid out from it down, some in
// the viewport. After T2 it was the sixth, its children all below the viewport, and T3 measured a paint of none in it
// (review B's M1 on M12/P4/S2). Scrolling it up instead would turn on how far the page scrolls, the folder closed.

const round = process.env.NW_PERF_ROUND ?? "1";
const times = 5;
/** How long a measure waits for its paint: far past every bound, short of the test's timeout. */
const paintMs = 60_000;

/** What a shape's test needs: the round's preparation, the notebook, a client, and both accounts' tokens. */
async function shapeOf(name: ShapeName) {
  const env = perfEnv();
  const notebook = env.notebooks[name];
  test.skip(notebook === undefined, `this round has no ${name} notebook: NW_PERF_SHAPES names the shapes`);
  if (notebook === undefined) throw new Error("skipped");
  const api = createClient({ baseUrl: env.baseURL });
  const [admin, other] = await Promise.all([login(api, env.admin), login(api, env.other)]);
  const nav = `Pages of ${notebook.notebook}`;
  return { env, notebook, api, admin, other, nav, home: `/${env.workspace}/notebooks/${notebook.id}` };
}

for (const shape of ["flat", "deep", "attachments"] as const) {
  test(`the page tree, ${shape}`, async ({ browser }) => {
    const { env, notebook, api, admin, other, nav, home } = await shapeOf(shape);
    const bytes = await treeBytes(env.baseURL, admin.access_token, notebook.id);
    record("tree-bytes", { shape, round, browser: browser.version(), ...bytes });
    const context = await probedContext(browser, env.baseURL, admin, { kind: "tree", nav, roots: notebook.roots });
    try {
      const t1: ColdOpen[] = [];
      for (let i = 1; i <= times; i++) {
        // oxlint-disable-next-line no-await-in-loop -- one cold open at a time
        const open = await coldOpen(context, home, { heap: i === times });
        // The cache was cleared: the main script came over the network (control 5).
        expect(open.mainJs, "the main script's bytes over the network").toBeGreaterThan(0);
        // The end painted every root, or rows past the viewport's bottom edge: one commit renders the whole list.
        expect(open.rows, "rows at the end").toBeGreaterThanOrEqual(Math.min(notebook.roots, open.inView + 1));
        record("t1", { shape, round, i, ...open });
        t1.push(open);
      }
      if (shape === "deep") {
        record("t1-profile", { shape, round, ...(await profiledOpen(context, env, home)) });
      }
      const tab = await openTab(context, home);
      const t3: number[] = [];
      const dialog: number[] = [];
      if (notebook.big !== undefined) {
        for (let i = 1; i <= times; i++) {
          // oxlint-disable-next-line no-await-in-loop -- one opening at a time
          const one = await openFolder(tab, nav, notebook.big, `t3-${i}`);
          record("t3", { shape, round, i, ...one });
          t3.push(one.t3);
        }
      }
      const scrolled = await firstRowInView(tab.page, nav);
      record("t2-scroll", { shape, round, ...scrolled });
      const t2: T2[] = [];
      for (let i = 1; i <= times; i++) {
        // oxlint-disable-next-line no-await-in-loop -- one change of the tree at a time
        const one = await changeOnce(tab, api, other.access_token, notebook.id, nav, `000 T2 ${round}-${i}`, `t2-${i}`);
        record("t2", { shape, round, i, ...one });
        t2.push(one);
        // oxlint-disable-next-line no-await-in-loop -- the refresher's last read lands first
        await sleep(3_000);
      }
      const voided = t2.filter((one) => !one.intersects).length;
      expect(voided, "T2s whose new row was not in the viewport").toBeLessThanOrEqual(times / 2);
      if (shape === "deep" && notebook.big !== undefined) {
        for (let i = 1; i <= times; i++) {
          // oxlint-disable-next-line no-await-in-loop -- one dialog at a time
          const ms = await moveDialog(tab, nav, notebook.big, `move-${i}`);
          record("move-dialog", { shape, round, i, ms });
          dialog.push(ms);
        }
      }
      const valid = t2.filter((one) => one.intersects);
      record("tree", {
        shape,
        round,
        t1: summary(t1.map((open) => open.t1)),
        t1Longest: summary(t1.map((open) => open.longest)),
        heap: t1.at(-1)?.heap,
        homeItems: t1.at(-1)?.homeItems,
        items: t1.at(-1)?.items,
        t2: summary(valid.map((one) => one.t2)),
        t2Longest: summary(valid.map((one) => one.longest)),
        t2Voided: voided,
        ...(t3.length > 0 ? { t3: summary(t3) } : {}),
        ...(dialog.length > 0 ? { moveDialog: summary(dialog) } : {}),
      });
    } finally {
      await context.close();
    }
  });
}

test("the page tree's measures see what they measure", async ({ browser }) => {
  test.skip(process.env.NW_PERF_CONTROLS !== "1", "the controls run with NW_PERF_CONTROLS=1");
  const shape = (process.env.NW_PERF_CONTROL_SHAPE ?? "deep") as ShapeName;
  const { env, notebook, api, admin, other, nav, home } = await shapeOf(shape);
  const context = await probedContext(browser, env.baseURL, admin, { kind: "tree", nav, roots: notebook.roots });
  try {
    const plain: number[] = [];
    for (let i = 1; i <= 3; i++) {
      // oxlint-disable-next-line no-await-in-loop -- one cold open at a time
      plain.push((await coldOpen(context, home)).t1);
    }
    const base = median(plain);
    // 1: the zero is the document's start, before the main script: holding it back 1 s adds 1 s.
    const script = await coldOpen(context, home, { before: (tab) => holdBack(tab, "**/assets/index-*.js", 1_000) });
    // 2: the end waits for the tree's answer: holding it back 1 s adds 1 s.
    const tree = await coldOpen(context, home, {
      before: (tab) => holdBack(tab, "**/api/v0/notebooks/*/nodes", 1_000),
    });
    // 5: without clearing the cache, the main script comes from it: the measure's assertion would fail.
    const cached = await coldOpen(context, home, { clear: false });
    record("control-t1", { shape, round, plain, script: script.t1, tree: tree.t1, cachedMainJs: cached.mainJs });
    expect.soft(script.t1 - base, "T1 with the main script held back 1 s, less T1").toBeGreaterThanOrEqual(850);
    expect.soft(script.t1 - base).toBeLessThanOrEqual(1_150);
    expect.soft(tree.t1 - base, "T1 with the tree's answer held back 1 s, less T1").toBeGreaterThanOrEqual(850);
    expect.soft(tree.t1 - base).toBeLessThanOrEqual(1_150);
    expect.soft(cached.mainJs, "the main script's bytes over the network, from the cache").toBe(0);

    const tab = await openTab(context, home);
    await firstRowInView(tab.page, nav);
    const t2: number[] = [];
    for (let i = 1; i <= times; i++) {
      // oxlint-disable-next-line no-await-in-loop -- one change at a time
      t2.push((await changeOnce(tab, api, other.access_token, notebook.id, nav, `000 C2 ${round}-${i}`, `c2-${i}`)).t2);
      // oxlint-disable-next-line no-await-in-loop -- the refresher's last read lands first
      await sleep(3_000);
    }
    // 4: the tolerance is 0.1 s, or the five's range and 0.05 s when the range is over 0.1 s.
    const range = Math.max(...t2) - Math.min(...t2);
    const tolerance = range > 100 ? range + 50 : 100;
    // 3: a task of 300 ms in the page after the answer is the longest long task.
    const busy = await changeOnce(tab, api, other.access_token, notebook.id, nav, `000 C2 ${round}-busy`, "c2-busy", {
      busy: true,
    });
    await sleep(3_000);
    // 4: this tab's reads of the tree held back 0.5 s add 0.5 s.
    await holdBack(tab.page, "**/api/v0/notebooks/*/nodes", 500);
    const held = await changeOnce(tab, api, other.access_token, notebook.id, nav, `000 C2 ${round}-held`, "c2-held");
    record("control-t2", { shape, round, t2, range, tolerance, busy: busy.longest, held: held.t2 });
    expect.soft(busy.longest, "the longest long task with a task of 300 ms").toBeGreaterThanOrEqual(300);
    expect
      .soft(Math.abs(held.t2 - median(t2) - 500), "T2 with the tree held back 0.5 s, less 0.5 s")
      .toBeLessThanOrEqual(tolerance);

    // 6: the children of a folder not opened are never painted.
    const closed = notebook.big === undefined ? undefined : "r01";
    if (closed !== undefined) {
      await watchFor(tab.page, "closed", { kind: "children", nav, button: `Subpages of ${closed}`, count: 1_000 });
      await sleep(5_000);
      const seen = await tab.page.evaluate(() => window.nwPerf.seen("closed"));
      record("control-t3", { shape, round, seen: seen ?? null });
      expect.soft(seen, "children of a closed folder seen").toBeUndefined();
    }
  } finally {
    await context.close();
  }
});

/** Holds back each answer to what for ms. The route stays: taking a page's last one away drops requests out. */
async function holdBack(page: Page, what: string, ms: number): Promise<void> {
  await page.route(what, async (route) => {
    await sleep(ms);
    await route.fallback();
  });
}

/** A tab of the context at home, painted, with its API requests out followed (the event stream's aside). */
async function openTab(context: BrowserContext, home: string): Promise<{ page: Page; out: Set<Request> }> {
  const page = await context.newPage();
  await watchPage(page);
  const out = new Set<Request>();
  page.on("request", (request) => {
    const path = new URL(request.url()).pathname;
    if (path.startsWith("/api/") && path !== "/api/v0/events") out.add(request);
  });
  page.on("requestfinished", (request) => out.delete(request));
  page.on("requestfailed", (request) => out.delete(request));
  await goto(page, home);
  await painted(page, "t1", paintMs);
  return { page, out };
}

/** Waits until the tab has had no API request out and no long task for ms. */
async function quietFor(tab: { page: Page; out: Set<Request> }, ms: number): Promise<void> {
  const deadline = Date.now() + 30_000;
  let idleSince = Date.now();
  for (;;) {
    if (tab.out.size > 0) idleSince = Date.now();
    // oxlint-disable-next-line no-await-in-loop -- looked at until quiet
    const { now, lastEnd } = await tab.page.evaluate(() => {
      const last = window.nwPerf.longTasks().at(-1);
      return { now: performance.now(), lastEnd: last === undefined ? 0 : last.start + last.duration };
    });
    if (tab.out.size === 0 && Date.now() - idleSince >= ms && now - lastEnd >= ms) return;
    if (Date.now() > deadline) throw new Error(`the tab was not quiet for ${ms} ms within 30 s`);
    // oxlint-disable-next-line no-await-in-loop -- the same
    await sleep(100);
  }
}

/**
 * Scrolls the window, when it must, until the first row of the tree is in the viewport: the left column scrolls with
 * the page (workspace-layout.tsx). Answers where the row was and whether it scrolled.
 */
async function firstRowInView(page: Page, nav: string) {
  return page.evaluate((label) => {
    const tree = [...document.querySelectorAll("nav")].find((each) => each.getAttribute("aria-label") === label);
    const row = tree?.querySelector(":scope > ul > li:first-child > :first-child");
    if (!row) throw new Error(`no first row in ${label}`);
    const box = row.getBoundingClientRect();
    const scroll = box.top < 0 || box.bottom > innerHeight;
    if (scroll) row.scrollIntoView({ block: "center" });
    return { top: box.top, bottom: box.bottom, innerHeight, scrolled: scroll, scrollY };
  }, nav);
}

/** One T2: from the answer, on Node's clock, to the tab's paint, on the page's, both since the epoch. */
interface T2 {
  t2: number;
  met: number;
  longest: number;
  intersects: boolean;
}

/**
 * One T2 (design 4.3): once the tab is quiet for 1 s, the other account creates name first at the root
 * (after_id: null); T2 runs from the answer's arrival to the paint of the tab's first root row named name; the
 * longest long task is of those that started between. busy puts a task of 300 ms in the page after the answer
 * (control 3).
 */
async function changeOnce(
  tab: { page: Page; out: Set<Request> },
  api: ApiClient,
  credential: string,
  notebookId: string,
  nav: string,
  name: string,
  key: string,
  { busy = false } = {}
): Promise<T2> {
  await quietFor(tab, 1_000);
  await watchFor(tab.page, key, { kind: "first", nav, name });
  const { response } = await postPage(api, credential, notebookId, { parent_id: null, title: name, after_id: null });
  const answered = performance.timeOrigin + performance.now();
  expect(response.status, `create ${name}`).toBe(201);
  if (busy) {
    await tab.page.evaluate(() => {
      setTimeout(() => {
        const end = performance.now() + 300;
        while (performance.now() < end) {
          // busy
        }
      });
    });
  }
  const seen = await painted(tab.page, key, paintMs);
  const page = await tab.page.evaluate(() => ({ origin: performance.timeOrigin, tasks: window.nwPerf.longTasks() }));
  const from = answered - page.origin;
  return {
    t2: page.origin + seen.painted - answered,
    met: page.origin + seen.met - answered,
    longest: longestOf(page.tasks, from, seen.painted),
    intersects: seen.intersects,
  };
}

function longestOf(tasks: LongTask[], from: number, to: number): number {
  return Math.max(0, ...tasks.filter((task) => task.start >= from && task.start <= to).map((task) => task.duration));
}

/**
 * One T3 (design 4.3): once the tab is quiet for 1 s, a click on the button that opens big; T3 runs from the click
 * event's timeStamp to the paint of its children down to the viewport's bottom edge, at least one of them in it. Then
 * it closes again, and the tab rests a second.
 */
async function openFolder(tab: { page: Page; out: Set<Request> }, nav: string, big: string, key: string) {
  await quietFor(tab, 1_000);
  const label = `Subpages of ${big}`;
  const button = tab.page
    .getByRole("navigation", { name: nav, exact: true })
    .getByRole("button", { name: label, exact: true });
  await watchFor(tab.page, key, { kind: "children", nav, button: label, count: 1_000 });
  await button.click();
  const seen = await painted(tab.page, key, paintMs);
  const click = await tab.page.evaluate(() => window.nwPerf.lastClick());
  expect(click?.label, "the click's button").toBe(label);
  // Children painted in the viewport: with none in it, the end holds once the list is there, whatever it paints.
  expect(seen.inView, "the children in the viewport").toBeGreaterThanOrEqual(1);
  await button.click();
  await expect(button).toHaveAttribute("aria-expanded", "false");
  await sleep(1_000);
  const at = click?.timeStamp ?? Number.NaN;
  return { t3: seen.painted - at, met: seen.met - at, rows: seen.rows, inView: seen.inView };
}

/** The time from the click on "Move to…" in big's menu to the paint of the dialog's parent select with its options. */
async function moveDialog(tab: { page: Page; out: Set<Request> }, nav: string, big: string, key: string) {
  await quietFor(tab, 1_000);
  await tab.page
    .getByRole("navigation", { name: nav, exact: true })
    .getByRole("button", { name: `Actions for ${big}`, exact: true })
    .click();
  const item = tab.page.getByRole("menuitem", { name: "Move to…", exact: true });
  await item.waitFor();
  await watchFor(tab.page, key, { kind: "dialog", title: `Move ${big}` });
  await item.click();
  const seen = await painted(tab.page, key, paintMs);
  const click = await tab.page.evaluate(() => window.nwPerf.lastClick());
  await tab.page.keyboard.press("Escape");
  await expect(tab.page.getByRole("dialog")).toHaveCount(0);
  await sleep(1_000);
  return seen.painted - (click?.timeStamp ?? Number.NaN);
}

/**
 * A cold open of home profiled (Profiler, sampled every 100 µs), not one of the five: the self time of subtreeOf,
 * which each row's menu calls (page-tree-item.tsx), found in the built script by its body, its name being minified.
 * The tab first loads /healthz, of the same origin, so that the profiler started there keeps running in the same
 * renderer as the home loads.
 */
async function profiledOpen(context: BrowserContext, env: PerfEnv, home: string) {
  let found: { name?: string; url?: string; selfMs: number; samples: number } = { selfMs: 0, samples: 0 };
  const open = await coldOpen(context, home, {
    before: async (tab, cdp) => {
      await goto(tab, new URL("/healthz", env.baseURL).toString());
      await cdp.send("Profiler.enable");
      await cdp.send("Profiler.setSamplingInterval", { interval: 100 });
      await cdp.send("Profiler.start");
    },
    after: async (tab, cdp) => {
      const { profile } = await cdp.send("Profiler.stop");
      const scripts = await tab.evaluate(() =>
        (performance.getEntriesByType("resource") as PerformanceResourceTiming[])
          .map((entry) => entry.name)
          .filter((name) => new URL(name).pathname.endsWith(".js"))
      );
      const where = await minifiedName(scripts);
      found = { ...where, ...selfTime(profile, where) };
    },
  });
  return { t1: open.t1, subtreeOf: found };
}

/**
 * The name subtreeOf (W/stores/page-tree.ts) has in the built scripts, and its script: the function that pushes the
 * children of each page it collected while their count stays within tree.byId.size. Undefined when none or more
 * than one matches.
 */
async function minifiedName(scripts: string[]): Promise<{ name: string; url: string } | undefined> {
  const found = new Map<string, { name: string; url: string }>();
  for (const script of scripts) {
    // oxlint-disable-next-line no-await-in-loop -- a few scripts
    const text = await (await fetch(script)).text();
    for (const match of text.matchAll(/function ([\w$]+)\(([\w$]+),([\w$]+)\)\{[^{}]*?\2\.byId\.get\(\3\)/g)) {
      // Its own body: up to the next function.
      const next = text.indexOf("function ", match.index + 1);
      const body = text.slice(match.index, next === -1 ? undefined : next);
      if (/\.length<=[\w$]+\.byId\.size/.test(body)) {
        const name = match[1] ?? "";
        found.set(`${script} ${name}`, { name, url: script });
      }
    }
  }
  return found.size === 1 ? [...found.values()][0] : undefined;
}

/** The self time in profile of the function name of the script url, in ms, and its samples. */
function selfTime(
  profile: {
    nodes: { id: number; callFrame: { functionName: string; url: string } }[];
    samples?: number[];
    timeDeltas?: number[];
  },
  where: { name: string; url: string } | undefined
) {
  const ids = new Set(
    profile.nodes
      .filter((node) => node.callFrame.functionName === where?.name && node.callFrame.url === where.url)
      .map((node) => node.id)
  );
  let us = 0;
  let samples = 0;
  for (const [i, id] of (profile.samples ?? []).entries()) {
    if (ids.has(id)) {
      // A sample's time is the delta to the next one.
      us += profile.timeDeltas?.[i + 1] ?? 0;
      samples += 1;
    }
  }
  return { selfMs: us / 1_000, samples };
}

function summary(values: number[]) {
  return { values, median: values.length > 0 ? median(values) : null };
}
