import { execFile, spawn, type ChildProcess } from "node:child_process";
import { once } from "node:events";
import { closeSync, existsSync, mkdirSync, openSync, readFileSync, rmSync } from "node:fs";
import path from "node:path";
import { setTimeout as sleep } from "node:timers/promises";
import { promisify } from "node:util";

/** The binary under test: make build compiles it with the web frontend embedded. */
const binary = path.resolve(import.meta.dirname, "../../bin/nervewiki");

/** nervewiki connects with this application_name, so its sessions show in pg_stat_activity. */
export const applicationName = "nervewiki";

const startTimeoutMs = 30_000;
const stopTimeoutMs = 30_000;
const commandTimeoutMs = 60_000;
const pollIntervalMs = 100;

/**
 * The worker-fixture timeout for nervewiki in test.ts: Playwright's default
 * (30 s, shared by setup and teardown) leaves no room for startTimeoutMs and
 * stopTimeoutMs together, so the fixture's own timeouts — and the "(log: …)"
 * errors they produce — could never fire first.
 */
export const nervewikiFixtureTimeoutMs = startTimeoutMs + stopTimeoutMs + 10_000;

/** A nervewiki serve process of this run. */
export interface Nervewiki {
  readonly baseURL: string;
  /** Its storage directory (storage.dir), the attachments' files under blobs/ (M7/P2). */
  readonly storageDir: string;
  /** Sends SIGTERM and waits for nervewiki to exit with code 0. */
  stop(): Promise<void>;
}

export interface StartOptions {
  /** Variables added to the test configuration, e.g. NWIKI_DATABASE__AUTO_MIGRATE=false. */
  env?: Record<string, string>;
  /**
   * What the start waits for: "ready", /readyz answering 200 and the event
   * stream's listener listening (the default); "live", /healthz answering
   * 200, for a nervewiki that is meant not to be ready.
   */
  until?: "ready" | "live";
}

/** Fails with what to do when bin/nervewiki has not been built: global setup calls it first. */
export function requireBinary(): void {
  if (!existsSync(binary)) {
    throw new Error(`${binary} does not exist: run the stories with make e2e, which runs make build first`);
  }
}

export interface RunOptions {
  /** The command's standard input, e.g. the password of nervewiki users create; empty by default. */
  input?: string;
  /** Variables added to the test configuration, e.g. NWIKI_LOG__LEVEL=debug. */
  env?: Record<string, string>;
}

/**
 * Runs a nervewiki command, such as migrate up, with the test configuration
 * on the database at databaseUrl. It rejects when the command exits non-zero,
 * with the exit code as the error's code and its stdout and stderr, and kills
 * it after commandTimeoutMs: global setup runs it before any Playwright
 * timeout applies.
 */
export async function runNervewiki(
  args: string[],
  databaseUrl: string,
  { input = "", env = {} }: RunOptions = {}
): Promise<{ stdout: string; stderr: string }> {
  const run = promisify(execFile)(binary, args, {
    env: { ...nervewikiEnv(databaseUrl), ...env },
    timeout: commandTimeoutMs,
    killSignal: "SIGKILL",
  });
  run.child.stdin?.end(input);
  return run;
}

/**
 * Starts nervewiki serve with the test configuration on the database at
 * databaseUrl and waits until it is ready (or live, see StartOptions).
 * nervewiki listens on a port the system picks and writes its address to
 * server.addr_file, next to logFile, which gets its output; its storage
 * directory is next to it too, emptied first: every server has its own
 * (M7/P1 design 3.5).
 */
export async function startNervewiki(
  databaseUrl: string,
  logFile: string,
  { env = {}, until = "ready" }: StartOptions = {}
): Promise<Nervewiki> {
  mkdirSync(path.dirname(logFile), { recursive: true });
  const addrFile = logFile.replace(/\.log$/, "") + ".addr";
  rmSync(addrFile, { force: true });
  const storageDir = logFile.replace(/\.log$/, "") + ".data";
  rmSync(storageDir, { recursive: true, force: true });
  const log = openSync(logFile, "w");
  const child = spawn(binary, ["serve"], {
    env: {
      ...nervewikiEnv(databaseUrl),
      NWIKI_SERVER__ADDR: "127.0.0.1:0",
      NWIKI_SERVER__ADDR_FILE: addrFile,
      NWIKI_STORAGE__DIR: storageDir,
      ...env,
    },
    stdio: ["ignore", log, log],
  });
  closeSync(log); // the child has its own copy
  // A spawn failure (ENOENT/EACCES) emits "error" instead of "exit"; capture it
  // so waitFor can surface it through the same log-path error below.
  let spawnError: Error | undefined;
  child.once("error", (err) => {
    spawnError = err;
  });
  const probe = until === "ready" ? "/readyz" : "/healthz";
  const deadline = Date.now() + startTimeoutMs;
  try {
    const baseURL = await waitFor(probe, child, addrFile, deadline, () => spawnError);
    if (until === "ready") {
      // /readyz does not wait for the listener: until it listens, a stream is 503 not_ready, which a page's
      // first stream would wait out (README, the event stream).
      await waitForLog(logFile, listening, child, deadline);
    }
    return { baseURL, storageDir, stop: () => stop(child, logFile) };
  } catch (err) {
    await kill(child);
    throw new Error(`nervewiki was not ${until} (log: ${logFile})`, { cause: err });
  }
}

/**
 * The test configuration on the given database, logging at info: the log goes
 * to a file, and a failed test's log shows its requests. The caller's own
 * NWIKI_* variables are left out.
 */
function nervewikiEnv(databaseUrl: string): NodeJS.ProcessEnv {
  const url = new URL(databaseUrl);
  url.searchParams.set("application_name", applicationName);
  const inherited = Object.entries(process.env).filter(([name]) => !name.startsWith("NWIKI_"));
  return {
    ...Object.fromEntries(inherited),
    NWIKI_ENV: "test",
    NWIKI_LOG__LEVEL: "info",
    NWIKI_DATABASE__URL: url.toString(),
  };
}

/** The address nervewiki wrote to addrFile; undefined until it has. nervewiki renames the file into place, so it is never partial. */
function readAddr(addrFile: string): string | undefined {
  try {
    return readFileSync(addrFile, "utf8");
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      return undefined;
    }
    throw err;
  }
}

/** What nervewiki logs once the event stream's listener listens. */
const listening = 'msg="notification listener listening"';

/** waitForLog waits until logFile has text, while child runs, until deadline. */
async function waitForLog(logFile: string, text: string, child: ChildProcess, deadline: number): Promise<void> {
  while (!readFileSync(logFile, "utf8").includes(text)) {
    if (child.exitCode !== null || child.signalCode !== null) {
      throw new Error(`nervewiki exited with ${child.exitCode ?? child.signalCode}`);
    }
    if (Date.now() > deadline) {
      throw new Error(`no ${text} in the log within ${startTimeoutMs} ms`);
    }
    // oxlint-disable-next-line no-await-in-loop -- polled until it is there
    await sleep(pollIntervalMs);
  }
}

/**
 * Waits until nervewiki has written its address and probe there answers 200,
 * and returns the base URL; fails when nervewiki exits, fails to spawn, or
 * the deadline passes. Each request may only use the time left, so a server
 * that accepts the connection but never answers cannot hold the wait past
 * the deadline.
 */
async function waitFor(
  probe: string,
  child: ChildProcess,
  addrFile: string,
  deadline: number,
  spawnError: () => Error | undefined
): Promise<string> {
  const failure = spawnError();
  if (failure) {
    throw failure;
  }
  if (child.exitCode !== null) {
    throw new Error(`nervewiki exited with code ${child.exitCode}`);
  }
  const timeLeft = deadline - Date.now();
  if (timeLeft <= 0) {
    throw new Error(`no 200 from ${probe} within ${startTimeoutMs} ms`);
  }
  const addr = readAddr(addrFile);
  if (addr !== undefined) {
    const baseURL = `http://${addr}`;
    const ok = await fetch(`${baseURL}${probe}`, { signal: AbortSignal.timeout(timeLeft) }).then(
      (res) => res.ok,
      () => false // no answer before the deadline
    );
    if (ok) {
      return baseURL;
    }
  }
  await sleep(pollIntervalMs);
  return waitFor(probe, child, addrFile, deadline, spawnError);
}

/** Kills a nervewiki that never started and waits until it is gone. */
async function kill(child: ChildProcess): Promise<void> {
  // No pid: the spawn failed, so there is no process and no "exit" to wait for.
  if (child.pid === undefined || child.exitCode !== null || child.signalCode !== null) {
    return;
  }
  const exited = once(child, "exit");
  child.kill("SIGKILL");
  await exited;
}

async function stop(child: ChildProcess, logFile: string): Promise<void> {
  if (child.exitCode === null && child.signalCode === null) {
    const exited = once(child, "exit");
    child.kill("SIGTERM");
    const timer = setTimeout(() => child.kill("SIGKILL"), stopTimeoutMs);
    await exited;
    clearTimeout(timer);
  }
  if (child.exitCode !== 0) {
    throw new Error(
      `nervewiki did not exit cleanly: code ${child.exitCode}, signal ${child.signalCode} (log: ${logFile})`
    );
  }
}
