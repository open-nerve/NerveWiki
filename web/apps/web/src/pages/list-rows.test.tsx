import { screen, within } from "@testing-library/react";
import { expect, test } from "vitest";

import { notebookJSON } from "../test/fakes";
import { changesetId, created, wrote } from "../test/history-server";
import { guide, pageServer } from "../test/page-server";
import { renderApp } from "../test/render";

// M13/P5/S2 (decision 925, review B's P5B-I2): the lists' pages have one kind of row, page after page: its first line
// 14px and 500, the main link in it underlined as it is hovered alone (00-M13 4.7: a list's links are not), the row in a
// card. Each page's row as its own tests make it; jsdom lays nothing out, so these hold the classes, and the rows side
// by side are seen in the screenshots (05-P5 7.4).

const home = `/lab/notebooks/${notebookJSON.id}`;

/** A page's row: the row, its main link. */
type Row = { row: HTMLElement; link: HTMLElement };

/** rowOf is the first row of the list named list, with its first link, once it shows. */
async function rowOf(list: string): Promise<Row> {
  const listed = await screen.findByRole("list", { name: list });
  const row = within(listed).getAllByRole("listitem")[0] as HTMLElement;
  return { row, link: within(row).getAllByRole("link")[0] as HTMLElement };
}

/** classesOf are element's classes. */
const classesOf = (element: Element | null | undefined) => [...(element?.classList ?? [])];

const rows: [string, () => Promise<Row>, ((row: HTMLElement) => Promise<void>)?][] = [
  [
    "a search's hit",
    async () => {
      renderApp("/lab/search?q=Guide", pageServer().app);
      return rowOf("Search results");
    },
  ],
  [
    "the history's changeset, an ingestion's",
    async () => {
      const server = pageServer({ mode: "llm_wiki" });
      server.log("ingest", { changes: [wrote(guide, 1, 2)], ingestCount: 1, ingestSources: [guide.id] });
      renderApp(`${home}/history`, server.app);
      return rowOf("Changes");
    },
    async (row) => {
      // The ingestion's sources in the row's facts: the block no size of its own, its links as the row's.
      const sources = within(row).getByRole("list", { name: "Ingested sources" });
      const source = await within(sources).findByRole("link");
      expect(classesOf(sources.parentElement)).not.toContain("text-sm");
      expect(classesOf(source)).toContain("hover:underline");
      expect(classesOf(source)).not.toContain("underline");
    },
  ],
  [
    "a changeset's change",
    async () => {
      const server = pageServer();
      server.log("one", { id: changesetId(9201), changes: [created(guide)] });
      renderApp(`${home}/history/${changesetId(9201)}`, server.app);
      return rowOf("Nodes of this change");
    },
  ],
];

test.each(rows)(
  "%s is a list's row: its first line 14px and 500, its link underlined as hovered alone, in a card",
  async (_, open, more) => {
    const { row, link } = await open();

    const line = link.parentElement;
    expect(classesOf(line)).toEqual(expect.arrayContaining(["text-sm", "font-medium"]));
    expect(classesOf(link)).toContain("hover:underline");
    expect(classesOf(link)).not.toContain("underline");
    // The list is the card itself, or in one.
    expect(row.closest(".rounded-card")).not.toBeNull();
    await more?.(row);
  }
);
