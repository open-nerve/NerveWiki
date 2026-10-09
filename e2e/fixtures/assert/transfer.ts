import { createHash } from "node:crypto";
import { existsSync, statSync } from "node:fs";
import path from "node:path";

import { expect } from "@playwright/test";

import type { Database } from "../db";

// Database and store assertions of the export stories (M7/P5 design 3.16;
// v0.1 design 13.4, item 3): the page version and the API version of a
// story call the same function (v0.1 design 10.1).

/**
 * archivePath is where the store at storageDir keeps the archive of the export id: under exports/, in the shards of
 * the first two bytes of the SHA-256 of its name (storage.Local).
 */
function archivePath(storageDir: string, id: string): string {
  const name = `${id}.zip`;
  const hash = createHash("sha256").update(name).digest("hex");
  return path.join(storageDir, "exports", hash.slice(0, 2), hash.slice(2, 4), name);
}

/** What an export that succeeded wrote, as its report counts it. */
export interface ExportCounts {
  pages: number;
  attachments: number;
  renamed: number;
  missing: number;
}

/**
 * transfer_jobs and the store at storageDir: the job id is creatorId's export from client, of root (null for the whole
 * notebook), succeeded with nodes of its nodes done, its report the counts and no failure, started before it ended;
 * its archive is in the store, of its row's bytes, which are bytes.
 */
export async function expectExported(
  db: Database,
  storageDir: string,
  id: string,
  expected: {
    creatorId: string;
    client: string;
    root: string | null;
    nodes: number;
    counts: ExportCounts;
    bytes: number;
  }
): Promise<void> {
  const rows = await db.query(
    `SELECT kind, state, client, root_id, created_by_id = $2 AS by_creator, progress_done::int AS done,
            progress_total::int AS total, result_bytes::int AS result_bytes, report->'failure' AS failure,
            (report->'counts'->>'pages')::int AS pages, (report->'counts'->>'attachments')::int AS attachments,
            (report->'counts'->>'renamed')::int AS renamed, (report->'counts'->>'missing')::int AS missing,
            created_at <= started_at AND started_at <= finished_at AS ordered, deleted_at IS NULL AS live
       FROM transfer_jobs WHERE id = $1`,
    [id, expected.creatorId]
  );
  expect(rows).toEqual([
    {
      kind: "export",
      state: "succeeded",
      client: expected.client,
      root_id: expected.root,
      by_creator: true,
      done: expected.nodes,
      total: expected.nodes,
      result_bytes: expected.bytes,
      failure: null,
      ...expected.counts,
      ordered: true,
      live: true,
    },
  ]);
  const file = archivePath(storageDir, id);
  expect(existsSync(file), `the archive of ${id} in the store`).toBe(true);
  expect(statSync(file).size).toBe(expected.bytes);
}

/** transfer_jobs and the store at storageDir: the export id has expired, and its archive is no longer in the store. */
export async function expectExpired(db: Database, storageDir: string, id: string): Promise<void> {
  expect(
    await db.query("SELECT state, result_bytes IS NOT NULL AS bytes FROM transfer_jobs WHERE id = $1", [id])
  ).toEqual([{ state: "expired", bytes: true }]);
  expect(existsSync(archivePath(storageDir, id)), `the archive of ${id} in the store`).toBe(false);
}
