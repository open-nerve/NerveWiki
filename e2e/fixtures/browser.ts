import { expect, type Page, type Request } from "@playwright/test";

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
 * Checks that nothing went wrong in page since watch began: no uncaught exception, no Content-Security-Policy
 * violation, and no console error or warning, such as React Router's warning about a missing HydrateFallback.
 * It logs a probe of each kind first and expects to find it, so that a watch that does not hear the console
 * cannot pass.
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
    .toEqual({ errors: [probe], warnings: [probe] });
}
