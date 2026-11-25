import { screen, within } from "@testing-library/react";
import { expect, test } from "vitest";

import { notebookJSON } from "../../test/fakes";
import { pageNode, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// A notebook's home (M13/P5/S1, decision 919): its title and New page; its root pages a row each, a page's icon
// before its name, how many pages are under it at the row's end, which the link's description says; no settings link.

const home = `/lab/notebooks/${notebookJSON.id}`;
const guide = pageNode(1, "Guide");
const install = pageNode(2, "Install", guide);
const usage = pageNode(3, "Usage", guide);
const linux = pageNode(4, "Linux", install);
const notes = pageNode(5, "Notes");
const plans = pageNode(6, "Plans for Q1");
const q1 = pageNode(7, "January", plans);

/** What a row shows read by no one, of its own children: the number of its subpages, if any. */
const seen = (row: HTMLElement | undefined) =>
  Array.from(row?.children ?? []).filter((child) => child.getAttribute("aria-hidden") === "true");

/** The home's list of its root pages, in the main. */
const listed = async () => within(screen.getByRole("main")).findByRole("list", { name: "Pages" });

test("a root page's row: its icon read by no one, its link named by the page alone, described by how many pages are under it, the number seen read by no one", async () => {
  renderApp(home, pageServer({ nodes: [guide, install, usage, linux, notes, plans, q1] }).app);
  const list = await listed();

  const rows = within(list).getAllByRole("listitem");
  expect(
    within(list)
      .getAllByRole("link")
      .map((link) => link.textContent)
  ).toEqual(["Guide", "Notes", "Plans for Q1"]);
  for (const row of rows) {
    const icon = row.querySelector("svg");
    expect(icon?.getAttribute("aria-hidden")).toBe("true");
    expect(row.querySelector("a")?.contains(icon ?? null)).toBe(true);
  }
  // Two pages under Guide (Linux is under Install, not under Guide), one under Plans for Q1, none under Notes.
  expect(within(list).getByRole("link", { name: "Guide", description: "2 subpages" })).toBeTruthy();
  expect(within(list).getByRole("link", { name: "Plans for Q1", description: "1 subpage" })).toBeTruthy();
  // The sentence the description reads is hidden: read once, as the description, not again in browse mode (decision
  // 919, P5S1C-M2).
  const described = within(list).getByRole("link", { name: "Guide" }).getAttribute("aria-describedby") ?? "";
  expect(document.getElementById(described)?.hidden).toBe(true);
  const plain = within(list).getByRole("link", { name: "Notes" });
  expect(plain.getAttribute("aria-describedby")).toBeNull();
  // The number seen is outside the link, read by no one; Notes' row has none.
  const [guideRow, notesRow, plansRow] = rows;
  expect(seen(guideRow).map((child) => child.textContent)).toEqual(["2"]);
  expect(seen(plansRow).map((child) => child.textContent)).toEqual(["1"]);
  expect(seen(notesRow)).toEqual([]);
  expect(guideRow?.querySelector("a")?.textContent).toBe("Guide");
  expect(
    within(guideRow as HTMLElement)
      .getByText("2")
      .closest("a")
  ).toBeNull();
});

test("in Chinese, how many pages are under a root page: one, and more", async () => {
  const server = pageServer({ nodes: [guide, install, usage, notes, plans, q1] });
  server.app.preferences.setLocale("zh-CN");
  renderApp(home, server.app);
  const list = await within(screen.getByRole("main")).findByRole("list", { name: "页面" });

  expect(within(list).getByRole("link", { name: "Guide", description: "2 个子页面" })).toBeTruthy();
  expect(within(list).getByRole("link", { name: "Plans for Q1", description: "1 个子页面" })).toBeTruthy();
});

test("how many pages are under a root page, seen and said, is written as the locale writes numbers", async () => {
  const big = pageNode(8, "Archive");
  const under = Array.from({ length: 1200 }, (_, i) => ({
    ...pageNode(9, `Box ${i}`, big),
    id: `0199a2b4-0000-7000-8000-${String(10_000 + i).padStart(12, "0")}`,
  }));
  renderApp(home, pageServer({ nodes: [big, ...under] }).app);
  const list = await listed();

  expect(within(list).getByRole("link", { name: "Archive", description: "1,200 subpages" })).toBeTruthy();
  expect(seen(within(list).getByRole("listitem")).map((child) => child.textContent)).toEqual(["1,200"]);
});

test("the home has no link to the notebook's settings: the left column's cog is the way there", async () => {
  renderApp(home, pageServer({ nodes: [guide, notes] }).app);
  await listed();

  expect(within(screen.getByRole("main")).queryByRole("link", { name: "Notebook settings" })).toBeNull();
  expect(screen.getByRole("link", { name: "Settings of Plans" })).toBeTruthy();
});

test("a notebook with no page says so in a paragraph, its icon read by no one; New page once, in the title's row", async () => {
  renderApp(home, pageServer({ nodes: [] }).app);

  const line = await screen.findByText("No pages yet.");
  expect(line.tagName).toBe("P");
  expect(line.previousElementSibling?.getAttribute("aria-hidden")).toBe("true");
  const section = screen.getByRole("heading", { level: 1, name: "Plans" }).closest("section") as HTMLElement;
  expect(within(section).getAllByRole("button", { name: "New page" })).toHaveLength(1);
});
