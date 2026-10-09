import { execFile } from "node:child_process";
import { writeFile } from "node:fs/promises";
import { promisify } from "node:util";

import { PostgreSqlContainer } from "@testcontainers/postgresql";
import { Client, Pool, escapeIdentifier, type QueryResultRow } from "pg";

/**
 * The PostgreSQL image of the development database (deploy/compose.dev.yaml),
 * initialised the same way: nervewiki refuses a database whose locale is not
 * builtin C.UTF-8.
 */
const image = "postgres:18.6-trixie";
const initdbArgs = "--locale-provider=builtin --locale=C.UTF-8";

/** The database global setup migrates once; every worker gets a copy of it. */
export const templateDatabase = "nervewiki_template";

/** Global setup hands the server's URL and its container to the workers in these variables. */
const serverUrlVariable = "NWIKI_E2E_POSTGRES_URL";
const containerVariable = "NWIKI_E2E_POSTGRES_CONTAINER";

/** A database of this run: nervewiki serves from it, stories assert on it. */
export interface Database {
  readonly url: string;
  /** Runs one statement on this database, through its own pool, and returns the rows. */
  query<Row extends QueryResultRow>(sql: string, params?: unknown[]): Promise<Row[]>;
  /** Writes a plain SQL pg_dump of this database to file, for the artifacts of a failed test. */
  dump(file: string): Promise<void>;
  /** Closes the pool. */
  close(): Promise<void>;
}

/**
 * Starts the PostgreSQL server of this run and publishes its URL and
 * container to the workers. The testcontainers reaper (Ryuk) removes the
 * container if the run dies before stop is called.
 */
export async function startPostgres(): Promise<{ stop: () => Promise<void> }> {
  const container = await new PostgreSqlContainer(image)
    .withEnvironment({ POSTGRES_INITDB_ARGS: initdbArgs })
    // Each worker's nervewiki takes up to database.max_conns (10) and its tests' pool up to 10 more, and a story
    // may start another nervewiki: a worker a core's half takes more than the default 100 on a large machine.
    .withCommand(["postgres", "-c", "max_connections=300"])
    .withDatabase("postgres")
    .start();
  process.env[serverUrlVariable] = databaseUrl(container.getConnectionUri(), "postgres");
  process.env[containerVariable] = container.getId();
  return {
    stop: async () => {
      await container.stop();
    },
  };
}

// pg waits forever by default; a server that accepts connections but never
// answers must fail the query instead of hanging the run. pg_dump runs in
// the container and is killed after dumpTimeoutMs.
const connectTimeoutMs = 10_000;
const queryTimeoutMs = 30_000;
const dumpTimeoutMs = 60_000;

/** Runs one statement on the server's postgres database, as the superuser. */
async function admin(sql: string): Promise<void> {
  const client = new Client({
    connectionString: requireVariable(serverUrlVariable),
    connectionTimeoutMillis: connectTimeoutMs,
    query_timeout: queryTimeoutMs,
  });
  await client.connect();
  try {
    await client.query(sql);
  } finally {
    await client.end();
  }
}

/**
 * Creates database name, as a copy of template when given (otherwise empty,
 * with the server's builtin C.UTF-8 locale), and returns its URL.
 */
export async function createDatabase(name: string, template?: string): Promise<string> {
  const copy = template ? ` TEMPLATE ${escapeIdentifier(template)}` : "";
  await admin(`CREATE DATABASE ${escapeIdentifier(name)}${copy}`);
  return databaseUrl(requireVariable(serverUrlVariable), name);
}

/** Drops database name, closing the connections to it: the database becomes unavailable to whoever used it. */
export async function dropDatabase(name: string): Promise<void> {
  await admin(`DROP DATABASE ${escapeIdentifier(name)} WITH (FORCE)`);
}

/** Opens a pool on database name of the server startPostgres started: a worker's own database. */
export function openDatabase(name: string): Database {
  const serverUrl = requireVariable(serverUrlVariable);
  const url = databaseUrl(serverUrl, name);
  const pool = new Pool({
    connectionString: url,
    max: 2,
    connectionTimeoutMillis: connectTimeoutMs,
    query_timeout: queryTimeoutMs,
  });
  return {
    url,
    query: async <Row extends QueryResultRow>(sql: string, params?: unknown[]) =>
      (await pool.query<Row>(sql, params)).rows,
    dump: (file) => dump(serverUrl, name, file),
    close: () => pool.end(),
  };
}

async function dump(serverUrl: string, name: string, file: string): Promise<void> {
  const user = decodeURIComponent(new URL(serverUrl).username);
  const { stdout } = await promisify(execFile)(
    "docker",
    ["exec", requireVariable(containerVariable), "pg_dump", "--username", user, "--no-owner", "--dbname", name],
    { timeout: dumpTimeoutMs, killSignal: "SIGKILL", maxBuffer: 256 * 1024 * 1024 }
  );
  await writeFile(file, stdout);
}

function requireVariable(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(`${name} is not set: run the stories with playwright test (see global-setup.ts)`);
  }
  return value;
}

function databaseUrl(serverUrl: string, name: string): string {
  const url = new URL(serverUrl);
  url.pathname = `/${name}`;
  url.searchParams.set("sslmode", "disable");
  return url.toString();
}
