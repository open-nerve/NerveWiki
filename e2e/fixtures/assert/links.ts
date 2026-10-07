import { expect } from "@playwright/test";

import type { Database } from "../db";

// Database assertions of the link stories: the index (M6 design 4.3), kept
// in the write's own transaction, so it is read at once. The page version
// and the API version of a story call the same function (v0.1 design
// 10.1).

/** A link of a page as the index has it: where it is written, what it names, and the page it leads to. */
export interface IndexedLink {
  kind: "wikilink" | "embed" | "link" | "image";
  /** A property link's path ("related.0"); null in the body. */
  property: string | null;
  target: string;
  /** The page it leads to; null for none. */
  resolved: string | null;
}

/** indexed_pages: the page id is indexed at its content's revision, by the extraction it was made with. */
async function expectIndexedAtItsRevision(db: Database, id: string): Promise<void> {
  const rows = await db.query(
    `SELECT i.revision = c.revision AS current, i.extractor >= 1 AS extracted
       FROM indexed_pages i JOIN page_contents c ON c.node_id = i.node_id WHERE i.node_id = $1`,
    [id]
  );
  expect(rows).toEqual([{ current: true, extracted: true }]);
}

/** indexed_pages, page_links: the page sourceId, indexed at its revision, has links, in the order they are written. */
export async function expectIndexedLinks(db: Database, sourceId: string, links: IndexedLink[]): Promise<void> {
  await expectIndexedAtItsRevision(db, sourceId);
  const rows = await db.query(
    `SELECT kind, property_key AS property, target, resolved_id AS resolved
       FROM page_links WHERE source_id = $1 ORDER BY range_start`,
    [sourceId]
  );
  expect(rows).toEqual(links);
}

/** indexed_pages, page_tags: the page sourceId, indexed at its revision, has tags, each written count times. */
export async function expectIndexedTags(
  db: Database,
  sourceId: string,
  tags: { tag: string; count: number }[]
): Promise<void> {
  await expectIndexedAtItsRevision(db, sourceId);
  const rows = await db.query(`SELECT tag, count FROM page_tags WHERE source_id = $1 ORDER BY tag_key`, [sourceId]);
  expect(rows).toEqual(tags);
}

/** indexed_pages, page_aliases: the page sourceId, indexed at its revision, has aliases, in their keys' order. */
export async function expectIndexedAliases(db: Database, sourceId: string, aliases: string[]): Promise<void> {
  await expectIndexedAtItsRevision(db, sourceId);
  const rows = await db.query<{ alias: string }>(
    `SELECT alias FROM page_aliases WHERE source_id = $1 ORDER BY alias_key`,
    [sourceId]
  );
  expect(rows.map((row) => row.alias)).toEqual(aliases);
}
