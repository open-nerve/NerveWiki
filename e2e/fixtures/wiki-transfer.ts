import { readFileSync } from "node:fs";

import type { TransferJob } from "@nervewiki/api-client";
import { expect, type Locator, type Page } from "@playwright/test";

import { answerTo } from "./browser";
import { unzip, type ZipEntry } from "./zip";

// The imports and exports as a user works them (M7/P5 design 4.3, 4.5; P6
// design 4.2): the import's dialog sent, or the export's confirmed, the
// job's row among the notebook's recent jobs, an export's archive
// downloaded by the row's link.

/** The row of the job that does what, among the recent jobs page shows: its name begins with it. */
export function jobRow(page: Page, what: string): Locator {
  const escaped = what.replaceAll(/[.*+?^${}()|[\]\\]/g, String.raw`\$&`);
  return page.getByRole("list", { name: "Recent jobs" }).getByRole("listitem", { name: new RegExp(`^${escaped}, `) });
}

/**
 * Confirms the export of the notebook notebookId in its dialog titled title, open on page; resolves the job the
 * server started, queued.
 */
export async function exportWith(page: Page, notebookId: string, title: string): Promise<TransferJob> {
  const answer = answerTo(page, "POST", `/api/v0/notebooks/${notebookId}/exports`);
  await page.getByRole("alertdialog", { name: title }).getByRole("button", { name: "Export", exact: true }).click();
  const response = await answer;
  expect(response.status(), "the export").toBe(202);
  return (await response.json()) as TransferJob;
}

/**
 * Imports archive, a zip named name, into the notebook notebookId by the dialog of its settings, open on page: under
 * the page the place's select names place, or at the root; resolves the job the server started, queued.
 */
export async function importWith(
  page: Page,
  notebookId: string,
  archive: Buffer,
  { name = "vault.zip", place }: { name?: string; place?: string } = {}
): Promise<TransferJob> {
  await page.getByRole("button", { name: "Import a zip…" }).click();
  const dialog = page.getByRole("dialog", { name: /^Import into / });
  await dialog.getByLabel("Zip archive").setInputFiles({ name, mimeType: "application/zip", buffer: archive });
  if (place !== undefined) {
    await dialog.getByLabel("Import under").selectOption({ label: place });
  }
  const answer = answerTo(page, "POST", `/api/v0/notebooks/${notebookId}/imports`);
  await dialog.getByRole("button", { name: "Import", exact: true }).click();
  const response = await answer;
  expect(response.status(), "the import").toBe(202);
  return (await response.json()) as TransferJob;
}

/**
 * Opens the report of the row's job by its control: its counts' terms and values, and its problems, as locators
 * the stories expect of (they are read as the report shows).
 */
export async function openReport(row: Locator): Promise<{ terms: Locator; counts: Locator; problems: Locator }> {
  await row.getByRole("button", { name: /^Report on / }).click();
  return { terms: row.locator("dt"), counts: row.locator("dd"), problems: row.getByRole("list").getByRole("listitem") };
}

/**
 * Waits for the row's job to succeed, then downloads its archive by its link: the file's name, entries and size,
 * and its bytes.
 */
export async function downloadFrom(
  page: Page,
  row: Locator
): Promise<{ name: string; entries: ZipEntry[]; bytes: number; archive: Buffer }> {
  await expect(row.getByText("Done", { exact: true })).toBeVisible({ timeout: 15_000 });
  const downloading = page.waitForEvent("download");
  await row.getByRole("link", { name: /^Download / }).click();
  const downloaded = await downloading;
  const buf = readFileSync(await downloaded.path());
  return { name: downloaded.suggestedFilename(), entries: unzip(buf), bytes: buf.length, archive: buf };
}
