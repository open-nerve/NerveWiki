import path from "node:path";

import { createClient, type ApiClient } from "@nervewiki/api-client";
import { test as base } from "@playwright/test";

import { createDatabase, dropDatabase, openDatabase, templateDatabase, type Database } from "./db";
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
