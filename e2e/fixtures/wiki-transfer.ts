import { readFileSync } from "node:fs";

import type { TransferJob } from "@nervewiki/api-client";
import { expect, type Locator, type Page } from "@playwright/test";

import { unzip, type ZipEntry } from "./zip";

// The exports as a user works them (M7/P5 design 4.3, 4.5): the export's
// dialog confirmed, the job's row among the notebook's recent jobs, and its
// archive downloaded by the row's link.

/** The row of the job named name, by what it does, among the recent jobs page shows. */
export function jobRow(page: Page, name: string): Locator {
  return page.getByRole("list", { name: "Recent jobs" }).getByRole("listitem", { name, exact: true });
}

/** Confirms the export's dialog titled title; resolves the job the server started, queued. */
export async function confirmExport(page: Page, title: string): Promise<TransferJob> {
  const dialog = page.getByRole("alertdialog", { name: title });
  const answer = page.waitForResponse(
    (response) => response.request().method() === "POST" && new URL(response.url()).pathname.endsWith("/exports")
  );
  await dialog.getByRole("button", { name: "Export", exact: true }).click();
  const response = await answer;
  expect(response.status(), "the export").toBe(202);
  return (await response.json()) as TransferJob;
}

/** Waits for the row's job to succeed, then downloads its archive by its link: the file's name, entries and size. */
export async function downloadFrom(
  page: Page,
  row: Locator
): Promise<{ name: string; entries: ZipEntry[]; bytes: number }> {
  await expect(row.getByText("Done", { exact: true })).toBeVisible({ timeout: 15_000 });
  const downloading = page.waitForEvent("download");
  await row.getByRole("link", { name: "Download" }).click();
  const downloaded = await downloading;
  const buf = readFileSync(await downloaded.path());
  return { name: downloaded.suggestedFilename(), entries: unzip(buf), bytes: buf.length };
}
