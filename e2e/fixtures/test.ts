import path from "node:path";

import { createClient, type ApiClient, type AuthTokens } from "@nervewiki/api-client";
import { test as base, type BrowserContext, type Page } from "@playwright/test";

import { signInContext } from "./auth";
import { expectQuietPage, pageStatesOf, watchPage, type PageStates, type PageWatch } from "./browser";
import { createDatabase, dropDatabase, openDatabase, templateDatabase, type Database } from "./db";
import { connectEvents, type EventStream } from "./events";
import { nervewikiFixtureTimeoutMs, startNervewiki, type Nervewiki, type StartOptions } from "./server";

export { expect } from "@playwright/test";

/** A database made for one test: see newDatabase. */
interface TestDatabase {
  readonly url: string;
  /** Drops the database, closing every connection to it. */
  drop(): Promise<void>;
}

interface WorkerFixtures {
  /** The worker's own database, a copy of the migrated template, with a pool for the assertions. */
  db: Database;
  /** The worker's own nervewiki serve, on that database. */
  nervewiki: Nervewiki;
}

interface TestFixtures {
  /** The typed API client for the worker's nervewiki. */
  api: ApiClient;
  /** What the test's page did, watched from before its first navigation: see page. */
  pageWatch: PageWatch;
  /** The states of the test's pages once it failed, reported once as it ends: see pageStatesOf. */
  pageStates: PageStates;
  /**
   * Creates another database on the run's PostgreSQL: "migrated", a copy of the
   * template; "empty", with no migration applied. It outlives the test, until
   * the run's PostgreSQL stops.
   */
  newDatabase: (contents: "migrated" | "empty") => Promise<TestDatabase>;
  /**
   * Starts another nervewiki on the database at databaseUrl, with options
   * (extra variables, what to wait for); it stops when the test ends. Each
   * start adds the nervewiki fixture's budget to the test's timeout, so the
   * fixture's own timeouts fire first.
   */
  nervewikiWith: (databaseUrl: string, options?: StartOptions) => Promise<Nervewiki>;
  /**
   * Opens credential's event stream, past its hello, at the worker's nervewiki or at baseURL; it closes as the
   * test ends.
   */
  openEvents: (credential: string, baseURL?: string) => Promise<EventStream>;
  /**
   * Signs the test's browser context in with tokens, at the worker's nervewiki or at baseURL (see
   * signInContext), and returns the test's page, which has loaded nothing yet: its first page refreshes
   * the session.
   */
  signedInPage: (tokens: AuthTokens, baseURL?: string) => Promise<Page>;
  /**
   * Opens a page of another browser context signed in with tokens at the worker's nervewiki: another
   * account's tab, which has loaded nothing yet. It is watched as the test's page is, and a test that
   * passes must have left it quiet too; its context closes as the test ends.
   */
  anotherPage: (tokens: AuthTokens) => Promise<Page>;
  /**
   * Opens another tab in page's browser context, signed in as page is: it has loaded nothing yet. It is
   * watched as the test's page is, and a test that passes must have left it quiet too, unless it closed it.
   */
  anotherTab: (page: Page) => Promise<Page>;
  /**
   * When the test fails, a pg_dump of the worker's database joins its trace and screenshot. The logs are
   * in test-results/: the worker's nervewiki's at its root, those of nervewikiWith in the test's directory.
   * The databases of newDatabase are not dumped.
   */
  databaseSnapshot: void;
}

/**
 * The version make build stamped into bin/nervewiki, which the stories expect
 * nervewiki to report: make e2e passes it as NWIKI_E2E_VERSION.
 */
export function stampedVersion(): string {
  const version = process.env.NWIKI_E2E_VERSION;
  if (!version) {
    throw new Error(
      "NWIKI_E2E_VERSION is not set: run the stories with make e2e, which passes the VERSION it built with"
    );
  }
  return version;
}

/** Numbers the databases newDatabase creates in this worker: a worker is one process. */
let databases = 0;

/** The watch of each page the fixtures open: the test's, for pageWatch, and the other tabs and pages. */
const watches = new WeakMap<Page, PageWatch>();

/**
 * watchOf is the watch of a page the fixtures opened: another tab's or another account's, whose console a
 * story declares as the test's page's (pageWatch).
 */
export function watchOf(page: Page): PageWatch {
  const watch = watches.get(page);
  if (!watch) {
    throw new Error("the fixtures did not open this page");
  }
  return watch;
}

/** Stories import test from here: every worker runs its own nervewiki on its own database. */
export const test = base.extend<TestFixtures, WorkerFixtures>({
  db: [
    // oxlint-disable-next-line no-empty-pattern -- Playwright reads a fixture's dependencies from this pattern
    async ({}, use, workerInfo) => {
      const name = `e2e_w${workerInfo.workerIndex}`;
      await createDatabase(name, templateDatabase);
      const db = openDatabase(name);
      await use(db);
      await db.close();
    },
    { scope: "worker" },
  ],
  nervewiki: [
    async ({ db }, use, workerInfo) => {
      const logFile = path.join(workerInfo.project.outputDir, `nervewiki-w${workerInfo.workerIndex}.log`);
      const nervewiki = await startNervewiki(db.url, logFile);
      await use(nervewiki);
      await nervewiki.stop();
    },
    // Playwright's default worker-fixture budget (30 s, shared by setup and
    // teardown) is shorter than the fixture's own timeouts; give it room to
    // let those fire and report first.
    { scope: "worker", timeout: nervewikiFixtureTimeoutMs },
  ],
  // page and request resolve relative URLs against the worker's nervewiki.
  baseURL: async ({ nervewiki }, use) => {
    await use(nervewiki.baseURL);
  },
  // Every test's page is watched from before its first navigation, and a test that passes must have left it
  // quiet (expectQuietPage): the check runs as the test ends, so no story can forget it. What the page asked
  // of the API stays each story's to assert, through pageWatch.
  page: async ({ page, pageStates }, use, testInfo) => {
    const watch = await watchPage(page);
    watches.set(page, watch);
    await use(page);
    if (testInfo.status === testInfo.expectedStatus) {
      await expectQuietPage(page, watch);
    } else {
      await pageStates.add([["page", page, watch]]);
    }
  },
  // Torn down after the fixtures of pages, which depend on it.
  // oxlint-disable-next-line no-empty-pattern -- Playwright reads a fixture's dependencies from this pattern
  pageStates: async ({}, use, testInfo) => {
    const states = pageStatesOf(testInfo);
    await use(states);
    states.fail();
  },
  pageWatch: async ({ page }, use) => {
    const watch = watches.get(page);
    if (!watch) {
      throw new Error("the page fixture did not watch this page");
    }
    await use(watch);
  },
  api: async ({ nervewiki }, use) => {
    await use(createClient({ baseUrl: nervewiki.baseURL }));
  },
  // oxlint-disable-next-line no-empty-pattern -- Playwright reads a fixture's dependencies from this pattern
  newDatabase: async ({}, use, testInfo) => {
    await use(async (contents) => {
      databases += 1;
      const name = `e2e_w${testInfo.workerIndex}_${databases}`;
      const url = await createDatabase(name, contents === "migrated" ? templateDatabase : undefined);
      return { url, drop: () => dropDatabase(name) };
    });
  },
  // oxlint-disable-next-line no-empty-pattern -- Playwright reads a fixture's dependencies from this pattern
  nervewikiWith: async ({}, use, testInfo) => {
    const started: Nervewiki[] = [];
    await use(async (databaseUrl, options) => {
      testInfo.setTimeout(testInfo.timeout + nervewikiFixtureTimeoutMs);
      const log = testInfo.outputPath(`nervewiki-${started.length + 1}.log`);
      const nervewiki = await startNervewiki(databaseUrl, log, options);
      started.push(nervewiki);
      return nervewiki;
    });
    await Promise.all(started.map((nervewiki) => nervewiki.stop()));
  },
  openEvents: async ({ nervewiki }, use) => {
    const opened: EventStream[] = [];
    await use(async (credential, baseURL = nervewiki.baseURL) => {
      const stream = await connectEvents(baseURL, credential);
      opened.push(stream);
      return stream;
    });
    for (const stream of opened) {
      stream.close();
    }
  },
  signedInPage: async ({ page, nervewiki }, use) => {
    await use(async (tokens, baseURL = nervewiki.baseURL) => {
      await signInContext(page.context(), baseURL, tokens);
      return page;
    });
  },
  anotherPage: async ({ browser, nervewiki, pageStates }, use, testInfo) => {
    const contexts: BrowserContext[] = [];
    const watched: [Page, PageWatch][] = [];
    await use(async (tokens) => {
      const context = await browser.newContext({ baseURL: nervewiki.baseURL });
      contexts.push(context);
      await signInContext(context, nervewiki.baseURL, tokens);
      const page = await context.newPage();
      const watch = await watchPage(page);
      watches.set(page, watch);
      watched.push([page, watch]);
      return page;
    });
    try {
      if (testInfo.status === testInfo.expectedStatus) {
        await Promise.all(
          watched.filter(([page]) => !page.isClosed()).map(([page, watch]) => expectQuietPage(page, watch))
        );
      } else {
        await pageStates.add(watched.map(([page, watch], i) => [`another page ${i + 1}`, page, watch]));
      }
    } finally {
      await Promise.all(contexts.map((context) => context.close()));
    }
  },
  anotherTab: async ({ pageStates }, use, testInfo) => {
    const watched: [Page, PageWatch][] = [];
    await use(async (page) => {
      const tab = await page.context().newPage();
      const watch = await watchPage(tab);
      watches.set(tab, watch);
      watched.push([tab, watch]);
      return tab;
    });
    if (testInfo.status === testInfo.expectedStatus) {
      await Promise.all(watched.filter(([tab]) => !tab.isClosed()).map(([tab, watch]) => expectQuietPage(tab, watch)));
    } else {
      await pageStates.add(watched.map(([tab, watch], i) => [`another tab ${i + 1}`, tab, watch]));
    }
  },
  databaseSnapshot: [
    async ({ db }, use, testInfo) => {
      await use();
      if (testInfo.status !== testInfo.expectedStatus) {
        const file = testInfo.outputPath("database.sql");
        await db.dump(file);
        await testInfo.attach("database", { path: file, contentType: "application/sql" });
      }
    },
    { auto: true },
  ],
});
