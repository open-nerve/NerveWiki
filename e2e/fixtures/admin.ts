import { expect } from "@playwright/test";

import type { Database } from "./db";
import { runNervewiki, type RunOptions } from "./server";

// The server administrator's commands, nervewiki users (M1/P4 design 3.7)
// and nervewiki workspaces (M2/P4 design 3.3), on a worker's database: the
// worker's nervewiki sees what they change on its next request.

/** Every log line, so that a password in any of them shows. */
const allLogs = { NWIKI_LOG__LEVEL: "debug" };

/** The start of a log line in the test configuration's text format. */
const logLine = /^time=\S+ level=[A-Z]+ msg=/m;

/**
 * Runs nervewiki users with args, and password, when given, as the line of
 * its standard input, and returns its output: the one line the command
 * prints. A command that sets a password logs what it did, so its logs are
 * there to search: neither the output nor the logs, at every level, hold
 * the password. (A command that changes nothing, such as activating an
 * active account, logs nothing.)
 */
export async function nervewikiUsers(db: Database, args: string[], password?: string): Promise<string> {
  const { stdout, stderr } = await runNervewiki(["users", ...args], db.url, { input: lineOf(password), env: allLogs });
  const label = `nervewiki users ${args.join(" ")}`;
  if (password !== undefined) {
    expect(stderr, `${label} logs`).toMatch(logLine);
    expectNoPassword(label, stdout + stderr, password);
  }
  return stdout;
}

/**
 * Runs nervewiki users with args, and password as nervewikiUsers does, and
 * expects it to fail: exit code 1, no output, and "nervewiki: <message>" as
 * the last line of stderr, which does not hold the password either.
 */
export async function nervewikiUsersFails(
  db: Database,
  args: string[],
  message: string,
  password?: string
): Promise<void> {
  const stderr = await expectFailure(["users", ...args], db, message, { input: lineOf(password), env: allLogs });
  expectNoPassword(`nervewiki users ${args.join(" ")}`, stderr, password);
}

/**
 * Runs nervewiki workspaces with args, and env added to the test
 * configuration, and returns its output: the one line the command prints.
 */
export async function nervewikiWorkspaces(
  db: Database,
  args: string[],
  env: Record<string, string> = {}
): Promise<string> {
  const { stdout } = await runNervewiki(["workspaces", ...args], db.url, { env });
  return stdout;
}

/** Runs nervewiki workspaces as nervewikiWorkspaces does, and expects it to fail as nervewikiUsersFails does. */
export async function nervewikiWorkspacesFails(
  db: Database,
  args: string[],
  message: string,
  env: Record<string, string> = {}
): Promise<void> {
  await expectFailure(["workspaces", ...args], db, message, { env });
}

/**
 * Runs nervewiki with args and expects it to fail: exit code 1, no output,
 * and "nervewiki: <message>" as the last line of stderr, which it returns.
 */
async function expectFailure(args: string[], db: Database, message: string, options: RunOptions): Promise<string> {
  const failure = await runNervewiki(args, db.url, options).then(
    () => undefined,
    (err: { code?: unknown; stdout?: string; stderr?: string }) => err
  );
  const label = `nervewiki ${args.join(" ")}`;
  expect(failure, `${label} fails`).toBeDefined();
  expect(failure?.code, label).toBe(1);
  expect(failure?.stdout, label).toBe("");
  expect(failure?.stderr?.endsWith(`nervewiki: ${message}\n`), `${label}: ${failure?.stderr}`).toBe(true);
  return failure?.stderr ?? "";
}

/** The standard input that gives password: one line; none without a password. */
function lineOf(password?: string): string {
  return password === undefined ? "" : `${password}\n`;
}

/**
 * Fails when text holds password: as is, in hex of either case, or in
 * either base64 alphabet. The unpadded base64 spellings also find the
 * padded ones.
 */
function expectNoPassword(label: string, text: string, password?: string): void {
  if (!password) {
    return;
  }
  const bytes = Buffer.from(password);
  const hex = bytes.toString("hex");
  for (const spelling of [
    password,
    hex,
    hex.toUpperCase(),
    bytes.toString("base64").replace(/=+$/, ""),
    bytes.toString("base64url"),
  ]) {
    expect(text.includes(spelling), `${label}: the output or the logs hold the password as ${spelling}`).toBe(false);
  }
}
