import { STATUS_CODES } from "node:http";

import { expect, type Locator, type Page, type Request, type Response } from "@playwright/test";

/** What a page did that a story checks: its API calls, and what went wrong in it. */
export interface PageWatch {
  /** Every API request, as "<method> <path>". */
  readonly apiRequests: string[];
  /** API requests that failed: "<status> <method> <path>", or the browser's error for one without an answer. */
  readonly apiFailures: string[];
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

function apiPath(request: Request): string | undefined {
  const { pathname } = new URL(request.url());
  return pathname.startsWith("/api/") ? pathname : undefined;
}

/**
 * Starts watching page, and every document it loads from now on: call it before the page's first
 * navigation, so that nothing the app does as it starts goes unseen.
 */
export async function watchPage(page: Page): Promise<PageWatch> {
  const watch: PageWatch = {
    apiRequests: [],
    apiFailures: [],
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
    if (message.type() === "error") {
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
