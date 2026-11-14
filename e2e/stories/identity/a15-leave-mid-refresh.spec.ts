import { setTimeout as sleep } from "node:timers/promises";

import type { AuthTokens } from "@nervewiki/api-client";
import type { Page, Request } from "@playwright/test";

import { expectRevoked, generationOf, sessionOf } from "../../fixtures/assert/identity";
import { emailFor, password, recordOf, registerOnboarded } from "../../fixtures/auth";
import { signInWith, signUpWith } from "../../fixtures/auth-pages";
import { failedToLoad, type PageWatch } from "../../fixtures/browser";
import type { Database } from "../../fixtures/db";
import { goto, reload } from "../../fixtures/navigation";
import { expect, test } from "../../fixtures/test";

// A15, leaving a page while its refresh is out (M1 design 10: the session ends, and the user signs in again; a
// known limitation in the release notes). The stories leave a document through fixtures/navigation.ts, which
// waits first: two tests here leave through its escape and land on the sign-in page; four go through it and stay
// signed in, each holding back one of the things the helper waits for; the last has none of them, its document
// signed up itself, and is not kept waiting. The answer, the next document and the first refresh are held so
// that each order happens for certain, not by running it again and again (05-lessons 6). Only the page has a
// version: the API's side, a retired token used again, is A5 (API).

/** A promise, and the function that settles it. */
function signal(): { settled: Promise<void>; settle: () => void } {
  let settle: (() => void) | undefined;
  const settled = new Promise<void>((resolve) => {
    settle = resolve;
  });
  return { settled, settle: () => settle?.() };
}

/** Whether event came within ms: "first", or "timeout". */
function within(event: Promise<void>, ms: number): Promise<"first" | "timeout"> {
  return Promise.race([event.then(() => "first" as const), sleep(ms, "timeout" as const)]);
}

/** Whether request is page's request of the top document at path. */
function documentAt(page: Page, request: Request, path: string): boolean {
  return (
    request.isNavigationRequest() && request.frame() === page.mainFrame() && new URL(request.url()).pathname === path
  );
}

/** What routeTheRefreshAgainstTheNextDocument holds. */
interface RefreshRoutes {
  /** Settles once nervewiki answered the page's first refresh: the session has turned to the next token. */
  readonly fetched: Promise<void>;
  /** Settles once the page asks for its next document under /settings/. */
  readonly nextDocument: Promise<void>;
  /** What the routes saw go wrong; a test asserts it is empty. */
  readonly problems: string[];
}

/**
 * Routes page's first refresh against its next document, before its first navigation: nervewiki answers the
 * refresh, and the page gets that answer 2 s later, or never once its next document was asked for first (the
 * document that sent it is leaving). The next document under /settings/ waits until that refresh reached
 * nervewiki, 5 s at most, so that the old document can still send it; past that it goes on, and the problems
 * say so.
 */
async function routeTheRefreshAgainstTheNextDocument(page: Page): Promise<RefreshRoutes> {
  const fetched = signal();
  const nextDocument = signal();
  const problems: string[] = [];
  let refreshes = 0;
  let documents = 0;
  await page.route("**/api/v0/auth/refresh", async (route) => {
    refreshes += 1;
    const response = await route.fetch();
    if (refreshes > 1 || problems.length > 0) {
      await route.fulfill({ response });
      return;
    }
    fetched.settle();
    if ((await within(nextDocument.settled, 2_000)) === "timeout") {
      await route.fulfill({ response });
    }
  });
  await page.route("**/settings/**", async (route) => {
    if (route.request().resourceType() !== "document") {
      await route.fallback();
      return;
    }
    documents += 1;
    if (documents > 1) {
      nextDocument.settle();
      if ((await within(fetched.settled, 5_000)) === "timeout") {
        problems.push("the old document never sent its refresh");
      }
    }
    await route.fallback();
  });
  return { fetched: fetched.settled, nextDocument: nextDocument.settled, problems };
}

/**
 * Holds back the first refresh of page's /settings/profile without taking the refresh lock, before its first
 * navigation: the app's lock requests there wait for a gate, which the document opens itself once a test
 * address (not under /api/v0/, which the helper and pageWatch follow) answers "go". The function returned
 * makes it answer so. The document asks for itself: Chromium does not answer page.evaluate on a document
 * while its next one is requested and not yet committed.
 */
async function holdTheFirstRefresh(page: Page): Promise<() => void> {
  let go = false;
  await page.route("**/__a15/release", (route) =>
    route.fulfill({ contentType: "text/plain", body: go ? "go" : "wait" })
  );
  await page.addInitScript(() => {
    if (location.pathname !== "/settings/profile") return;
    const request = LockManager.prototype.request as (...args: unknown[]) => Promise<unknown>;
    let open: (() => void) | undefined;
    const gate = new Promise<void>((resolve) => {
      open = resolve;
    });
    LockManager.prototype.request = function (this: LockManager, ...args: unknown[]) {
      return gate.then(() => request.apply(this, args));
    } as LockManager["request"];
    const ask = (): void => {
      fetch("/__a15/release", { cache: "no-store" })
        .then((answer) => answer.text())
        .then(
          (word) => (word === "go" ? open?.() : void setTimeout(ask, 25)),
          () => void setTimeout(ask, 25)
        );
    };
    ask();
  });
  return () => {
    go = true;
  };
}

/**
 * Holds the refresh lock in page's document, as another tab's refresh, sign-in or sign-out does. The function
 * returned lets it go; a document already left has nothing to let go.
 */
async function holdTheRefreshLock(page: Page): Promise<() => Promise<void>> {
  await page.evaluate((name) => {
    void navigator.locks.request(
      name,
      () =>
        new Promise<void>((resolve) => {
          Object.assign(window, { a15Release: resolve });
        })
    );
  }, "nwiki.auth.refresh");
  await page.waitForFunction(() => "a15Release" in window);
  return async () => {
    await page.evaluate(() => (window as unknown as { a15Release?: () => void }).a15Release?.()).catch(() => undefined);
  };
}

/** The generation of page's refresh token, or "no record". */
async function generationIn(page: Page): Promise<number | "no record"> {
  const record = await recordOf(page);
  return record === null ? "no record" : generationOf(record.refresh_token);
}

/** page went back to sign in from path: its session ended as reuse, after its next refresh answered 401. */
async function expectSentBackToSignIn(
  page: Page,
  pageWatch: PageWatch,
  db: Database,
  tokens: AuthTokens,
  path: string
): Promise<void> {
  const signIn = `/sign-in?next=${encodeURIComponent(path)}`;
  await expect(page.getByRole("heading", { level: 1, name: "Sign in", exact: true })).toBeVisible();
  await expect(page).toHaveURL(signIn);
  expect(await recordOf(page)).toBeNull();
  await expectRevoked(db, tokens.refresh_token, "reuse_detected");
  expect(pageWatch.signInLandings).toEqual([`${signIn} after a refresh answered 401`]);
  // The browsers differ on reporting the refresh the old document never got back; the 401 is everyone's.
  expect(pageWatch.apiFailures).toContain("401 POST /api/v0/auth/refresh");
  pageWatch.expectConsole({ errors: [failedToLoad(401)] });
}

/** page shows the settings' section at path, its session refreshed once per document and live. */
async function expectSignedInOn(
  page: Page,
  pageWatch: PageWatch,
  db: Database,
  tokens: AuthTokens,
  path: string,
  section: string
): Promise<void> {
  await expect(page.getByRole("heading", { level: 1, name: "Settings", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { level: 2, name: section, exact: true })).toBeVisible();
  await expect(page).toHaveURL(path);
  await expect.poll(() => generationIn(page)).toBe(2);
  expect((await sessionOf(db, tokens.refresh_token)).revoked_at).toBeNull();
  expect(pageWatch.signInLandings).toEqual([]);
}

test("A15 (page): reloading through the escape while the refresh's answer is out ends the session", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const tokens = await registerOnboarded(api, emailFor(testInfo));
  const page = await signedInPage(tokens);
  const routes = await routeTheRefreshAgainstTheNextDocument(page);

  await goto(page, "/settings/profile");
  await routes.fetched;
  // nervewiki has turned the session to the next token; the page leaves before the answer comes back.
  await reload(page, { inFlight: "leave" });

  await expectSentBackToSignIn(page, pageWatch, db, tokens, "/settings/profile");
  expect(routes.problems).toEqual([]);
  // The address replaced with itself, as a router may, is no second landing.
  await Promise.all([
    page.waitForEvent("framenavigated", (frame) => frame === page.mainFrame()),
    page.evaluate(() => history.replaceState(history.state, "", location.href)),
  ]);
  expect(pageWatch.signInLandings).toEqual(["/sign-in?next=%2Fsettings%2Fprofile after a refresh answered 401"]);
});

test("A15 (page): reloading through the helper at that moment waits for the answer, and the page stays signed in", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const tokens = await registerOnboarded(api, emailFor(testInfo));
  const page = await signedInPage(tokens);
  const routes = await routeTheRefreshAgainstTheNextDocument(page);

  await goto(page, "/settings/profile");
  await routes.fetched;
  // The helper waits for the answer (2 s on), the lock and the reads after it, then reloads.
  await reload(page);

  await expectSignedInOn(page, pageWatch, db, tokens, "/settings/profile", "Profile");
  expect(routes.problems).toEqual([]);
});

test("A15 (page): leaving through the escape before the document's first refresh went out ends the session", async ({
  api,
  browserName,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  test.skip(
    browserName === "webkit",
    "WebKit sends nothing from the old document once the next one is requested: no window to leave mid-refresh in (M12/P1 design 3.6)"
  );
  const tokens = await registerOnboarded(api, emailFor(testInfo));
  const page = await signedInPage(tokens);
  const release = await holdTheFirstRefresh(page);
  const routes = await routeTheRefreshAgainstTheNextDocument(page);

  await goto(page, "/settings/profile");
  const leaving = goto(page, "/settings/security", { inFlight: "leave" });
  // The next document is asked for, and held; only then does the old one send its first refresh, which
  // nervewiki carries out and the old document never gets back.
  await routes.nextDocument;
  release();
  await leaving;

  await expectSentBackToSignIn(page, pageWatch, db, tokens, "/settings/security");
  expect(routes.problems).toEqual([]);
});

test("A15 (page): leaving through the helper at that moment waits for the first refresh, and the page stays signed in", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const tokens = await registerOnboarded(api, emailFor(testInfo));
  const page = await signedInPage(tokens);
  const release = await holdTheFirstRefresh(page);
  const routes = await routeTheRefreshAgainstTheNextDocument(page);

  await goto(page, "/settings/profile");
  const leaving = goto(page, "/settings/security");
  // Nothing is out and the lock is free: only the first refresh, not yet sent, holds the helper. A helper that
  // does not wait for it asks for the next document at once, well within the second.
  await within(routes.nextDocument, 1_000);
  release();
  await leaving;

  expect(routes.problems).toEqual([]);
  await expectSignedInOn(page, pageWatch, db, tokens, "/settings/security", "Password");
});

test("A15 (page): leaving through the helper while a read is out waits for its answer", async ({
  api,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const page = await signedInPage(await registerOnboarded(api, emailFor(testInfo)));
  const nextDocument = signal();
  page.on("request", (request) => {
    if (documentAt(page, request, "/settings/security")) nextDocument.settle();
  });
  // The account's first read is held: answered 2 s on, or never once the next document was asked for first.
  const reached = signal();
  const held: { read?: Request } = {};
  let answered: ((outcome: "answered" | "dropped") => void) | undefined;
  const outcome = new Promise<"answered" | "dropped">((resolve) => {
    answered = resolve;
  });
  await page.route("**/api/v0/me", async (route) => {
    if (held.read !== undefined || route.request().method() !== "GET") {
      await route.fallback();
      return;
    }
    held.read = route.request();
    const response = await route.fetch();
    reached.settle();
    if ((await within(nextDocument.settled, 2_000)) === "first") {
      answered?.("dropped");
      return;
    }
    await route.fulfill({ response });
    answered?.("answered");
  });

  await goto(page, "/settings/profile");
  await reached.settled;
  await goto(page, "/settings/security");

  expect(await outcome).toBe("answered");
  expect((await held.read?.response())?.status()).toBe(200);
  // The read was not cut off by the next document.
  expect(pageWatch.apiFailures).toEqual([]);
  await expect(page).toHaveURL("/settings/security");
  await expect(page.getByRole("heading", { level: 2, name: "Password", exact: true })).toBeVisible();
});

test("A15 (page): leaving through the helper while the refresh lock is held waits until it is let go", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const tokens = await registerOnboarded(api, emailFor(testInfo));
  const page = await signedInPage(tokens);
  const nextDocument = signal();
  let requestedAt = 0;
  page.on("request", (request) => {
    if (requestedAt === 0 && documentAt(page, request, "/settings/security")) {
      requestedAt = Date.now();
      nextDocument.settle();
    }
  });

  await goto(page, "/settings/profile");
  // The document's first refresh has been answered; then the page holds the refresh lock, as a refresh writing
  // its record, or another tab's, does.
  await page.waitForFunction(
    (path) => performance.getEntriesByType("resource").some((entry) => new URL(entry.name).pathname === path),
    "/api/v0/auth/refresh"
  );
  const letGo = await holdTheRefreshLock(page);
  const leaving = goto(page, "/settings/security");
  await within(nextDocument.settled, 1_000);
  const releasedAt = Date.now();
  // A helper that did not wait has left this document already: there is nothing to let go then.
  await letGo();
  await leaving;

  expect(requestedAt, "the next document is asked for once the lock is let go").toBeGreaterThan(releasedAt);
  await expect(page).toHaveURL("/settings/security");
  await expect(page.getByRole("heading", { level: 2, name: "Password", exact: true })).toBeVisible();
  expect((await sessionOf(db, tokens.refresh_token)).revoked_at).toBeNull();
});

test("A15 (page): a page that signed up in its own document has no first refresh to wait for: the helper reloads it", async ({
  db,
  page,
  pageWatch,
}, testInfo) => {
  await goto(page, "/sign-up");
  const answer = await signUpWith(page, emailFor(testInfo), password);
  expect(answer.status()).toBe(201);
  const tokens = (await answer.json()) as AuthTokens;
  await expect(page.getByRole("heading", { level: 1, name: "Your name", exact: true })).toBeVisible();
  // The document has its tokens from the sign-up, which wrote the record: no refresh comes from it.
  expect(await generationIn(page)).toBe(0);
  expect(pageWatch.apiRequests).not.toContain("POST /api/v0/auth/refresh");

  await reload(page);

  await expect(page).toHaveURL("/onboarding");
  await expect(page.getByRole("heading", { level: 1, name: "Your name", exact: true })).toBeVisible();
  await expect.poll(() => generationIn(page)).toBe(1);
  expect((await sessionOf(db, tokens.refresh_token)).revoked_at).toBeNull();
});

test("A15 (page): leaving through the helper while a sign-in waits for the refresh lock to write its record waits until it is written", async ({
  api,
  db,
  page,
  pageWatch,
}, testInfo) => {
  const email = emailFor(testInfo);
  await registerOnboarded(api, email);
  const nextDocument = signal();
  let requestedAt = 0;
  page.on("request", (request) => {
    if (requestedAt === 0 && documentAt(page, request, "/settings/security")) {
      requestedAt = Date.now();
      nextDocument.settle();
    }
  });

  await goto(page, "/settings/profile");
  await expect(page.getByRole("heading", { level: 1, name: "Sign in", exact: true })).toBeVisible();
  // The page holds the refresh lock, as another tab's refresh or sign-in does: the sign-in answered, its record
  // waits for the lock to be written (W/session/token-manager.ts:164-172). Nothing is out, the document has no
  // record yet, and only the lock holds the helper.
  const letGo = await holdTheRefreshLock(page);
  const answer = await signInWith(page, email, password);
  expect(answer.status()).toBe(200);
  const tokens = (await answer.json()) as AuthTokens;
  expect(await recordOf(page)).toBeNull();
  const leaving = goto(page, "/settings/security");
  await within(nextDocument.settled, 1_000);
  const releasedAt = Date.now();
  await letGo();
  await leaving;

  expect(requestedAt, "the next document is asked for once the sign-in wrote its record").toBeGreaterThan(releasedAt);
  await expect(page).toHaveURL("/settings/security");
  await expect(page.getByRole("heading", { level: 2, name: "Password", exact: true })).toBeVisible();
  // The next document found the sign-in's record and refreshed it: the session goes on, the page did not go back
  // to sign in.
  await expect.poll(() => generationIn(page)).toBe(1);
  expect((await sessionOf(db, tokens.refresh_token)).revoked_at).toBeNull();
  expect(pageWatch.signInLandings).toEqual(["/sign-in?next=%2Fsettings%2Fprofile"]);
});
