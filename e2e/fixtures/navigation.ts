import { setTimeout as sleep } from "node:timers/promises";

import type { Page, Request, Response } from "@playwright/test";

// How the stories leave a document (00-M12 4.4, M12/P1/S1). A document that leaves before its refresh's answer
// comes back loses the refreshed token: the next document refreshes with the old one, nervewiki takes that for
// reuse, and the session ends, the page on /sign-in (M1 design 10 accepts this; A15 shows it). So goto and reload
// here wait first, until the page has no API request out, its document's first refresh has been answered and
// no refresh holds the lock; a story that leaves mid-request on purpose says so with the escape, the option
// inFlight set to leave. A gate (tools/e2e-navigation/check.mjs, make lint-web) lets no other module under e2e/
// call page.goto or page.reload, and counts the escapes. Back and forward within one document need none of this.
// Closing a tab leaves a document too, and may lose its refresh the same way; the helper does not cover it:
// the stories close an app's tab once it showed what only a signed-in session reads, and a story to come
// waits for such a thing first as well.

/**
 * A story's choice besides Playwright's options: "leave" goes at once, whatever is out. The gate counts the
 * calls that pass it.
 */
interface Leaving {
  inFlight?: "leave";
}

/** How long settle waits before it fails, naming what it waits for: longer than a refresh's own 8 s. */
const settleMs = 10_000;
const pollMs = 25;
/** How long one read in the page may take at least, even past settleMs: what it reads names the wait. */
const readMs = 1_000;
// The session's record, its refresh lock, the lease without Web Locks (W/session/token-manager.ts:16,
// W/session/refresh-lock.ts:14, :25), and the answers that give a document its tokens: the first refresh of one
// that started with the record (token-manager.ts:221-229), or the sign-in or sign-up that wrote it there
// (token-manager.ts:164-172, W/services/auth.service.ts:17, :26), after which no first refresh comes.
const authKey = "nwiki.auth";
const lockName = "nwiki.auth.refresh";
const leaseKey = "nwiki.auth.lease";
const tokenPaths = ["/api/v0/auth/refresh", "/api/v0/auth/login", "/api/v0/auth/register"];

/** Each watched page's API requests not answered yet. */
const unanswered = new WeakMap<Page, Set<Request>>();

/**
 * Follows page's API requests from now on: fetch and XHR under /api/v0/, but the event stream, which stays
 * open. watchPage calls it before the page's first navigation. A request that a route holds and never
 * answers stays here once its document is gone where the browser reports no failure for it (Chromium): the
 * page's next goto or reload waits for it, and fails.
 */
export function followRequests(page: Page): void {
  const open = new Set<Request>();
  unanswered.set(page, open);
  page.on("request", (request) => {
    const { pathname } = new URL(request.url());
    const type = request.resourceType();
    if ((type === "fetch" || type === "xhr") && pathname.startsWith("/api/v0/") && pathname !== "/api/v0/events") {
      open.add(request);
    }
  });
  page.on("requestfinished", (request) => open.delete(request));
  page.on("requestfailed", (request) => open.delete(request));
}

/** page.goto, once the page's document has nothing out that leaving would lose. */
export async function goto(
  page: Page,
  url: string,
  options: NonNullable<Parameters<Page["goto"]>[1]> & Leaving = {}
): Promise<Response | null> {
  const { inFlight, ...rest } = options;
  if (inFlight !== "leave") await settle(page);
  return page.goto(url, rest);
}

/** page.reload, the same way. */
export async function reload(
  page: Page,
  options: NonNullable<Parameters<Page["reload"]>[0]> & Leaving = {}
): Promise<Response | null> {
  const { inFlight, ...rest } = options;
  if (inFlight !== "leave") await settle(page);
  return page.reload(rest);
}

/**
 * Waits until page's document has nothing out that leaving it would lose: no API request unanswered, and in
 * the page (sessionWaits) its first refresh answered and the refresh lock free. Fails after settleMs, naming
 * what it still waits for.
 */
async function settle(page: Page): Promise<void> {
  const open = unanswered.get(page);
  if (open === undefined) {
    throw new Error("goto, reload: the page is not watched; call watchPage before its first navigation");
  }
  const deadline = Date.now() + settleMs;
  for (;;) {
    let waits = [...open].map((request) => `${request.method()} ${new URL(request.url()).pathname}`);
    if (waits.length === 0) {
      // oxlint-disable-next-line no-await-in-loop -- read again each round until nothing is left to wait for
      waits = await sessionWaits(page, Math.max(readMs, deadline - Date.now()));
    }
    if (waits.length === 0) return;
    if (Date.now() >= deadline) {
      throw new Error(
        `goto, reload: after ${settleMs} ms the page still waits for ${waits.join(", ")}; ` +
          'a story that leaves it on purpose passes { inFlight: "leave" }'
      );
    }
    // oxlint-disable-next-line no-await-in-loop -- looked at again each round until nothing is left to wait for
    await sleep(pollMs);
  }
}

/**
 * What page's document still has to finish before it may be left, read in the page: its first refresh, while
 * it has a session's record and no answer that gave it tokens (tokenPaths), and the refresh lock (the lease
 * without Web Locks); none when it has nothing. The read races remainingMs: Chromium does not answer it while
 * the next document is requested and not yet committed, which counts as waiting for that document.
 */
async function sessionWaits(page: Page, remainingMs: number): Promise<string[]> {
  const read = page
    .evaluate(
      async (names) => {
        let record: string | null;
        try {
          record = localStorage.getItem(names.authKey);
        } catch {
          return []; // an opaque origin (an attachment shown sandboxed at its address, about:blank): no session here
        }
        // No session's record, or not the app (W/main.tsx:17 mounts on #root): no refresh to lose.
        if (record === null || document.getElementById("root") === null) return [];
        // A document's resource timings are its own requests, each added once its answer was read.
        const tokens = performance
          .getEntriesByType("resource")
          .some((entry) => names.tokenPaths.includes(new URL(entry.name).pathname));
        if (!tokens) return ["the document's first refresh"];
        if ("locks" in navigator) {
          const { held = [], pending = [] } = await navigator.locks.query();
          return [...held, ...pending].some((lock) => lock.name === names.lockName) ? ["the refresh lock"] : [];
        }
        const lease = JSON.parse(localStorage.getItem(names.leaseKey) ?? "null") as { expires?: number } | null;
        return lease !== null && (lease.expires ?? 0) > Date.now() ? ["the refresh lease"] : [];
      },
      { authKey, lockName, leaseKey, tokenPaths }
    )
    // A document a navigation is replacing: the next round reads the next one. Any other failure is a document
    // that cannot run the app, which has no session to lose.
    .catch((error: unknown) =>
      String(error).includes("Execution context was destroyed") ? ["the next document"] : []
    );
  const controller = new AbortController();
  const pending = sleep(remainingMs, ["the next document (pending)"], { signal: controller.signal }).catch(() => []);
  try {
    return await Promise.race([read, pending]);
  } finally {
    controller.abort();
  }
}
