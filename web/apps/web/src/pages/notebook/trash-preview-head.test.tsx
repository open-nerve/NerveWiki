import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { docFactsClass, titleTextClass } from "../../app/page-classes";
import { cn } from "../../lib/cn";
import { notebookJSON, problem } from "../../test/fakes";
import { guide, install, linux, notes, pageNode, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";
import { docColumnClass, docTitleClass, headCrumbsClass, headRowClass } from "../page/page-layout";

// A trash item's preview headed as a page is (M13/P5/S2): its own file, so that trash-preview.test.tsx's cells keep
// their lines (its :587 is a known flake, named by its line).

const previewPath = (id: string) => `/lab/notebooks/${notebookJSON.id}/trash/${id}`;

/** trashed is pageServer with Install in its trash, Linux with it. */
function trashed() {
  const server = pageServer();
  server.trashNode(install.id);
  return { server };
}

// M13/P5/S2 (decisions 929, 937): the preview is headed as a page is: its trail in the head's row, its title the page's
// 32px one, a long name broken anywhere in it (M12-polish 5), Restore beside it, on the right below it too; its facts
// the document's grey.
const longName = `Notes${"quarterlyroadmap".repeat(4)}`;

test("a preview is headed as a page is, its title a long name broken in a span of its own, Restore on the right, its facts grey", async () => {
  const user = userEvent.setup();
  const long = pageNode(30, longName);
  const server = pageServer({
    nodes: [guide, install, linux, notes, long],
    answers: { "POST /api/v0/nodes/*/restore": () => problem(422, "page.restore_too_large") },
  });
  server.trashNode(long.id);
  renderApp(previewPath(long.id), server.app);
  const title = await screen.findByRole("heading", { level: 1, name: longName });

  expect(title.textContent).toBe(longName);
  expect(title.className).toBe(cn(docTitleClass, titleTextClass, "mb-0 flex items-start gap-2.5"));
  const name = title.querySelector("span.wrap-anywhere") as HTMLElement;
  expect(name.textContent).toBe(longName);
  expect(name.classList.contains("min-w-0")).toBe(true);
  expect(title.querySelector("svg")?.classList.contains("size-6")).toBe(true);
  const trail = screen.getByRole("navigation", { name: "Breadcrumb" });
  expect(trail.parentElement?.className).toBe(headCrumbsClass);
  expect(trail.parentElement?.parentElement?.className).toBe(headRowClass);
  expect(title.closest(".max-w-180")?.className).toBe(docColumnClass);
  const restore = screen.getByRole("button", { name: /^Restore “/ });
  expect(restore.parentElement?.classList.contains("ms-auto")).toBe(true);
  expect([...restore.classList]).toEqual(expect.arrayContaining(["h-7.5", "rounded-full"]));
  expect(title.parentElement?.nextElementSibling?.className).toBe(cn(docFactsClass, "mt-2.5 space-y-0.5"));

  // Refused as too large: Restore in the background, named for the item, breaks its name as its title does (review
  // B's P5B-M12), shrinking in its row as it wraps.
  await user.click(restore);
  const background = await screen.findByRole("button", { name: /in the background/u });
  expect([...background.classList]).toEqual(
    expect.arrayContaining(["h-auto", "min-h-7.5", "shrink", "whitespace-normal", "wrap-anywhere", "text-start"])
  );
  expect([...background.classList]).not.toContain("shrink-0");
  expect([...background.classList]).not.toContain("whitespace-nowrap");
});

// M13/P5/S2 (decisions 915, 946, 951): on its way, the preview's column is where the item read has it, below an empty
// row of its trail alone; no longer in the trash, or past its time, the same.

/** emptyHeadOf is the row above the column inside is in. */
const emptyHeadOf = (inside: HTMLElement) => inside.closest(".max-w-180")?.previousElementSibling as HTMLElement;

test("a preview on its way lays its head's row out empty, the trail alone, as the item read lays it", async () => {
  const { server } = trashed();
  let open!: () => void;
  server.trashWaits.item = new Promise<void>((resolve) => {
    open = resolve;
  });
  renderApp(previewPath(install.id), server.app);
  const loading = await waitFor(() => {
    const found = within(screen.getByRole("main")).getByText("Loading…", { selector: "output" });
    expect(found.closest(".max-w-180")).not.toBeNull();
    return found;
  });
  const empty = emptyHeadOf(loading);
  expect(empty.getAttribute("aria-hidden")).toBe("true");
  expect(empty.className).toBe(headRowClass);
  expect([...empty.children].map((block) => block.className)).toEqual([cn(headCrumbsClass, "h-[1lh] text-compact")]);

  act(() => open());
  const trail = await screen.findByRole("navigation", { name: "Breadcrumb" });
  expect(trail.parentElement?.parentElement?.children).toHaveLength(1);
});

test("a preview of a node no longer in the trash has the column and the empty head's row of one on its way", async () => {
  const { server } = trashed();
  renderApp(previewPath(guide.id), server.app);
  const title = await screen.findByRole("heading", { level: 1, name: "Trash" });

  const row = emptyHeadOf(title);
  expect(row.getAttribute("aria-hidden")).toBe("true");
  expect(row.className).toBe(headRowClass);
  expect(title.className).toBe(docTitleClass);
});
