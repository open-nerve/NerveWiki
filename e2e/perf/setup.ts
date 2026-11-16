import { generateKeyPairSync } from "node:crypto";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import { setTimeout as sleep } from "node:timers/promises";

import { createClient, type ApiClient } from "@nervewiki/api-client";

import { bearer, createToken, login, onboardingSteps, password } from "../fixtures/auth";
import { createDatabase, openDatabase, startPostgres } from "../fixtures/db";
import { accept, invite } from "../fixtures/invitations";
import { createNotebook } from "../fixtures/notebooks";
import { requireBinary, runNervewiki, startNervewiki, type Nervewiki } from "../fixtures/server";
import { startImport } from "../fixtures/transfer";
import { createWorkspace } from "../fixtures/workspaces";
import { record, roundDir } from "./record";
import { isShapeName, seed, sha256, shapes, type ShapeName } from "./shapes";

// One round of perf's preparation (M12/P4 design 4.2): a PostgreSQL of its own (no fsync=off), a production serve
// on it (make build's binary, NWIKI_ENV=prod), two accounts done with onboarding in one workspace, and a notebook per
// shape that NW_PERF_SHAPES names (flat, deep and attachments, the page tree's, by default), each imported, then the
// background left quiet. A round is one run: cd e2e && pnpm perf [file], through heavy.sh. Its results go to
// NW_PERF_OUT (record.ts), its serve's log and storage and perf-env.json beside them.

/** What the tests of the round read of its preparation. */
export interface PerfEnv {
  readonly baseURL: string;
  /** The first account: the workspace's admin, who creates every notebook; the tabs measured are its. */
  readonly admin: string;
  /** The second: a member of the workspace, the "other account" of T2. */
  readonly other: string;
  /** Both accounts' password. */
  readonly password: string;
  readonly workspace: string;
  readonly notebooks: Partial<Record<ShapeName, { readonly id: string } & ShapeFacts>>;
}

/** What a test needs of a shape besides its notebook's id. */
type ShapeFacts = { readonly notebook: string; readonly roots: number; readonly big?: string };

/** The round's preparation, read by a test: each logs in itself, an access token lasting 15 minutes. */
export function perfEnv(): PerfEnv {
  return JSON.parse(readFileSync(envFile(), "utf8")) as PerfEnv;
}

function envFile(): string {
  return path.join(roundDir(), "perf-env.json");
}

const admin = "perf-admin@example.com";
const other = "perf-other@example.com";
/** How long an import may take, polled every importPollMs. */
const importMs = 60 * 60_000;
const importPollMs = 2_000;
/** How long the background may take to go quiet, and how long it stays so before the round starts. */
const quietMs = 60 * 60_000;
const quietForMs = 30_000;

export default async function setup(): Promise<() => Promise<void>> {
  requireBinary();
  // Empty: no notebook, for what makes its own (the editor's keys of B9).
  const names = (process.env.NW_PERF_SHAPES ?? "flat,deep,attachments")
    .split(",")
    .map((name) => name.trim())
    .filter((name) => name !== "");
  const unknown = names.filter((name) => !isShapeName(name));
  if (unknown.length > 0) {
    throw new Error(`NW_PERF_SHAPES names no shape ${unknown.join(", ")}: one of ${Object.keys(shapes).join(", ")}`);
  }
  const dir = roundDir();
  mkdirSync(dir, { recursive: true });
  const env = serveEnv(dir);
  record("round", {
    shapes: names,
    seed,
    cpus: os.cpus().length,
    loadavg: os.loadavg(),
    totalmem: os.totalmem(),
    freemem: os.freemem(),
  });
  const postgres = await startPostgres();
  let server: Nervewiki | undefined;
  const stop = async () => {
    await server?.stop();
    await postgres.stop();
  };
  try {
    const url = await createDatabase("perf");
    await runNervewiki(["migrate", "up"], url, { env });
    for (const email of [admin, other]) {
      // oxlint-disable-next-line no-await-in-loop -- one command at a time, each its own process
      await runNervewiki(["users", "create", "--email", email], url, { input: `${password}\n`, env });
    }
    const version = (await runNervewiki(["version"], url, { env })).stdout.trim();
    server = await startNervewiki(url, path.join(dir, "nervewiki.log"), { env });
    const api = createClient({ baseUrl: server.baseURL });
    const [adminTokens, otherTokens] = await Promise.all([onboarded(api, admin), onboarded(api, other)]);
    // A token of the setup's own: the imports and the quiet outlast an access token's 15 minutes.
    const pat = (await createToken(api, adminTokens.access_token, { name: "perf setup" })).token;
    const workspace = await createWorkspace(api, pat, "Perf", "perf");
    await accept(api, otherTokens.access_token, await invite(api, pat, workspace.slug, other, "member"));
    const notebooks: Record<string, { id: string } & ShapeFacts> = {};
    for (const name of names) {
      // oxlint-disable-next-line no-await-in-loop -- the server imports one at a time (jobs.import_workers)
      notebooks[name] = await importShape(api, pat, workspace.slug, name as ShapeName);
    }
    const db = openDatabase("perf");
    try {
      const [database] = await db.query<{ version: string }>("SELECT version()");
      record("versions", { nervewiki: version, postgres: database?.version });
      await quiet((sql) => db.query<Left>(sql));
    } finally {
      await db.close();
    }
    const perf: PerfEnv = { baseURL: server.baseURL, admin, other, password, workspace: workspace.slug, notebooks };
    writeFileSync(envFile(), `${JSON.stringify(perf, null, 2)}\n`);
  } catch (error) {
    await stop();
    throw error;
  }
  return stop;
}

/**
 * The production configuration of serve, with what the setup needs besides (design 4.2, decision 460): a JWT key of
 * the round's, written into its directory; a text log, which startNervewiki waits on; and the three buckets the
 * logins and token creations take, at 600 a minute, none of them on a path measured.
 */
function serveEnv(dir: string): Record<string, string> {
  const key = path.join(dir, "jwt.pem");
  const { privateKey } = generateKeyPairSync("ed25519");
  writeFileSync(key, privateKey.export({ type: "pkcs8", format: "pem" }), { mode: 0o600 });
  return {
    NWIKI_ENV: "prod",
    NWIKI_AUTH__JWT__PRIVATE_KEY_FILE: key,
    NWIKI_LOG__FORMAT: "text",
    NWIKI_RATELIMIT__PASSWORD_USER__PER_MINUTE: "600",
    NWIKI_RATELIMIT__LOGIN_IP__PER_MINUTE: "600",
    NWIKI_RATELIMIT__LOGIN_IP_EMAIL__PER_MINUTE: "600",
  };
}

/** Logs email in and records the web app's onboarding steps for it, as registerOnboarded does: users create does not. */
async function onboarded(api: ApiClient, email: string) {
  const tokens = await login(api, email);
  for (const step of onboardingSteps) {
    // oxlint-disable-next-line no-await-in-loop -- the steps in their order
    const { response, error } = await api.POST("/api/v0/me/onboarding-steps", {
      body: { step },
      headers: bearer(tokens.access_token),
    });
    if (response.status !== 200) {
      throw new Error(`record ${step} for ${email}: ${response.status} ${JSON.stringify(error)}`);
    }
  }
  return tokens;
}

/** Creates the notebook of the shape name, open to the workspace's members as editors, and imports its archives. */
async function importShape(api: ApiClient, pat: string, slug: string, name: ShapeName, title = shapes[name].notebook) {
  const shape = shapes[name];
  const notebook = await createNotebook(api, pat, slug, title, "editor");
  const archives = shape.archives();
  for (const [i, archive] of archives.entries()) {
    const started = Date.now();
    // oxlint-disable-next-line no-await-in-loop -- one import after another into the same notebook
    const job = await startImport(api, pat, notebook.id, archive);
    // oxlint-disable-next-line no-await-in-loop -- the same
    const state = await jobEnded(api, pat, job.id);
    record("import", {
      shape: name,
      notebook: title,
      archive: i,
      bytes: archive.length,
      sha256: sha256(archive),
      ms: Date.now() - started,
      state,
    });
    if (state !== "succeeded") {
      throw new Error(`the import ${i} of ${name} ended ${state}`);
    }
  }
  return { id: notebook.id, notebook: title, roots: shape.roots, big: shape.big };
}

/**
 * A notebook of the shape name, titled title, imported with credential (a token of the first account's) during a
 * test, and the background quiet again after it, as the setup leaves it: for a measure that wants one of its own
 * each time (the attachment list's rereads, design 4.7).
 */
export async function importNotebook(
  api: ApiClient,
  credential: string,
  slug: string,
  name: ShapeName,
  title: string
): Promise<{ id: string } & ShapeFacts> {
  const notebook = await importShape(api, credential, slug, name, title);
  const db = openDatabase("perf");
  try {
    await quiet((sql) => db.query<Left>(sql));
  } finally {
    await db.close();
  }
  return notebook;
}

/** The state the job id ends in, polled every importPollMs, within importMs. */
async function jobEnded(api: ApiClient, credential: string, id: string): Promise<string> {
  const deadline = Date.now() + importMs;
  for (;;) {
    // oxlint-disable-next-line no-await-in-loop -- polled until the job ends
    const { data, response } = await api.GET("/api/v0/transfer-jobs/{job_id}", {
      params: { path: { job_id: id } },
      headers: bearer(credential),
    });
    if (response.status !== 200 || data === undefined) {
      throw new Error(`read the job ${id}: ${response.status}`);
    }
    if (data.state !== "queued" && data.state !== "running") {
      return data.state;
    }
    if (Date.now() > deadline) {
      throw new Error(`the job ${id} did not end within ${importMs} ms`);
    }
    // oxlint-disable-next-line no-await-in-loop -- the same
    await sleep(importPollMs);
  }
}

/** What is left in the background: the pages the search's queue holds, and River's jobs running or available. */
type Left = { queued: number; jobs: number };

/**
 * Waits until the search's queue is empty and River has no job running or available, both at one look, then
 * quietForMs more: an import leaves its pages to the search's job, whose CPU a measure would take in (M8: 10,000
 * pages in about 2 minutes). The periodic jobs come and go every few seconds; a look between them sees none.
 */
async function quiet(query: (sql: string) => Promise<Left[]>): Promise<void> {
  const started = Date.now();
  for (;;) {
    // oxlint-disable-next-line no-await-in-loop -- polled until quiet
    const [left] = await query(
      "SELECT (SELECT count(*) FROM page_search_queue)::int AS queued, " +
        "(SELECT count(*) FROM river_job WHERE state IN ('running', 'available'))::int AS jobs"
    );
    if (left !== undefined && left.queued === 0 && left.jobs === 0) {
      break;
    }
    if (Date.now() - started > quietMs) {
      throw new Error(`the background was not quiet within ${quietMs} ms: ${JSON.stringify(left)}`);
    }
    // oxlint-disable-next-line no-await-in-loop -- the same
    await sleep(1_000);
  }
  record("quiet", { ms: Date.now() - started, idleMs: quietForMs });
  await sleep(quietForMs);
}
