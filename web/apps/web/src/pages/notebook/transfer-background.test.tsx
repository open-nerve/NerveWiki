import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import type { RevertConflict } from "../../services/page.service";
import type { TransferJob } from "../../services/transfer.service";
import { notebookJSON } from "../../test/fakes";
import { changesetId } from "../../test/history-server";
import { jobChangesetId, jobJSON, jobsServer, reportOf, underWay } from "../../test/jobs-server";
import { ada, bob, guide, install, linux, notes, pageNode } from "../../test/page-server";
import { renderApp } from "../../test/render";

// A notebook's background restores and reverts among its jobs (M8/P4 design 4.3).

const home = `/lab/notebooks/${notebookJSON.id}`;
const transfer = `${home}/settings/transfer`;
const history = (id: string) => `${home}/history/${id}`;
const list = () => screen.findByRole("list", { name: "Recent jobs" });
/** The row of the job whose name starts with name: what it does. */
const row = async (name: string) =>
  within(await list()).findByRole("listitem", { name: (named) => named.startsWith(name) });
/** links are the links of a row to the history, each by its text, its name and where it leads. */
const links = (of: HTMLElement) =>
  within(of)
    .queryAllByRole("link")
    .filter((link) => link.textContent !== "Download")
    .map((link) => [link.textContent, link.getAttribute("aria-label"), link.getAttribute("href")]);

const restoreOf = (n: number, more: Partial<TransferJob> = {}) =>
  jobJSON(n, { kind: "trash_restore", root_id: linux.id, name: "Linux", changeset_id: jobChangesetId(n), ...more });
const revertOf = (n: number, more: Partial<TransferJob> = {}) =>
  jobJSON(n, {
    kind: "revert",
    name: "import",
    changeset_id: jobChangesetId(n),
    target_changeset_id: changesetId(7),
    ...more,
  });

/** reportOpened opens the report of the job whose row's name starts with name: it answers the report. */
async function reportOpened(user: ReturnType<typeof userEvent.setup>, name: string): Promise<HTMLElement> {
  const job = await row(name);
  const report = within(job).getByRole("button", { name: (named) => named.startsWith("Report on ") });
  await user.click(report);
  return document.getElementById(report.getAttribute("aria-controls") ?? "") as HTMLElement;
}

/** countsOf are the counts a report shows, each its name and its number. */
const countsOf = (report: HTMLElement) =>
  [...report.querySelectorAll("dt")].map(
    (count) => `${count.textContent ?? ""} ${count.nextElementSibling?.textContent ?? ""}`
  );

test("a restore's and a revert's rows say what they do: a revert by the kind of the change it undoes, without an article; one named as another, by the end of its id too", async () => {
  const server = jobsServer({
    jobs: [
      restoreOf(1),
      revertOf(2),
      revertOf(3, { name: "revert", created_by: bob }),
      revertOf(4, { name: "trash_restore" }),
      revertOf(5, { name: "trash_restore" }),
    ],
  });
  renderApp(transfer, server.app);

  const rows = within(await list())
    .getAllByRole("listitem")
    .map((each) => each.getAttribute("aria-label"));
  expect(rows).toEqual([
    "Restore of Linux from the trash, Oct 5, 2026, 8:00 AM",
    "Undo: Import, Oct 5, 2026, 8:00 AM",
    "Undo: Undo change, Bob, Oct 5, 2026, 8:00 AM",
    "Undo: Restore from trash, Oct 5, 2026, 8:00 AM (ID 000704)",
    "Undo: Restore from trash, Oct 5, 2026, 8:00 AM (ID 000705)",
  ]);

  act(() => server.app.preferences.setLocale("zh-CN"));
  await waitFor(() =>
    expect(
      within(screen.getByRole("list", { name: "最近的任务" }))
        .getAllByRole("listitem")
        .map((each) => each.querySelector("p")?.textContent)
    ).toEqual(["从回收站恢复「Linux」", "撤销一次导入", "撤销一次撤销", "撤销一次恢复", "撤销一次恢复"])
  );
});

test("a job that wrote leads to its changeset in the history, a revert to the change it undoes too, each named by the job; one that wrote nothing has no changeset to lead to", async () => {
  const imported = jobJSON(1, { kind: "import", name: "Vault.zip", changeset_id: jobChangesetId(1) });
  const server = jobsServer({
    jobs: [
      imported,
      underWay(restoreOf(2), 40, 103),
      revertOf(3, { progress: { done: 1501, total: 1501 } }),
      // Nothing to undo: no step, no changeset.
      revertOf(4, { progress: { done: 0, total: 0 }, report: reportOf(null) }),
      underWay(restoreOf(5), 0, 0),
      // Interrupted, its progress may be a heartbeat behind what it wrote.
      restoreOf(6, { state: "failed", progress: { done: 0, total: 103 }, report: reportOf("interrupted") }),
      // Interrupted as the queue dropped it before it began (the rescue's): it wrote nothing (P4B review B-M2).
      restoreOf(9, {
        state: "failed",
        started_at: null,
        progress: { done: 0, total: 0 },
        report: reportOf("interrupted"),
      }),
      // An export, and an import of before the jobs had their changeset, have none.
      jobJSON(7),
      jobJSON(8, { kind: "import", name: "Old.zip", changeset_id: null }),
    ],
  });
  renderApp(transfer, server.app);

  expect(links(await row("Import of Vault.zip"))).toEqual([
    [
      "See in the history",
      "See in the history (Import of Vault.zip, Oct 5, 2026, 8:00 AM)",
      history(jobChangesetId(1)),
    ],
  ]);
  const running = await row("Restore of Linux from the trash, Oct 5, 2026, 8:00 AM (ID 000702)");
  expect(links(running)).toEqual([
    [
      "See in the history",
      "See in the history (Restore of Linux from the trash, Oct 5, 2026, 8:00 AM (ID 000702))",
      history(jobChangesetId(2)),
    ],
  ]);
  expect(links(await row("Undo: Import, Oct 5, 2026, 8:00 AM (ID 000703)"))).toEqual([
    [
      "See in the history",
      "See in the history (Undo: Import, Oct 5, 2026, 8:00 AM (ID 000703))",
      history(jobChangesetId(3)),
    ],
    [
      "The change undone",
      "The change undone (Undo: Import, Oct 5, 2026, 8:00 AM (ID 000703))",
      history(changesetId(7)),
    ],
  ]);
  expect(links(await row("Undo: Import, Oct 5, 2026, 8:00 AM (ID 000704)"))).toEqual([
    [
      "The change undone",
      "The change undone (Undo: Import, Oct 5, 2026, 8:00 AM (ID 000704))",
      history(changesetId(7)),
    ],
  ]);
  expect(links(await row("Restore of Linux from the trash, Oct 5, 2026, 8:00 AM (ID 000705)"))).toEqual([]);
  expect(links(await row("Restore of Linux from the trash, Oct 5, 2026, 8:00 AM (ID 000706)"))).toEqual([
    [
      "See in the history",
      "See in the history (Restore of Linux from the trash, Oct 5, 2026, 8:00 AM (ID 000706))",
      history(jobChangesetId(6)),
    ],
  ]);
  expect(links(await row("Restore of Linux from the trash, Oct 5, 2026, 8:00 AM (ID 000709)"))).toEqual([]);
  expect(links(await row("Export of the whole notebook"))).toEqual([]);
  expect(links(await row("Import of Old.zip"))).toEqual([]);
});

test("a restore running says its progress in nodes, as the jobs are read again", async () => {
  const server = jobsServer({ jobs: [underWay(restoreOf(1), 0, 103)] });
  renderApp(transfer, server.app);

  const job = await row("Restore of Linux");
  expect(within(job).getByText("Running")).toBeTruthy();
  expect(within(job).getByRole("progressbar", { name: "Progress: 0 of 103" })).toBeTruthy();
  server.jobs = [underWay(restoreOf(1), 100, 103)];
  await waitFor(() => expect(within(job).getByRole("progressbar", { name: "Progress: 100 of 103" })).toBeTruthy(), {
    timeout: 3000,
  });
});

test.each<[string, NonNullable<NonNullable<TransferJob["report"]>["placed"]>, string]>([
  [
    "under the page it was under",
    { parent_id: install.id, reason: "original" },
    "Put back where it was, under “Install”.",
  ],
  [
    "at the top level, where it was",
    { parent_id: null, reason: "original" },
    "Put back where it was, at the notebook's top level.",
  ],
  [
    "under a page the tree has not",
    { parent_id: pageNode(40, "Gone").id, reason: "original" },
    "Put back where it was.",
  ],
  [
    "at the top level, the page it was under in the trash",
    { parent_id: null, reason: "parent_in_trash" },
    "Put at the notebook's top level: the page it was under is in the trash.",
  ],
  [
    "at the top level, too deep where it was",
    { parent_id: null, reason: "too_deep" },
    "Put at the notebook's top level: where it was, it would nest more than 10 levels deep.",
  ],
])(
  "a restore's report counts what it restored and skipped, and says where it put the item: %s",
  async (_, placed, says) => {
    const user = userEvent.setup();
    const report = { ...reportOf(null, { pages: 102, attachments: 1, renamed: 2, skipped: 3 }), placed };
    const server = jobsServer({ jobs: [restoreOf(1, { report, progress: { done: 103, total: 103 } })] });
    renderApp(transfer, server.app);

    const shown = await reportOpened(user, "Restore of Linux");

    expect(countsOf(shown)).toEqual([
      "Pages 102",
      "Attachments 1",
      "Renamed 2",
      "Skipped, restored by someone first 3",
    ]);
    expect(within(shown).getByText(says)).toBeTruthy();
    expect(within(shown).queryByText(/left are in the trash/u)).toBeNull();
  }
);

test("a revert's report counts what it did to the nodes and the pages; one with nothing to undo says so", async () => {
  const user = userEvent.setup();
  const counts = { restored: 5, deleted: 1501, put_back: 2, written: 3, skipped: 1 };
  const server = jobsServer({
    jobs: [
      revertOf(1, { report: reportOf(null, counts), progress: { done: 1512, total: 1512 } }),
      revertOf(2, { report: reportOf(null), progress: { done: 0, total: 0 } }),
    ],
  });
  renderApp(transfer, server.app);

  const done = await reportOpened(user, "Undo: Import, Oct 5, 2026, 8:00 AM (ID 000701)");
  expect(countsOf(done)).toEqual([
    "Nodes restored 5",
    "Nodes deleted 1,501",
    "Nodes put back 2",
    "Pages written back 3",
    "Skipped, restored by someone first 1",
  ]);
  expect(within(done).queryByText(/^Nothing to undo/u)).toBeNull();
  const nothing = await reportOpened(user, "Undo: Import, Oct 5, 2026, 8:00 AM (ID 000702)");
  expect(
    within(nothing).getByText("Nothing to undo: whatever the change touched is as it was before it.")
  ).toBeTruthy();
});

test.each<[string, TransferJob, string, string[]]>([
  [
    "a restore failed as what it restored was deleted",
    restoreOf(1, {
      state: "failed",
      report: reportOf("tree_changed", { pages: 39 }),
      progress: { done: 40, total: 103 },
    }),
    "Failed · While the restore ran, what it had restored was deleted or moved too deep.",
    ["Nodes it had not come to: 63. They are in the trash, in items of their own: each can be restored alone."],
  ],
  [
    "a restore cancelled",
    restoreOf(1, { state: "cancelled", report: reportOf(null, { pages: 99 }), progress: { done: 100, total: 1203 } }),
    "Cancelled",
    [
      "What it restored stays in the notebook. Nodes left: 1,103. They are in the trash, in items of their own: each can be restored alone.",
    ],
  ],
  [
    "a restore failed before it restored anything",
    restoreOf(1, { state: "failed", report: reportOf("trash_changed"), progress: { done: 0, total: 103 } }),
    "Failed · The item in the trash changed before the restore came to it: someone restored it, the purge took it, or it was deleted again.",
    [],
  ],
  [
    "a restore failed as its item changed after its first batch",
    restoreOf(1, {
      state: "failed",
      report: reportOf("trash_changed", { pages: 99 }),
      progress: { done: 100, total: 103 },
    }),
    "Failed · The item in the trash changed before the restore came to it: someone restored it, the purge took it, or it was deleted again.",
    [],
  ],
  [
    "a restore failed as its starter could no longer edit the notebook",
    restoreOf(1, { state: "failed", report: reportOf("forbidden"), progress: { done: 0, total: 103 } }),
    "Failed · The person who started it could no longer edit the notebook.",
    [],
  ],
  [
    "a restore failed as what it restored was undone",
    restoreOf(1, {
      state: "failed",
      report: reportOf("changeset_reverted", { pages: 99 }),
      progress: { done: 100, total: 103 },
    }),
    "Failed · What the restore had restored was undone while it ran.",
    ["What it restored is in the trash again, with the nodes it had not come to."],
  ],
  [
    "a restore failed as its item went past its time before its first batch",
    restoreOf(1, { state: "failed", report: reportOf("restore_expired"), progress: { done: 0, total: 103 } }),
    "Failed · The item went past the time it could be restored in.",
    [],
  ],
  [
    "a restore failed as its item went past its time between its batches",
    restoreOf(1, {
      state: "failed",
      report: reportOf("restore_expired", { pages: 99 }),
      progress: { done: 100, total: 103 },
    }),
    "Failed · The item went past the time it could be restored in.",
    [
      "What it restored stays in the notebook. Nodes left: 3. Their time in the trash is over: they can no longer be restored.",
    ],
  ],
  [
    "a revert failed as its starter could no longer edit the notebook",
    revertOf(1, { state: "failed", report: reportOf("forbidden"), progress: { done: 0, total: 1501 } }),
    "Failed · The person who started it could no longer edit the notebook.",
    [],
  ],
  [
    "a revert failed as its changeset was undone",
    revertOf(1, {
      state: "failed",
      report: reportOf("changeset_reverted", { deleted: 300 }),
      progress: { done: 300, total: 1501 },
    }),
    "Failed · What the undo had done was itself undone while it ran.",
    [],
  ],
  [
    "a revert interrupted",
    revertOf(1, {
      state: "failed",
      report: reportOf("interrupted", { deleted: 100 }),
      progress: { done: 100, total: 1501 },
    }),
    "Failed · The job was interrupted: the server stopped or restarted, or the job stopped responding.",
    [
      "What it undid stays. Nodes not undone: 1,401. To go back to before, undo this undo in the history; then the change can be undone whole again.",
      "If it stopped in the middle of restoring a group, restoring the items it left in the trash one by one completes that group: do not undo this undo first.",
    ],
  ],
  [
    "a revert failed as the purge took part of its change at its start",
    revertOf(1, { state: "failed", report: reportOf("changeset_trimmed"), progress: { done: 0, total: 0 } }),
    "Failed · The purge took part of the change: it can no longer be undone.",
    [],
  ],
  [
    "a revert failed as the purge took part of its change between its batches",
    revertOf(1, {
      state: "failed",
      report: reportOf("changeset_trimmed", { deleted: 300 }),
      progress: { done: 300, total: 1501 },
    }),
    "Failed · The purge took part of the change: it can no longer be undone.",
    ["What it undid stays. Nodes not undone: 1,201. To go back to before, undo this undo in the history."],
  ],
])("%s says why, and what became of what it did, what is left and what to do", async (_, job, state, says) => {
  const user = userEvent.setup();
  const server = jobsServer({ jobs: [job] });
  renderApp(transfer, server.app);

  const title = job.kind === "revert" ? "Undo: Import" : "Restore of Linux";
  expect(within(await row(title)).getByText(state, { selector: "p" })).toBeTruthy();

  const shown = await reportOpened(user, title);

  await waitFor(() => expect(server.asked).toContain(`GET job ${job.name}`));
  expect([...shown.querySelectorAll(":scope > p")].map((line) => line.textContent)).toEqual(says);
});

test.each([
  {
    failure: "page_locked" as const,
    lock: { page_id: guide.id, ...bob },
    says: ["Bob is editing “Guide”.Open “Guide”"],
  },
  { failure: "revert_conflict" as const, lock: null, says: [] },
])(
  "a revert stopped as it began ($failure), at 0 of 0, says why, and not that there was nothing to undo",
  async ({ failure, lock, says }) => {
    const user = userEvent.setup();
    const job = revertOf(1, {
      state: "failed",
      report: { ...reportOf(failure), lock },
      progress: { done: 0, total: 0 },
    });
    const server = jobsServer({ jobs: [job] });
    renderApp(transfer, server.app);
    await screen.findByRole("navigation", { name: "Pages of Plans" });

    const shown = await reportOpened(user, "Undo: Import");

    await waitFor(() => expect(server.asked).toContain("GET job import"));
    expect([...shown.querySelectorAll(":scope > p")].map((line) => line.textContent)).toEqual(says);
  }
);

test("the reports of an import, a restore and a revert stopped by none show no conflicts", async () => {
  const user = userEvent.setup();
  const server = jobsServer({
    jobs: [
      jobJSON(1, { kind: "import", name: "Vault.zip", changeset_id: jobChangesetId(1) }),
      restoreOf(2, { report: reportOf(null, { pages: 4 }) }),
      revertOf(3),
    ],
  });
  renderApp(transfer, server.app);

  for (const [title, name] of [
    ["Import of Vault.zip", "Vault.zip"],
    ["Restore of Linux", "Linux"],
    ["Undo: Import", "import"],
  ] as const) {
    // oxlint-disable-next-line no-await-in-loop -- one report after another, each read as it opens
    const shown = await reportOpened(user, title);
    // oxlint-disable-next-line no-await-in-loop -- as above
    await waitFor(() => expect(server.asked).toContain(`GET job ${name}`));
    expect(within(shown).queryByRole("heading", { name: /^Nodes that stopped the undo/u })).toBeNull();
    expect(within(shown).queryByText(/The change undone/u)).toBeNull();
  }
});

test("a revert stopped by its conflicts lists them, read as its report opens: each why, by its name, those of one name by the end of their ids; how many, the first hundred listed; no links, which the change undone's undo has", async () => {
  const user = userEvent.setup();
  const job = revertOf(1, {
    state: "failed",
    report: reportOf("revert_conflict", { deleted: 10 }),
    progress: { done: 10, total: 1501 },
  });
  const server = jobsServer({ jobs: [job] });
  const conflicts: RevertConflict[] = [
    { node_id: notes.id, name: "Notes", reason: "changed" },
    { node_id: guide.id, name: "Twin", reason: "children_added" },
    { node_id: install.id, name: "Twin", reason: "title_taken", holder_id: linux.id },
  ];
  server.conflicts.set(job.id, { conflicts, conflict_count: 1200 });
  renderApp(transfer, server.app);

  const shown = await reportOpened(user, "Undo: Import");

  const listed = await within(shown).findByRole("list", { name: "Nodes that stopped the undo: 1,200" });
  expect(
    within(listed)
      .getAllByRole("listitem")
      .map((each) => each.textContent)
  ).toEqual([
    "“Notes” changed since.",
    "“Twin” was put there since, and would go with the undo: move it away first. ID 000101",
    "Where “Twin” goes back, a page or attachment has its name: rename that one first. ID 000102",
  ]);
  expect(within(listed).queryAllByRole("link")).toEqual([]);
  expect(within(shown).getByText("The first 3 are listed.")).toBeTruthy();
  expect(
    within(shown).getByText("Undoing the change again from “The change undone” lists them with their links.")
  ).toBeTruthy();
  expect(
    within(await row("Undo: Import")).getByText("Failed · What the change touched changed since, or could not go back.")
  ).toBeTruthy();
});

test("a revert stopped by conflicts all listed does not say only the first are", async () => {
  const user = userEvent.setup();
  const job = revertOf(1, {
    state: "failed",
    report: reportOf("revert_conflict", { deleted: 10 }),
    progress: { done: 10, total: 1501 },
  });
  const server = jobsServer({ jobs: [job] });
  const conflicts: RevertConflict[] = [
    { node_id: notes.id, name: "Notes", reason: "changed" },
    { node_id: guide.id, name: "Guide", reason: "children_added" },
  ];
  server.conflicts.set(job.id, { conflicts, conflict_count: 2 });
  renderApp(transfer, server.app);

  const shown = await reportOpened(user, "Undo: Import");

  expect(await within(shown).findByRole("list", { name: "Nodes that stopped the undo: 2" })).toBeTruthy();
  expect(within(shown).queryByText(/^The first/u)).toBeNull();
});

test.each([
  {
    who: "another account",
    lock: { page_id: guide.id, ...bob },
    says: "Bob is editing “Guide”.",
    open: true,
  },
  {
    who: "the account itself elsewhere",
    lock: { page_id: guide.id, ...ada },
    says: "You are editing “Guide” elsewhere.",
    open: true,
  },
  {
    who: "another account, on a page the tree has not",
    lock: { page_id: pageNode(40, "Gone").id, ...bob },
    says: "Bob is editing a page the undo was about to change.",
    open: false,
  },
  {
    who: "the account itself elsewhere, on a page the tree has not",
    lock: { page_id: pageNode(40, "Gone").id, ...ada },
    says: "You are editing, elsewhere, a page the undo was about to change.",
    open: false,
  },
])(
  "a revert stopped by an edit session of $who says who edits which page, leading there",
  async ({ lock, says, open }) => {
    const user = userEvent.setup();
    const job = revertOf(1, {
      state: "failed",
      report: { ...reportOf("page_locked", { deleted: 5 }), lock },
      progress: { done: 5, total: 20 },
    });
    const server = jobsServer({ jobs: [job] });
    renderApp(transfer, server.app);
    await screen.findByRole("navigation", { name: "Pages of Plans" });

    const shown = await reportOpened(user, "Undo: Import");

    await waitFor(() => expect(within(shown).getByText(says)).toBeTruthy());
    expect(
      within(await row("Undo: Import")).getByText("Failed · Someone is editing a page the undo was about to change.")
    ).toBeTruthy();
    const opening = within(shown).queryByRole("link", { name: "Open “Guide”" });
    expect(opening?.getAttribute("href") ?? null).toBe(open ? `${home}/pages/${guide.id}` : null);
  }
);
