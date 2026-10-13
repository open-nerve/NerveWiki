/** A page of a list read a page at a time: the cursor of the next, null after the last. */
type ListPage = { next_cursor: string | null };

/**
 * readPages reads count pages of a list, from the first, fewer when the
 * list ends: read reads the page after cursor, the first without one. A
 * list whose items change or leave is read again so (v0.1 design 13.2,
 * item 19; M8 design 4.13): as many pages as were read, More's among them,
 * from the first, so that it is as long as it was. The trash's list reads
 * through it; the backlinks' and the versions' have their own, until
 * v0.1's polish.
 */
export async function readPages<T extends ListPage>(
  read: (cursor?: string) => Promise<T>,
  count: number
): Promise<T[]> {
  const pages = [await read()];
  for (let cursor = pages[0]?.next_cursor; cursor && pages.length < count; cursor = pages.at(-1)?.next_cursor) {
    // oxlint-disable-next-line no-await-in-loop -- each page after the one before
    pages.push(await read(cursor));
  }
  return pages;
}
