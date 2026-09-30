import { expect } from "@playwright/test";

import type { Database } from "./db";
import { runNervewiki } from "./server";

// The server administrator's commands, nervewiki users (M1/P4 design 3.7),
// on a worker's database: the worker's nervewiki sees what they change on
// its next request.

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
  const failure = await runNervewiki(["users", ...args], db.url, { input: lineOf(password), env: allLogs }).then(
    () => undefined,
    (err: { code?: unknown; stdout?: string; stderr?: string }) => err
  );
  const label = `nervewiki users ${args.join(" ")}`;
  expect(failure, `${label} fails`).toBeDefined();
  expect(failure?.code, label).toBe(1);
  expect(failure?.stdout, label).toBe("");
  expect(failure?.stderr?.endsWith(`nervewiki: ${message}\n`), `${label}: ${failure?.stderr}`).toBe(true);
  expectNoPassword(label, failure?.stderr ?? "", password);
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
