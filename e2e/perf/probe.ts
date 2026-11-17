import { constants, gzipSync, zstdCompressSync } from "node:zlib";

import type { AuthTokens } from "@nervewiki/api-client";
import type { Browser, BrowserContext, CDPSession, Page } from "@playwright/test";

import { bearer, signInContext } from "../fixtures/auth";
import { watchPage } from "../fixtures/browser";
import { goto } from "../fixtures/navigation";

// The timing script every document of a measured context gets (M12/P4 design 4.3): what a measure waits for, as a
// condition looked at on each change of the DOM (a MutationObserver) and each frame; and, the first time it holds,
// two animation frames later, the frame it holds in drawn, the moment "painted". Both moments are the document's
// performance.now(), from its timeOrigin, the start of its navigation. It also keeps the document's long tasks and
// the last click, whose event's timeStamp is on the same clock. No element timing: the rows would need an
// elementtiming attribute the product does not set.

/** What a measure waits for. */
export type Watch =
  /** T1: rows in the tree of the navigation nav, down to the viewport's bottom edge or all roots of them. */
  | { kind: "tree"; nav: string; roots: number }
  /** T2: the first row of the tree's root list named name, its link's text. */
  | { kind: "first"; nav: string; name: string }
  /** T3: the rows of the list the button label of the tree controls, down to the viewport's bottom edge or count. */
  | { kind: "children"; nav: string; button: string; count: number }
  /** The parent's select of the dialog titled title, with its options. */
  | { kind: "dialog"; title: string };

/** What the probe saw: when the condition first held and when that frame was drawn; what it counted once drawn. */
export interface Seen {
  met: number;
  painted?: number;
  /** The rows (or options) there, those in the viewport; whether the row looked at is in the viewport. */
  rows: number;
  inView: number;
  intersects: boolean;
}

/** A long task of the document: its start, on the document's clock, and how long it took. */
export interface LongTask {
  start: number;
  duration: number;
}

interface Probe {
  watch(key: string, what: Watch): void;
  seen(key: string): Seen | undefined;
  longTasks(): LongTask[];
  lastClick(): { timeStamp: number; label: string | null } | undefined;
}

declare global {
  interface Window {
    nwPerf: Probe;
  }
}

/** Gives every document of context the probe, which watches for t1 under the key "t1" from the document's start. */
async function installProbe(context: BrowserContext, t1: Watch): Promise<void> {
  await context.addInitScript(probe, t1);
}

/** Has page's probe watch for what under key. */
export async function watchFor(page: Page, key: string, what: Watch): Promise<void> {
  await page.evaluate(([k, w]) => window.nwPerf.watch(k, w), [key, what] as const);
}

/** What page's probe saw under key once painted, within timeout. */
export async function painted(page: Page, key: string, timeout: number): Promise<Seen & { painted: number }> {
  await page.waitForFunction((k) => window.nwPerf.seen(k)?.painted !== undefined, key, { timeout, polling: 50 });
  return (await page.evaluate((k) => window.nwPerf.seen(k), key)) as Seen & { painted: number };
}

/** A context of perf's at baseURL, signed in with tokens, every document of it probed for t1 (installProbe). */
export async function probedContext(
  browser: Browser,
  baseURL: string,
  tokens: AuthTokens,
  t1: Watch
): Promise<BrowserContext> {
  const context = await browser.newContext({ baseURL });
  await signInContext(context, baseURL, tokens);
  await installProbe(context, t1);
  return context;
}

/** What one cold open (T1) gives: its moments from the document's start, and what it counted. */
export interface ColdOpen {
  /** The moment the rows were painted: T1. */
  t1: number;
  met: number;
  rows: number;
  inView: number;
  /** The tree's answer read in full, and DOMContentLoaded's end. */
  nodesEnd?: number;
  domContentLoaded?: number;
  /** The bytes of the app's main script that came over the network: 0 from the HTTP cache. */
  mainJs?: number;
  /** The longest long task that started before the paint. */
  longest: number;
  /**
   * The items of the home's list of pages, and of the whole document, read just after T1, a round trip later: since
   * M12/P4/S4 a chunk or two past the first by then (the first chunk is rows, as T1 painted).
   */
  homeItems: number;
  items: number;
  /**
   * When every row was asked for (ColdOptions.all): the frame the tree's root list and the home's list both had them
   * all in, from the document's start, not yet drawn; the longest long task that started from T1 to it; and the items
   * of the home's list and of the document then (M12/P4/S4 design 6.3).
   */
  all?: number;
  allLongest?: number;
  allHomeItems?: number;
  allItems?: number;
  /** JSHeapUsedSize after a garbage collection, when asked for: once every row is in, when they were asked for. */
  heap?: number;
}

/** What coldOpen does besides: clear the HTTP cache (the default), read the heap, and before and after the open. */
export interface ColdOptions {
  clear?: boolean;
  heap?: boolean;
  /** Waits after T1 for every row: count of them in the root list of the tree nav and in the home's list. */
  all?: { nav: string; count: number };
  timeout?: number;
  before?: (tab: Page, cdp: CDPSession) => Promise<void>;
  after?: (tab: Page, cdp: CDPSession) => Promise<void>;
}

/**
 * One cold open of home, the context's (M12/P4 design 4.3, T1): a new tab, a renderer of its own, watched for the
 * helper's goto, its HTTP cache cleared; T1 is the moment the probe's "t1" painted, from the document's start. Asked
 * to, it then waits for every row (allRows), and reads the heap after. The tab then leaves for about:blank by the
 * helper, which waits for its refresh, and closes.
 */
export async function coldOpen(context: BrowserContext, home: string, options: ColdOptions = {}): Promise<ColdOpen> {
  const tab = await context.newPage();
  await watchPage(tab);
  const cdp = await context.newCDPSession(tab);
  if (options.clear !== false) {
    await cdp.send("Network.clearBrowserCache");
  }
  await options.before?.(tab, cdp);
  await goto(tab, home);
  const seen = await painted(tab, "t1", options.timeout ?? 120_000);
  const timing = await tab.evaluate((at) => {
    const resources = performance.getEntriesByType("resource") as PerformanceResourceTiming[];
    const nodes = resources.find((entry) => /^\/api\/v0\/notebooks\/[^/]+\/nodes$/.test(new URL(entry.name).pathname));
    const main = resources.find((entry) => /^\/assets\/index-[^/]+\.js$/.test(new URL(entry.name).pathname));
    const navigation = performance.getEntriesByType("navigation")[0] as PerformanceNavigationTiming | undefined;
    const before = window.nwPerf.longTasks().filter((task) => task.start < at);
    return {
      nodesEnd: nodes?.responseEnd,
      domContentLoaded: navigation?.domContentLoadedEventEnd,
      mainJs: main?.transferSize,
      longest: Math.max(0, ...before.map((task) => task.duration)),
      homeItems: document.querySelectorAll('ul[aria-label="Pages"] > li').length,
      items: document.querySelectorAll("li").length,
    };
  }, seen.painted);
  let every: Pick<ColdOpen, "all" | "allLongest" | "allHomeItems" | "allItems"> = {};
  if (options.all !== undefined) {
    const at = await allRows(tab, options.all.nav, options.all.count, options.timeout ?? 120_000);
    every = await tab.evaluate(
      ([from, to]) => ({
        all: to,
        allLongest: Math.max(
          0,
          ...window.nwPerf
            .longTasks()
            .filter((task) => task.start >= from && task.start <= to)
            .map((task) => task.duration)
        ),
        allHomeItems: document.querySelectorAll('ul[aria-label="Pages"] > li').length,
        allItems: document.querySelectorAll("li").length,
      }),
      [seen.painted, at] as const
    );
  }
  let heap: number | undefined;
  if (options.heap === true) {
    await cdp.send("HeapProfiler.collectGarbage");
    await cdp.send("Performance.enable");
    const { metrics } = await cdp.send("Performance.getMetrics");
    heap = metrics.find((metric) => metric.name === "JSHeapUsedSize")?.value;
  }
  await options.after?.(tab, cdp);
  await goto(tab, "about:blank");
  await tab.close();
  return { t1: seen.painted, met: seen.met, rows: seen.rows, inView: seen.inView, ...timing, ...every, heap };
}

/**
 * allRows waits, a frame at a time, until the root list of the tree nav and the home's list of pages each have count
 * rows (M12/P4/S4 design 3.6: the first chunk at once, the rest after each paint), and answers the moment of that
 * frame, from the document's start: every row is in the document, not yet drawn. A list not there has none: a page
 * without the home's list, or with it named otherwise, fails at the timeout, not counted full.
 */
export async function allRows(page: Page, nav: string, count: number, timeout: number): Promise<number> {
  return page.evaluate(
    ([label, rows, ms]) =>
      new Promise<number>((resolve, reject) => {
        const until = performance.now() + ms;
        const look = () => {
          const now = performance.now();
          const tree = [...document.querySelectorAll("nav")].find((each) => each.getAttribute("aria-label") === label);
          const left = tree?.querySelectorAll(":scope > ul > li").length ?? 0;
          const main = document.querySelectorAll('ul[aria-label="Pages"] > li').length;
          if (left >= rows && main >= rows) {
            resolve(now);
          } else if (now > until) {
            reject(new Error(`every row: ${left} in the tree and ${main} on the home of ${rows} after ${ms} ms`));
          } else {
            requestAnimationFrame(look);
          }
        };
        requestAnimationFrame(look);
      }),
    [nav, count, timeout] as const
  );
}

/**
 * The bytes of the tree's answer for the notebook notebookId, as credential reads it, and of the same JSON
 * compressed: gzip at levels 1 (nginx's default) and 6, zstd at level 3 (Caddy's encode default) (design 2.1 item 6).
 */
export async function treeBytes(baseURL: string, credential: string, notebookId: string) {
  const answer = await fetch(new URL(`/api/v0/notebooks/${notebookId}/nodes`, baseURL), {
    headers: bearer(credential),
  });
  if (answer.status !== 200) {
    throw new Error(`read the tree of ${notebookId}: ${answer.status}`);
  }
  const json = Buffer.from(await answer.arrayBuffer());
  return {
    json: json.length,
    gzip1: gzipSync(json, { level: 1 }).length,
    gzip6: gzipSync(json, { level: 6 }).length,
    zstd3: zstdCompressSync(json, { params: { [constants.ZSTD_c_compressionLevel]: 3 } }).length,
  };
}

/** The probe itself, serialised into each document: it uses nothing outside its body. */
function probe(first: Watch): void {
  const longTasks: LongTask[] = [];
  try {
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) longTasks.push({ start: entry.startTime, duration: entry.duration });
    }).observe({ type: "longtask", buffered: true });
  } catch {
    // no long tasks to watch here
  }
  let click: { timeStamp: number; label: string | null } | undefined;
  addEventListener(
    "click",
    (event) => {
      const target = event.target instanceof Element ? event.target.closest("[aria-label]") : null;
      click = { timeStamp: event.timeStamp, label: target?.getAttribute("aria-label") ?? null };
    },
    true
  );
  const pending = new Map<string, Watch>();
  const seen = new Map<string, Seen>();

  // oxlint-disable-next-line unicorn/consistent-function-scoping -- serialised whole: it stays in the probe's body
  const navOf = (label: string) =>
    [...document.querySelectorAll("nav")].find((nav) => nav.getAttribute("aria-label") === label);
  // oxlint-disable-next-line unicorn/consistent-function-scoping -- serialised whole: it stays in the probe's body
  const inViewport = (element: Element) => {
    const box = element.getBoundingClientRect();
    return box.bottom > 0 && box.top < innerHeight && box.height > 0;
  };
  // The rows of a list: each item's first child, the row's div (page-tree-item.tsx).
  // oxlint-disable-next-line unicorn/consistent-function-scoping -- serialised whole: it stays in the probe's body
  const rowsOf = (root: ParentNode, direct: boolean) => [
    ...root.querySelectorAll(direct ? ":scope > li > :first-child" : "li > :first-child"),
  ];
  // Rows down to the viewport's bottom edge, or at least count of them. The count is looked at first: the position
  // of a row lays the document out, which in the commit's microtask would be work the page itself does later, in the
  // frame; with count rows there (a whole list is one commit), it never needs one.
  // oxlint-disable-next-line unicorn/consistent-function-scoping -- serialised whole: it stays in the probe's body
  const filled = (rows: Element[], count: number) => {
    const last = rows.at(-1);
    return last !== undefined && (rows.length >= count || last.getBoundingClientRect().top >= innerHeight);
  };

  // What the condition holds on once it holds: the rows, the one row or the select, to count and place once painted.
  const look = (what: Watch): Element[] | undefined => {
    if (what.kind === "dialog") {
      const dialog = [...document.querySelectorAll('[role="dialog"]')].find(
        (each) => document.getElementById(each.getAttribute("aria-labelledby") ?? "")?.textContent === what.title
      );
      const select = dialog?.querySelector("select");
      return select && select.options.length >= 2 ? [select] : undefined;
    }
    const nav = navOf(what.nav);
    if (nav === undefined) return undefined;
    if (what.kind === "first") {
      const row = nav.querySelector(":scope > ul > li:first-child > :first-child");
      return row && row.querySelector("a")?.textContent === what.name ? [row] : undefined;
    }
    if (what.kind === "tree") {
      const rows = rowsOf(nav, false);
      return filled(rows, what.roots) ? rows : undefined;
    }
    const button = [...nav.querySelectorAll("button")].find((each) => each.getAttribute("aria-label") === what.button);
    const list = document.getElementById(button?.getAttribute("aria-controls") ?? "");
    if (list === null) return undefined;
    const rows = rowsOf(list, true);
    return filled(rows, what.count) ? rows : undefined;
  };

  // The moment it holds is taken before anything is counted; what it counts, rows in the viewport among them, only
  // once painted: placing them lays the document out, which must not fall between the two moments.
  const check = () => {
    for (const [key, what] of pending) {
      const met = performance.now();
      const found = look(what);
      if (found !== undefined) {
        pending.delete(key);
        const entry: Seen = { met, rows: 0, inView: 0, intersects: false };
        seen.set(key, entry);
        requestAnimationFrame(() =>
          requestAnimationFrame(() => {
            const drawn = performance.now();
            const select = found[0] instanceof HTMLSelectElement ? found[0] : undefined;
            entry.rows = select === undefined ? found.length : select.options.length;
            entry.inView = found.filter(inViewport).length;
            entry.intersects = found[0] !== undefined && inViewport(found[0]);
            entry.painted = drawn;
          })
        );
      }
    }
  };
  let looping = false;
  const loop = () => {
    check();
    looping = pending.size > 0;
    if (looping) requestAnimationFrame(loop);
  };
  const watch = (key: string, what: Watch) => {
    seen.delete(key);
    pending.set(key, what);
    check();
    if (!looping && pending.size > 0) {
      looping = true;
      requestAnimationFrame(loop);
    }
  };
  new MutationObserver(check).observe(document, { subtree: true, childList: true });
  window.nwPerf = {
    watch,
    seen: (key) => seen.get(key),
    longTasks: () => longTasks.slice(),
    lastClick: () => click,
  };
  watch("t1", first);
}
