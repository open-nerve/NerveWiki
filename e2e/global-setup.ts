import { execFile } from "node:child_process";
import { setTimeout as sleep } from "node:timers/promises";
import { promisify } from "node:util";

import { createDatabase, startPostgres, templateDatabase } from "./fixtures/db";
import { requireBinary, runNervewiki } from "./fixtures/server";

/**
 * Starts PostgreSQL once per run and migrates the template database with
 * bin/nervewiki migrate up; every worker copies the template (fixtures/test.ts).
 * The workers start once the host's network has settled too (networkSettled).
 * The returned function is the global teardown.
 */
export default async function globalSetup(): Promise<() => Promise<void>> {
  requireBinary();
  const postgres = await startPostgres();
  const started = Date.now();
  await Promise.all([
    networkSettled(started),
    createDatabase(templateDatabase).then((url) => runNervewiki(["migrate", "up"], url)),
  ]);
  return postgres.stop;
}

const run = promisify(execFile);
/** How long networkSettled waits at most for the host's tentative addresses. */
const settleMs = 5_000;
/** How long after PostgreSQL answers its container has settled, where ip cannot tell. */
const settledAfterMs = 3_000;
/** How long one look of ip may take. */
const probeMs = 1_000;

/**
 * networkSettled waits until the host has no IPv6 address still tentative
 * (ip -6 addr show tentative -dadfailed prints none), settleMs at most. The
 * container's veth on the host gains its link-local address about 1.4–2 s
 * after the container starts, its duplicate address detection done; a
 * fresh browser loading its first pages then has them fail with
 * net::ERR_NETWORK_CHANGED: the first pages of M10's two runs (the M10/P1/S4
 * and M10/P3/S2 reviews) and of M11/P1/S4's, as its review reproduced
 * (reviewer C, s4-review/c/netchange/). An address whose detection failed
 * stays tentative for good, flagged dadfailed: it is not waited for. Where
 * ip cannot tell, or takes over probeMs, it waits until settledAfterMs
 * after started, when startPostgres returned, the container up by then.
 */
async function networkSettled(started: number): Promise<void> {
  const deadline = Date.now() + settleMs;
  let addresses = await tentative();
  while (addresses !== undefined && addresses !== "" && Date.now() < deadline) {
    // oxlint-disable-next-line no-await-in-loop -- polled until none is tentative
    await sleep(100);
    // oxlint-disable-next-line no-await-in-loop -- polled until none is tentative
    addresses = await tentative();
  }
  if (addresses === undefined) {
    await sleep(Math.max(0, started + settledAfterMs - Date.now()));
  }
}

/** tentative is what ip prints of the host's IPv6 addresses still tentative; undefined where it cannot tell. */
async function tentative(): Promise<string | undefined> {
  try {
    return (await run("ip", ["-6", "addr", "show", "tentative", "-dadfailed"], { timeout: probeMs })).stdout.trim();
  } catch {
    return undefined;
  }
}
