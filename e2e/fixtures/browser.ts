import { STATUS_CODES } from "node:http";

import { expect, type Locator, type Page, type Request, type Response, type TestInfo } from "@playwright/test";

/**
 * What a page did that a story checks: its API calls, and what went wrong in it. The event stream (M5/P3
 * design 3.12) is a connection the app keeps in the background, opened again as it ends, whose failures race
 * the story's own steps (a stream reconnecting as the session ends answers 401): its requests and their
 * failures are not among the API's, and the browser's reports of its answers of 400 or more are not among
 * the console's errors but in eventStreamErrors, for the stories of the stream to check.
 */
export interface PageWatch {
  /** Every API request but the event stream's, as "<method> <path>". */
  readonly apiRequests: string[];
  /**
   * API requests but the event stream's that failed: "<status> <method> <path>", or the browser's error for
   * one without an answer.
   */
  readonly apiFailures: string[];
  /** The console's reports of the event stream's answers of 400 or more. */
  readonly eventStreamErrors: string[];
  /** Uncaught exceptions and unhandled rejections. */
  readonly pageErrors: string[];
  /** Console messages of type error. */
  readonly consoleErrors: string[];
  /** Console messages of type warning. */
  readonly consoleWarnings: string[];
  /** Content-Security-Policy violations, as "<directive> <blocked URI>". */
  readonly cspViolations: string[];
  /** The console errors and warnings the story said would come, in order: see expectConsole. */
  readonly expectedConsole: { readonly errors: string[]; readonly warnings: string[] };
  /**
   * Declares what the page will log that is no fault of the app, in the order it comes: the errors the
   * browser logs about what the story makes happen, such as its report of each answer of 400 or more
   * (failedToLoad). The check as the test ends (expectQuietPage) expects exactly what was declared.
   */
  expectConsole(expected: { errors?: readonly string[]; warnings?: readonly string[] }): void;
}

/** The next answer page gets to method path. */
export function answerTo(page: Page, method: string, path: string): Promise<Response> {
  return page.waitForResponse(
    (response) => response.request().method() === method && new URL(response.url()).pathname === path
  );
}

/** Counts the answers page gets to method path from now on: the function returned gives how many so far. */
export function countAnswers(page: Page, method: string, path: string): () => number {
  let count = 0;
  page.on("response", (response) => {
    if (response.request().method() === method && new URL(response.url()).pathname === path) {
      count += 1;
    }
  });
  return () => count;
}

/** The note under field: its problem when it has one, else its hint; null without either. */
export function noteOf(field: Locator): Promise<string | null> {
  return field.evaluate((input) => {
    const id = input.getAttribute("aria-describedby");
    return id === null ? null : (document.getElementById(id)?.textContent ?? null);
  });
}

/** What Chromium logs on the console when the page gets an answer of status, 400 or more, to a request. */
export function failedToLoad(status: number): string {
  return `Failed to load resource: the server responded with a status of ${status} (${STATUS_CODES[status]})`;
}

/** The path of the event stream, GET /api/v0/events. */
export const eventStreamPath = "/api/v0/events";

/** The path of request to the API, but the event stream's: undefined for any other request. */
function apiPath(request: Request): string | undefined {
  const { pathname } = new URL(request.url());
  return pathname.startsWith("/api/") && pathname !== eventStreamPath ? pathname : undefined;
}

/** Whether a console message reports a load of the event stream: the browser gives its address as where. */
function aboutEventStream(location: { url: string }): boolean {
  return URL.canParse(location.url) && new URL(location.url).pathname === eventStreamPath;
}

/**
 * Starts watching page, and every document it loads from now on: call it before the page's first
 * navigation, so that nothing the app does as it starts goes unseen.
 */
export async function watchPage(page: Page): Promise<PageWatch> {
  const watch: PageWatch = {
    apiRequests: [],
    apiFailures: [],
    eventStreamErrors: [],
    pageErrors: [],
    consoleErrors: [],
    consoleWarnings: [],
    cspViolations: [],
    expectedConsole: { errors: [], warnings: [] },
    expectConsole: ({ errors = [], warnings = [] }) => {
      watch.expectedConsole.errors.push(...errors);
      watch.expectedConsole.warnings.push(...warnings);
    },
  };
  page.on("request", (request) => {
    const path = apiPath(request);
    if (path !== undefined) {
      watch.apiRequests.push(`${request.method()} ${path}`);
    }
  });
  // A request nervewiki answered is judged by its status: Chromium also reports one failed (net::ERR_ABORTED)
  // when the page leaves the body of its answer unread.
  const answered = new WeakSet<Request>();
  page.on("response", (response) => {
    answered.add(response.request());
    const path = apiPath(response.request());
    if (path !== undefined && response.status() >= 400) {
      watch.apiFailures.push(`${response.status()} ${response.request().method()} ${path}`);
    }
  });
  page.on("requestfailed", (request) => {
    const path = apiPath(request);
    if (path !== undefined && !answered.has(request)) {
      watch.apiFailures.push(`${request.failure()?.errorText} ${request.method()} ${path}`);
    }
  });
  page.on("pageerror", (error) => {
    watch.pageErrors.push(error.message);
  });
  page.on("console", (message) => {
    if (message.type() === "error" && aboutEventStream(message.location())) {
      watch.eventStreamErrors.push(message.text());
    } else if (message.type() === "error") {
      watch.consoleErrors.push(message.text());
    } else if (message.type() === "warning") {
      watch.consoleWarnings.push(message.text());
    }
  });
  await page.exposeBinding("nervewikiE2eCspViolation", (_source, violation: string) => {
    watch.cspViolations.push(violation);
  });
  await page.addInitScript(() => {
    document.addEventListener("securitypolicyviolation", (event) => {
      const report = (window as unknown as { nervewikiE2eCspViolation: (violation: string) => void })
        .nervewikiE2eCspViolation;
      report(`${event.effectiveDirective} ${event.blockedURI}`);
    });
  });
  return watch;
}

/**
 * Follows the access token page sends from now on, in the Authorization header of its requests: the function
 * returned gives the last one sent, or "" before the first.
 */
export function followAccessToken(page: Page): () => string {
  let last = "";
  page.on("request", (request) => {
    last = request.headers().authorization?.replace(/^Bearer /, "") ?? last;
  });
  return () => last;
}

/**
 * Checks that nothing went wrong in page since watch began: no uncaught exception, no Content-Security-Policy
 * violation, and no console error or warning but the ones the story declared (expectConsole), such as React
 * Router's warning about a missing HydrateFallback. It logs a probe of each kind first and expects to find it
 * after them, so that a watch that does not hear the console cannot pass, and a declared line that stops
 * coming fails as an undeclared one does.
 */
export async function expectQuietPage(page: Page, watch: PageWatch): Promise<void> {
  expect(watch.pageErrors, "uncaught exceptions in the page").toEqual([]);
  expect(watch.cspViolations, "Content-Security-Policy violations").toEqual([]);
  const probe = "nervewiki-e2e: console probe";
  await page.evaluate((text) => {
    console.error(text);
    console.warn(text);
  }, probe);
  await expect
    .poll(() => ({ errors: watch.consoleErrors, warnings: watch.consoleWarnings }))
    .toEqual({
      errors: [...watch.expectedConsole.errors, probe],
      warnings: [...watch.expectedConsole.warnings, probe],
    });
}

/** How much of a page's ARIA snapshot pageState keeps. */
const snapshotLimit = 3000;

/**
 * pageState is what page showed and did last, as text: its address, the element in focus, its last API
 * requests and their failures, its errors, and its ARIA snapshot cut short; none for a closed page.
 */
async function pageState(page: Page, watch: PageWatch): Promise<string | undefined> {
  if (page.isClosed()) {
    return undefined;
  }
  const focused = await page
    .evaluate(() => {
      const element = document.activeElement;
      if (element === null) {
        return "none";
      }
      const label = element.getAttribute("aria-label") ?? element.textContent?.trim().slice(0, 40) ?? "";
      return `${element.tagName.toLowerCase()}${element.getAttribute("role") ? `[role=${element.getAttribute("role")}]` : ""} "${label}"`;
    })
    .catch((error: unknown) => `unknown: ${String(error)}`);
  const snapshot = await page
    .locator("body")
    .ariaSnapshot({ timeout: 2_000 })
    .catch((error: unknown) => `none: ${String(error)}`);
  const cut = snapshot.length > snapshotLimit ? `${snapshot.slice(0, snapshotLimit)}\n…` : snapshot;
  return [
    `url: ${page.url()}`,
    `focused: ${focused}`,
    `api requests, the last 20: ${watch.apiRequests.slice(-20).join(", ") || "none"}`,
    `api failures: ${watch.apiFailures.join(", ") || "none"}`,
    `page errors: ${watch.pageErrors.join(" | ") || "none"}`,
    `console errors: ${watch.consoleErrors.join(" | ") || "none"}`,
    "aria snapshot:",
    cut,
  ].join("\n");
}

/** PageStates gathers the states of a test's pages that failed: see pageStatesOf. */
export interface PageStates {
  /** add attaches each open page's state, named, as its fixture ends: the page is still open. */
  add(pages: [string, Page, PageWatch][]): Promise<void>;
  /** fail fails the teardown once with every state added, if the test failed or timed out. */
  fail(): void;
}

/**
 * pageStatesOf is the page states of the test of testInfo. A failure's state goes in an error as well as an
 * attachment, one for the test: CI's annotations carry a failure's errors, not its attachments, and keep only
 * a few of them; the trace and the job's log are not public. A test that ended as expected only has them
 * attached.
 */
export function pageStatesOf(testInfo: TestInfo): PageStates {
  const states: string[] = [];
  return {
    async add(pages) {
      const named = (
        await Promise.all(pages.map(async ([name, page, watch]) => [name, await pageState(page, watch)] as const))
      ).filter((entry): entry is readonly [string, string] => entry[1] !== undefined);
      await Promise.all(
        named.map(([name, state]) => testInfo.attach(`page state: ${name}`, { body: state, contentType: "text/plain" }))
      );
      states.push(...named.map(([name, state]) => `${name}\n${state}`));
    },
    fail() {
      if (states.length > 0 && (testInfo.status === "failed" || testInfo.status === "timedOut")) {
        throw new Error(`What the pages showed last:\n\n${states.join("\n\n")}`);
      }
    },
  };
}
