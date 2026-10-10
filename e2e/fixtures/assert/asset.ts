import { createHash } from "node:crypto";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import path from "node:path";

import type { Asset, Page } from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import { blobOf } from "../assets";
import type { Database } from "../db";
import { expectIndexedLinks } from "./links";
import { expectContentWritten } from "./page";

// Database and store assertions of the attachment stories (M7/P2 design
// 3.13), by table: the page version and the API version of a story call
// the same function (v0.1 design 10.1). The times are compared in SQL.

/**
 * blobPath is where the store at storageDir keeps the file of the blob id: under blobs/, in the shards of the first
 * two bytes of the SHA-256 of its name (storage.Local).
 */
export function blobPath(storageDir: string, id: string): string {
  const hash = createHash("sha256").update(id).digest("hex");
  return path.join(storageDir, "blobs", hash.slice(0, 2), hash.slice(2, 4), id);
}

/**
 * nodes, changesets, changeset_items, asset_blobs and the store at storageDir: a, as the API answered it, is an
 * attachment node uploaded by creatorId from client, in a changeset of its own with its item and no version; its row
 * has its type, size, SHA-256 and image size, as the answer has them, at the node's time; its file holds bytes.
 */
export async function expectUploaded(
  db: Database,
  storageDir: string,
  a: Asset,
  bytes: Uint8Array,
  creatorId: string,
  client: string
): Promise<void> {
  const nodes = await db.query(
    `SELECT n.notebook_id, n.parent_id, n.kind, n.name, n.created_by_id = $2 AND n.updated_by_id = $2 AS by_creator,
            n.created_at = $3::timestamptz AND n.updated_at = n.created_at AS at_answer, n.deleted_at IS NULL AS live,
            NOT EXISTS (SELECT 1 FROM page_contents c WHERE c.node_id = n.id) AS contentless
       FROM nodes n WHERE n.id = $1`,
    [a.id, creatorId, a.created_at]
  );
  expect(nodes).toEqual([
    {
      notebook_id: a.notebook_id,
      parent_id: a.parent_id,
      kind: "asset",
      name: a.name,
      by_creator: true,
      at_answer: true,
      live: true,
      contentless: true,
    },
  ]);
  const changesets = await db.query(
    `SELECT s.kind, s.client, s.created_by_id = $2 AS by_creator, s.created_at = $3::timestamptz AS at_creation,
            i.before_name IS NULL AND i.after_name = $4 AS item,
            NOT EXISTS (SELECT 1 FROM page_revisions r WHERE r.changeset_id = s.id) AS versionless
       FROM changesets s JOIN changeset_items i ON i.changeset_id = s.id WHERE i.node_id = $1`,
    [a.id, creatorId, a.created_at, a.name]
  );
  expect(changesets).toEqual([
    { kind: "edit", client, by_creator: true, at_creation: true, item: true, versionless: true },
  ]);
  const blob = blobOf(a);
  const rows = await db.query(
    `SELECT b.node_id, b.notebook_id, b.mime, b.byte_size::int AS byte_size, encode(b.sha256, 'hex') AS sha256,
            b.width, b.height, b.created_by_id = $3 AS by_creator, b.created_at = n.created_at AS at_node,
            b.deleted_at IS NULL AS live
       FROM asset_blobs b JOIN nodes n ON n.id = b.node_id WHERE b.id = $1 AND b.node_id = $2`,
    [blob, a.id, creatorId]
  );
  expect(rows).toEqual([
    {
      node_id: a.id,
      notebook_id: a.notebook_id,
      mime: a.mime,
      byte_size: bytes.length,
      sha256: createHash("sha256").update(bytes).digest("hex"),
      width: a.width,
      height: a.height,
      by_creator: true,
      at_node: true,
      live: true,
    },
  ]);
  expect(a.byte_size).toBe(bytes.length);
  expect(a.sha256).toBe(rows[0]?.sha256);
  expect(Buffer.compare(readFileSync(blobPath(storageDir, blob)), Buffer.from(bytes)), "the file in the store").toBe(0);
}

/** The files the store at storageDir keeps under blobs/, those being written (.tmp/) among them, sorted. */
export function storedBlobs(storageDir: string): string[] {
  const dir = path.join(storageDir, "blobs");
  if (!existsSync(dir)) {
    return [];
  }
  return readdirSync(dir, { recursive: true, withFileTypes: true })
    .filter((e) => e.isFile())
    .map((e) => path.relative(dir, path.join(e.parentPath, e.name)))
    .toSorted();
}

/** asset_blobs: the rows of the attachments ids are deleted at their nodes' time, each node deleted. */
export async function expectAssetsDeletedWithNodes(db: Database, ids: readonly string[]): Promise<void> {
  const rows = await db.query<{ node_id: string }>(
    `SELECT b.node_id FROM asset_blobs b JOIN nodes n ON n.id = b.node_id
      WHERE b.node_id = ANY($1::uuid[]) AND n.deleted_at IS NOT NULL AND b.deleted_at = n.deleted_at
      ORDER BY b.node_id`,
    [ids]
  );
  expect(rows.map((row) => row.node_id)).toEqual(ids.toSorted());
}

/** asset_blobs: the rows of the attachments ids are deleted at the time of the notebook notebookId's deletion. */
export async function expectAssetsDeletedWithNotebook(
  db: Database,
  notebookId: string,
  ids: readonly string[]
): Promise<void> {
  const rows = await db.query<{ node_id: string }>(
    `SELECT b.node_id FROM asset_blobs b JOIN notebooks n ON n.id = b.notebook_id
      WHERE n.id = $1 AND b.node_id = ANY($2::uuid[]) AND n.deleted_at IS NOT NULL AND b.deleted_at = n.deleted_at
      ORDER BY b.node_id`,
    [notebookId, ids]
  );
  expect(rows.map((row) => row.node_id)).toEqual(ids.toSorted());
}

/** asset_blobs and the store at storageDir: the attachments as are gone, their rows and their files. */
export async function expectAssetsPurged(db: Database, storageDir: string, as: readonly Asset[]): Promise<void> {
  const blobs = as.map(blobOf);
  const rows = await db.query("SELECT id FROM asset_blobs WHERE id = ANY($1::uuid[])", [blobs]);
  expect(rows, "the purged attachments' rows").toEqual([]);
  expect(
    blobs.filter((blob) => existsSync(blobPath(storageDir, blob))),
    "the purged attachments' files"
  ).toEqual([]);
}

/** Embedded is a page's content written, as the API answered its write, and the attachments it embeds, in order. */
export interface Embedded {
  written: Page;
  content: string;
  attachments: readonly Asset[];
}

/**
 * page_contents, nodes, asset_blobs, page_links and the store at storageDir: embedded's page holds its content,
 * written by writerId; each of its attachments was uploaded under the page by writerId from client, its file holding
 * bytes; and the index leads each embed to its attachment (M7 design 9, AS2).
 */
export async function expectEmbedded(
  db: Database,
  storageDir: string,
  { written, content, attachments }: Embedded,
  bytes: Uint8Array,
  writerId: string,
  client: string
): Promise<void> {
  await expectContentWritten(db, written, content, writerId);
  for (const asset of attachments) {
    // oxlint-disable-next-line no-await-in-loop -- one at a time
    await expectUploaded(db, storageDir, asset, bytes, writerId, client);
    expect(asset.parent_id).toBe(written.id);
  }
  await expectIndexedLinks(
    db,
    written.id,
    attachments.map((asset) => ({ kind: "embed", property: null, target: asset.link ?? "", resolved: asset.id }))
  );
}
