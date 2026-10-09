import { createHash } from "node:crypto";
import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import path from "node:path";

import { expect } from "@playwright/test";

import type { Database } from "../db";
import { blobPath } from "./asset";

// Database and store assertions of the export and import stories (M7/P5
// design 3.16, P6 design 3.18; v0.1 design 13.4, item 3): the page version
// and the API version of a story call the same function (v0.1 design
// 10.1).

/**
 * archivePath is where the store at storageDir keeps the archive of the job id of kind: under exports/ or imports/,
 * in the shards of the first two bytes of the SHA-256 of its name (storage.Local).
 */
function archivePath(storageDir: string, id: string, kind: "export" | "import" = "export"): string {
  const name = `${id}.zip`;
  const hash = createHash("sha256").update(name).digest("hex");
  return path.join(storageDir, `${kind}s`, hash.slice(0, 2), hash.slice(2, 4), name);
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

/**
 * transfer_jobs and the store at storageDir: the export id has expired, and its archive is no longer in the store,
 * deleted a moment after the expiry commits.
 */
export async function expectExpired(db: Database, storageDir: string, id: string): Promise<void> {
  expect(
    await db.query("SELECT state, result_bytes IS NOT NULL AS bytes FROM transfer_jobs WHERE id = $1", [id])
  ).toEqual([{ state: "expired", bytes: true }]);
  await expect
    .poll(() => existsSync(archivePath(storageDir, id)), { message: `the archive of ${id} in the store` })
    .toBe(false);
}

/** What an import wrote and skipped, as its report counts it. */
export interface ImportCounts {
  pages: number;
  attachments: number;
  renamed: number;
  skipped: number;
}

/**
 * transfer_jobs, changesets, changeset_items and the store at storageDir: the job id is creatorId's import from
 * client, under root (null for the notebook's root), ended as state with failure, done of its total nodes written, its
 * report the counts, started before it ended; the nodes it wrote are the items of one changeset of the import's kind,
 * creatorId's from client, made while it ran, none when it wrote none; its archive is no longer in the store.
 */
export async function expectImported(
  db: Database,
  storageDir: string,
  id: string,
  expected: {
    creatorId: string;
    client: string;
    root: string | null;
    state: "succeeded" | "failed" | "cancelled";
    failure: string | null;
    done: number;
    total: number;
    counts: ImportCounts;
  }
): Promise<void> {
  const rows = await db.query(
    `SELECT kind, state, client, root_id, created_by_id = $2 AS by_creator, progress_done::int AS done,
            progress_total::int AS total, result_bytes, report->>'failure' AS failure,
            (report->'counts'->>'pages')::int AS pages, (report->'counts'->>'attachments')::int AS attachments,
            (report->'counts'->>'renamed')::int AS renamed, (report->'counts'->>'skipped')::int AS skipped,
            (report->'counts'->>'missing')::int AS missing,
            created_at <= started_at AND started_at <= finished_at AS ordered, deleted_at IS NULL AS live
       FROM transfer_jobs WHERE id = $1`,
    [id, expected.creatorId]
  );
  expect(rows).toEqual([
    {
      kind: "import",
      state: expected.state,
      client: expected.client,
      root_id: expected.root,
      by_creator: true,
      done: expected.done,
      total: expected.total,
      result_bytes: null,
      failure: expected.failure,
      ...expected.counts,
      missing: 0,
      ordered: true,
      live: true,
    },
  ]);
  const changesets = await db.query(
    `SELECT s.client, s.created_by_id = $2 AS by_creator, count(i.id)::int AS items
       FROM transfer_jobs j JOIN changesets s ON s.notebook_id = j.notebook_id AND s.kind = 'import'
            AND s.created_at BETWEEN j.started_at AND j.finished_at
       JOIN changeset_items i ON i.changeset_id = s.id
      WHERE j.id = $1 GROUP BY s.id`,
    [id, expected.creatorId]
  );
  expect(changesets).toEqual(
    expected.done === 0 ? [] : [{ client: expected.client, by_creator: true, items: expected.done }]
  );
  await expect
    .poll(() => existsSync(archivePath(storageDir, id, "import")), { message: `the archive of ${id} in the store` })
    .toBe(false);
}

/** The files the store at storageDir keeps under imports/, those still being written left out. */
export function storedImports(storageDir: string): string[] {
  const dir = path.join(storageDir, "imports");
  if (!existsSync(dir)) {
    return [];
  }
  return readdirSync(dir, { recursive: true, withFileTypes: true })
    .filter((e) => e.isFile() && !path.relative(dir, e.parentPath).split(path.sep).includes(".tmp"))
    .map((e) => path.join(e.parentPath, e.name));
}

/** A node of a notebook as the import stories read it: its path from the root, its kind, its content or its file. */
export interface TreeEntry {
  path: string;
  kind: "page" | "asset";
  /** A page's content. */
  content?: string;
  /** An attachment's type, and its file's SHA-256 as the store holds it, in hex. */
  mime?: string;
  sha256?: string;
}

/** A row of a notebook's tree, its path from the root. */
interface TreeRow {
  id: string;
  path: string;
  kind: "page" | "asset";
  content: string | null;
  blob: string | null;
  mime: string | null;
}

/** nodes, page_contents, asset_blobs: the notebook notebookId's nodes, depth first in their order. */
async function rowsOf(db: Database, notebookId: string): Promise<TreeRow[]> {
  const rows = await db.query<Omit<TreeRow, "path"> & { parent_id: string | null; name: string }>(
    `SELECT n.id::text, n.parent_id::text, n.kind, n.name, c.content, b.id::text AS blob, b.mime
       FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id
            LEFT JOIN asset_blobs b ON b.node_id = n.id AND b.deleted_at IS NULL
      WHERE n.notebook_id = $1 AND n.deleted_at IS NULL ORDER BY n.sort_order, n.id`,
    [notebookId]
  );
  const out: TreeRow[] = [];
  const walk = (parent: string | null, prefix: string) => {
    for (const { parent_id, name, ...r } of rows.filter((x) => x.parent_id === parent)) {
      out.push({ ...r, path: prefix + name });
      walk(r.id, `${prefix}${name}/`);
    }
  };
  walk(null, "");
  return out;
}

/** The store at storageDir and the notebook notebookId's nodes, depth first in their order, as TreeEntry. */
export async function treeOf(db: Database, storageDir: string, notebookId: string): Promise<TreeEntry[]> {
  return (await rowsOf(db, notebookId)).map((r) =>
    r.kind === "page"
      ? { path: r.path, kind: r.kind, content: r.content ?? "" }
      : {
          path: r.path,
          kind: r.kind,
          mime: r.mime ?? "",
          sha256: r.blob === null ? "" : sha256Of(readFileSync(blobPath(storageDir, r.blob))),
        }
  );
}

/** The ids of the notebook notebookId's nodes, by their paths from the root. */
export async function idsOf(db: Database, notebookId: string): Promise<Map<string, string>> {
  return new Map((await rowsOf(db, notebookId)).map((r) => [r.path, r.id]));
}

/** The SHA-256 of bytes, in hex. */
export function sha256Of(bytes: Uint8Array): string {
  return createHash("sha256").update(bytes).digest("hex");
}
