import { setTimeout as sleep } from "node:timers/promises";

import type { Page, Request, Response } from "@playwright/test";

// How the stories leave a document (00-M12 4.4, M12/P1/S1). A document that leaves before its refresh's answer
// comes back loses the refreshed token: the next document refreshes with the old one, nervewiki takes that for
// reuse, and the session ends, the page on /sign-in (M1 design 10 accepts this; A15 shows it). So goto and reload
// here wait first, until the page has no API request out, its document has its tokens (the answer of its first
// refresh, or of a sign-in or sign-up in it) and nothing holds the refresh lock, under which a refresh, a sign-in
// or a sign-up writes its record; a story that leaves mid-request on purpose says so with the escape, the option
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

/** Each watched page's API requests not answered yet, of its top document. */
const unanswered = new WeakMap<Page, Set<Request>>();

/** An address without its fragment, which a document's request does not carry. */
function withoutFragment(url: string): string {
  return url.split("#")[0] ?? url;
}

/** Whether request asks for page's top document; a request whose frame is not there yet does not. */
function asksForTheTopDocument(page: Page, request: Request): boolean {
  try {
    return request.isNavigationRequest() && request.frame() === page.mainFrame();
  } catch {
    return false; // Playwright throws for a navigation request issued before its frame exists
  }
}

/**
 * Follows page's API requests from now on: fetch and XHR under /api/v0/, but the event stream, which stays
 * open. watchPage calls it before the page's first navigation. A request leaves the set once it is answered
 * and its answer read, or failed, or once its document is gone: when the top frame commits the document it
 * asked for last, whoever started that navigation (the helper, its escape, the page itself). Chromium reports
 * nothing more for a request whose document is gone (a route holds it, or its answer's body was not read yet:
 * LW3 on CI run 43686031128), nor does Firefox for one sent once the next document was asked for; otherwise
 * they report it failed, and WebKit does. A move within one document (history) keeps the set: its requests are
 * still out.
 */
export function followRequests(page: Page): void {
  const open = new Set<Request>();
  unanswered.set(page, open);
  // The address of the top document page asked for last, until it commits. A 204, or a download the browser
  // does not show, never does (WebKit shows one a route answers, and commits it): a move within the document to
  // that very address would be taken for its commit, any other is not; the next document asked for takes its
  // place.
  let asked: string | undefined;
  page.on("request", (request) => {
    if (asksForTheTopDocument(page, request)) {
      asked = withoutFragment(request.url());
      return;
    }
    const { pathname } = new URL(request.url());
    const type = request.resourceType();
    if ((type === "fetch" || type === "xhr") && pathname.startsWith("/api/v0/") && pathname !== "/api/v0/events") {
      open.add(request);
    }
  });
  page.on("requestfinished", (request) => open.delete(request));
  page.on("requestfailed", (request) => open.delete(request));
  // Playwright reports the commit before any request of the new document, which runs no script before it; it
  // drops the old document's requests from its own network idle at the same moment.
  page.on("framenavigated", (frame) => {
    if (frame !== page.mainFrame() || asked === undefined || withoutFragment(frame.url()) !== asked) return;
    asked = undefined;
    open.clear();
  });
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
 * the page (sessionWaits) the refresh lock free and its tokens answered, both in the same round. The page runs
 * on while it is read: the requests are looked at again a poll after the read, since with a route on the page
 * Playwright reports a request once the browser paused it, which can come after the read's answer. A request
 * the page sends after that, until its next document commits, goes out unwaited for; the commit drops it
 * (followRequests). Fails after settleMs, naming what it still waits for.
 */
async function settle(page: Page): Promise<void> {
  const open = unanswered.get(page);
  if (open === undefined) {
    throw new Error("goto, reload: the page is not watched; call watchPage before its first navigation");
  }
  const requests = () => [...open].map((request) => `${request.method()} ${new URL(request.url()).pathname}`);
  const deadline = Date.now() + settleMs;
  for (;;) {
    let waits = requests();
    if (waits.length === 0) {
      // oxlint-disable-next-line no-await-in-loop -- read again each round until nothing is left to wait for
      waits = await sessionWaits(page, Math.max(readMs, deadline - Date.now()));
      if (waits.length === 0) {
        // oxlint-disable-next-line no-await-in-loop -- a request sent as the page was read is reported a little later
        await sleep(pollMs);
        waits = requests();
      }
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
 * What page's document still has to finish before it may be left, read in the page: the refresh lock (the
 * lease without Web Locks), held or asked for, with or without a record, since a sign-in or a sign-up writes the
 * first one under it; then, while it has a session's record and no answer that gave it tokens (tokenPaths), its
 * first refresh; none when it has nothing. The read races remainingMs: Chromium does not answer it while the
 * next document is requested and not yet committed, which counts as waiting for that document.
 */
async function sessionWaits(page: Page, remainingMs: number): Promise<string[]> {
  const read = page
    .evaluate(
      async (names) => {
        try {
          localStorage.getItem(names.authKey);
        } catch {
          return []; // an opaque origin (an attachment shown sandboxed at its address, about:blank): no session here
        }
        // Not the app (W/main.tsx:17 mounts on #root): no session to lose.
        if (document.getElementById("root") === null) return [];
        // A refresh, a sign-in or a sign-up writes the record under the lock (W/session/token-manager.ts:164-172,
        // :288-310): a document whose sign-in was answered has no record until it holds it.
        if ("locks" in navigator) {
          const { held = [], pending = [] } = await navigator.locks.query();
          if ([...held, ...pending].some((lock) => lock.name === names.lockName)) return ["the refresh lock"];
        } else {
          const lease = JSON.parse(localStorage.getItem(names.leaseKey) ?? "null") as { expires?: number } | null;
          if (lease !== null && (lease.expires ?? 0) > Date.now()) return ["the refresh lease"];
        }
        // Read after the lock: no session's record by then, no refresh to lose.
        if (localStorage.getItem(names.authKey) === null) return [];
        // A document's resource timings are its own requests, each added once its answer was read.
        const tokens = performance
          .getEntriesByType("resource")
          .some((entry) => names.tokenPaths.includes(new URL(entry.name).pathname));
        return tokens ? [] : ["the document's first refresh"];
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
