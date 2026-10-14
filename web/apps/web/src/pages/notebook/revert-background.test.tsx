import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import type { RevertConflict } from "../../services/page.service";
import { json, notebookJSON, problem, type Answer } from "../../test/fakes";
import { changesetId, renamed, wrote } from "../../test/history-server";
import { jobJSON, jobsServer, reportOf, underWay } from "../../test/jobs-server";
import { bob, guide, install, linux, notes } from "../../test/page-server";
import { renderApp } from "../../test/render";

// A changeset too large to undo here, undone in the background (M8/P4 design 4.2).

const home = `/lab/notebooks/${notebookJSON.id}`;
const changesetPath = (id: string) => `${home}/history/${id}`;
const transfer = `${home}/settings/transfer`;
const reverts = `POST /api/v0/notebooks/${notebookJSON.id}/reverts`;

/**
 * large is jobsServer with Ada's rework in its history, too large to undo
 * here: it renamed Notes from Old notes, and wrote Guide, among a thousand
 * nodes or more. answers answer instead of it.
 */
function large(answers: Record<string, Answer> = {}) {
  const server = jobsServer({ answers, nodes: [guide, install, linux, notes] });
  const rework = server.log("rework", {
    at: "2026-10-05T08:01:00Z",
    changes: [renamed(notes, "Old notes"), wrote(guide, 1, 2)],
    counts: { nodes: 1000, contents: 3 },
    tooLarge: true,
  });
  return { server, rework };
}

/** gate is a wait a test opens: what waits for its promise goes on once open is called. */
function gate() {
  let open!: () => void;
  const promise = new Promise<void>((resolve) => {
    open = resolve;
  });
  return { promise, open };
}

/** title is the changeset's page's title. */
const title = () => screen.getByRole("heading", { level: 1, name: "Edit · Ada" });

/** starts are the starts of reverts in the background that went out. */
const starts = (asked: string[]) => asked.filter((each) => each.startsWith("POST revert"));

/**
 * tooLarge opens the dialog of Undo change, once the changeset's page
 * shows, types message, and undoes: refused as too large, the dialog asks
 * to undo in the background. It answers the dialog.
 */
async function tooLarge(user: ReturnType<typeof userEvent.setup>, message = ""): Promise<HTMLElement> {
  await user.click(await screen.findByRole("button", { name: "Undo change" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Undo this change?" });
  if (message !== "") {
    await user.type(within(dialog).getByRole("textbox", { name: "Message (optional)" }), message);
  }
  await user.click(within(dialog).getByRole("button", { name: "Undo" }));
  await within(dialog).findByRole("button", { name: "Undo in the background" });
  return dialog;
}

test("an undo too large here asks whether to undo it in the background: that it is too large and how many nodes it touched, a thousand as many or more, which takes the focus and describes the dialog; the message stays", async () => {
  const user = userEvent.setup();
  const { server, rework } = large();
  renderApp(changesetPath(rework.id), server.app);

  const dialog = await tooLarge(user, "Back");

  const heading = within(dialog).getByRole("heading", {
    level: 3,
    name: "This change is too large to undo here: its nodes, pages, content, links or those pages' history are more than one undo does. Nodes it touched: 1,000+.",
  });
  expect(document.activeElement).toBe(heading);
  expect(dialog.getAttribute("aria-describedby")).toBe(heading.id);
  expect(
    within(dialog).getByText(
      "Undo it in the background? An undo in the background goes in batches: stopped on the way, what it undid stays."
    )
  ).toBeTruthy();
  expect(within(dialog).getByText("Links written since to the new names or places are not changed back.")).toBeTruthy();
  expect((within(dialog).getByRole("textbox", { name: "Message (optional)" }) as HTMLInputElement).value).toBe("Back");
  expect(
    within(dialog)
      .getAllByRole("button")
      .map((button) => button.textContent)
  ).toEqual(["Cancel", "Undo in the background"]);
  expect(within(dialog).queryByRole("alert")).toBeNull();
  expect(server.sent.filter((each) => each.startsWith("REVERT"))).toEqual(['REVERT rework "Back"']);
  expect(starts(server.asked)).toEqual([]);

  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "Undo change" }));
  expect(starts(server.asked)).toEqual([]);
});

test("Undo in the background starts the undo's job with the message: the dialog closes, the page says so, leading to its progress, the title taking the focus, described by it; there, its row takes the focus", async () => {
  const user = userEvent.setup();
  const { server, rework } = large();
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await tooLarge(user, "  Back to <old>  ");

  await user.click(within(dialog).getByRole("button", { name: "Undo in the background" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  const status = screen.getByRole("status");
  expect(status.textContent).toBe("Undoing this change in the background has started. See its progress");
  expect(document.activeElement).toBe(title());
  expect(title().getAttribute("aria-describedby")).toBe(status.id);
  expect(starts(server.asked)).toEqual(['POST revert rework "Back to <old>"']);
  const job = server.jobs[0];
  expect([job?.kind, job?.state, job?.target_changeset_id, job?.name]).toEqual(["revert", "queued", rework.id, "edit"]);
  expect(server.messages.get(job?.id ?? "")).toBe("Back to <old>");
  const progress = within(status).getByRole("link", { name: "See its progress" });
  expect(progress.getAttribute("href")).toBe(transfer);

  await user.click(progress);
  const row = await screen.findByRole("listitem", { name: (name) => name.startsWith("Undo: Edit, ") });
  await waitFor(() => expect(document.activeElement).toBe(row));
});

test("an undo in the background without a message sends none", async () => {
  const user = userEvent.setup();
  const { server, rework } = large();
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await tooLarge(user, "   ");

  await user.click(within(dialog).getByRole("button", { name: "Undo in the background" }));

  await waitFor(() => expect(starts(server.asked)).toEqual(["POST revert rework"]));
  expect(server.messages.get(server.jobs[0]?.id ?? "")).toBeUndefined();
});

test.each<[string, (state: ReturnType<typeof large>) => void, string]>([
  [
    "a job that writes the tree under way",
    ({ server }) => {
      server.busy = true;
    },
    "An import, a restore or an undo is under way in this notebook: wait for it to end, then try again.",
  ],
  [
    "the queue full",
    ({ server }) => {
      server.queueFull = true;
    },
    "Too many jobs are waiting on the server. Try again in a few minutes.",
  ],
  [
    "a changeset too large for the background too",
    ({ rework }) => {
      rework.jobTooLarge = true;
    },
    "This change is too large to undo, even in the background.",
  ],
  [
    "a page someone edits",
    ({ rework }) => {
      rework.lockedBy = { page: guide.id, holder: bob };
    },
    "Bob is editing “Guide”.",
  ],
])(
  "an undo in the background refused as %s says so in the dialog, which stays, the focus on Undo in the background; pressed again, it starts",
  async (_, refuse, text) => {
    const user = userEvent.setup();
    const state = large();
    const { server, rework } = state;
    renderApp(changesetPath(rework.id), server.app);
    await screen.findByRole("navigation", { name: "Pages of Plans" });
    const dialog = await tooLarge(user);
    refuse(state);
    const start = within(dialog).getByRole("button", { name: "Undo in the background" });

    await user.click(start);

    expect((await within(dialog).findByRole("alert")).textContent).toBe(text);
    expect(screen.getByRole("alertdialog")).toBe(dialog);
    expect(document.activeElement).toBe(start);
    expect(server.jobs).toEqual([]);

    Object.assign(server, { busy: false, queueFull: false });
    Object.assign(rework, { jobTooLarge: false, lockedBy: undefined });
    await user.click(start);
    await waitFor(() =>
      expect(screen.getByRole("status").textContent).toBe(
        "Undoing this change in the background has started. See its progress"
      )
    );
    expect(starts(server.asked)).toEqual(["POST revert rework", "POST revert rework"]);
  }
);

test("an undo in the background refused for its conflicts lists them in the dialog, their title taking the focus", async () => {
  const user = userEvent.setup();
  const { server, rework } = large();
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await tooLarge(user);
  rework.conflicts = [{ node_id: notes.id, name: "Notes", reason: "changed" } satisfies RevertConflict];

  await user.click(within(dialog).getByRole("button", { name: "Undo in the background" }));

  const heading = await within(dialog).findByRole("heading", {
    level: 3,
    name: "Not undone. Nodes changed since, or that cannot go back: 1",
  });
  expect(document.activeElement).toBe(heading);
  expect(dialog.getAttribute("aria-describedby")).toBe(heading.id);
  expect(within(dialog).getByText("“Notes” changed since.")).toBeTruthy();
  expect(server.jobs).toEqual([]);
});

test("an undo in the background of a changeset the purge trimmed meanwhile says so as the dialog closes, the title taking the focus; read again, Undo change goes", async () => {
  const user = userEvent.setup();
  const { server, rework } = large();
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await tooLarge(user);
  rework.trimmed = true;

  await user.click(within(dialog).getByRole("button", { name: "Undo in the background" }));

  const alert = await screen.findByRole("alert");
  expect(alert.textContent).toBe("Part of this change has been purged from the trash: it can no longer be reverted.");
  await waitFor(() => expect(screen.queryByRole("button", { name: "Undo change" })).toBeNull());
  expect(document.activeElement).toBe(title());
  expect(title().getAttribute("aria-describedby")).toBe(alert.id);
});

test("an undo in the background refused, the role taken away meanwhile, says why as the dialog closes, and reads the notebooks again: Undo change goes, the focus to the title", async () => {
  const user = userEvent.setup();
  let role = "admin";
  const reads: string[] = [];
  const { server, rework } = large({
    "GET /api/v0/workspaces/lab/notebooks": () => {
      reads.push("GET notebooks");
      return json({ data: [{ ...notebookJSON, role }] });
    },
    [reverts]: () => problem(403, "forbidden"),
  });
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await tooLarge(user);
  role = "reader";
  reads.length = 0;

  await user.click(within(dialog).getByRole("button", { name: "Undo in the background" }));

  expect((await screen.findByRole("alert")).textContent).toBe("You do not have permission to do this.");
  await waitFor(() => expect(screen.queryByRole("button", { name: "Undo change" })).toBeNull());
  expect(reads).toEqual(["GET notebooks"]);
  expect(document.activeElement).toBe(title());
});

test("an undo in the background of a changeset gone meanwhile closes the dialog, and reads it and the notebooks again: gone, it is no page", async () => {
  const user = userEvent.setup();
  const reads: string[] = [];
  const { server, rework } = large({
    "GET /api/v0/workspaces/lab/notebooks": () => {
      reads.push("GET notebooks");
      return json({ data: [notebookJSON] });
    },
  });
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await tooLarge(user);
  server.changesets = server.changesets.filter((each) => each !== rework);
  reads.length = 0;

  await user.click(within(dialog).getByRole("button", { name: "Undo in the background" }));

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
  expect(reads).toEqual(["GET notebooks"]);
});

// M8/P4 design 4.2, review B-M6: the changeset's reverted_by is empty until the job's first batch; the jobs tell.
test.each([
  { how: "500" as const, behind: 0 },
  { how: "network" as const, behind: 0 },
  { how: "500" as const, behind: 300 },
  { how: "network" as const, behind: 3000 },
])(
  "an undo in the background whose answer was lost ($how) reads the jobs again: its job there, made by a server's clock $behind ms behind the browser's, the page says it started, leading to its progress, the title taking the focus",
  async ({ how, behind }) => {
    const user = userEvent.setup();
    const { server, rework } = large();
    renderApp(changesetPath(rework.id), server.app);
    const dialog = await tooLarge(user);
    Object.assign(server, { lost: how, clockBehind: behind });
    const listed = server.asked.filter((each) => each.startsWith("GET jobs")).length;

    await user.click(within(dialog).getByRole("button", { name: "Undo in the background" }));

    const status = screen.getByRole("status");
    await waitFor(() =>
      expect(status.textContent).toBe("Undoing this change in the background has started. See its progress")
    );
    expect(document.activeElement).toBe(title());
    expect(title().getAttribute("aria-describedby")).toBe(status.id);
    expect(server.asked.filter((each) => each.startsWith("GET jobs")).length).toBe(listed + 1);
    expect(starts(server.asked)).toEqual(["POST revert rework"]);
  }
);

/** revertOf is the job n, Ada's revert in the background of an edit, which succeeded; more changes what it says. */
const revertOf = (n: number, more: Parameters<typeof jobJSON>[1]) =>
  jobJSON(n, { kind: "revert", name: "edit", report: reportOf(null), ...more });

test("an undo in the background whose answer was lost says so as it reads the jobs again; none of them its own, it did not start", async () => {
  const user = userEvent.setup();
  const { server, rework } = large({ [reverts]: () => problem(500, "internal_error") });
  // Another account's revert of it, and one of Ada's of another change, are none of this one's, though made since;
  // nor is Ada's revert of it that ended long before.
  const since = new Date(Date.now() + 60_000).toISOString();
  server.jobs = [
    revertOf(1, { target_changeset_id: rework.id, created_by: bob, created_at: since }),
    revertOf(2, { target_changeset_id: changesetId(9999), created_at: since }),
    revertOf(3, {
      target_changeset_id: rework.id,
      state: "failed",
      progress: { done: 0, total: 0 },
      report: reportOf("revert_conflict"),
    }),
  ];
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await tooLarge(user);
  const listed = gate();
  server.listWaits = listed.promise;

  await user.click(within(dialog).getByRole("button", { name: "Undo in the background" }));

  const status = screen.getByRole("status");
  await waitFor(() =>
    expect(status.textContent).toBe(
      "Whether undoing it in the background started is not known: reading the jobs again…"
    )
  );
  act(() => listed.open());
  await waitFor(() =>
    expect(status.textContent).toBe("Undoing it in the background did not start: you can try again.")
  );
  expect(document.activeElement).toBe(title());
});

// P4B review A-N3: what is found out of an undo is not said over what came of one sent later.
test("an undo in the background whose answer was lost, found out only after another undo started, says nothing over it", async () => {
  const user = userEvent.setup();
  const { server, rework } = large();
  renderApp(changesetPath(rework.id), server.app);
  const first = await tooLarge(user);
  const listed = gate();
  Object.assign(server, { lost: "500", listWaits: listed.promise });
  await user.click(within(first).getByRole("button", { name: "Undo in the background" }));
  const status = screen.getByRole("status");
  await waitFor(() =>
    expect(status.textContent).toBe(
      "Whether undoing it in the background started is not known: reading the jobs again…"
    )
  );

  // Its job dropped meanwhile, another undo starts.
  Object.assign(server, { lost: undefined, jobs: [] });
  const second = await tooLarge(user);
  await user.click(within(second).getByRole("button", { name: "Undo in the background" }));
  await waitFor(() =>
    expect(status.textContent).toBe("Undoing this change in the background has started. See its progress")
  );
  // The first one's read of the jobs, answered last, finds nothing out.
  server.listDown = true;
  act(() => listed.open());
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));

  expect(status.textContent).toBe("Undoing this change in the background has started. See its progress");
  expect(starts(server.asked)).toEqual(["POST revert rework", "POST revert rework"]);
});

test("an undo in the background whose answer was lost, the jobs not read again, says it is not known whether it started, leading to the jobs", async () => {
  const user = userEvent.setup();
  const { server, rework } = large();
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await tooLarge(user);
  Object.assign(server, { lost: "500", listDown: true });

  await user.click(within(dialog).getByRole("button", { name: "Undo in the background" }));

  const status = screen.getByRole("status");
  await waitFor(() =>
    expect(status.textContent).toBe(
      "Whether undoing it in the background started is not known: the jobs could not be read. See the jobs"
    )
  );
  expect(within(status).getByRole("link", { name: "See the jobs" }).getAttribute("href")).toBe(transfer);
});

test("Undo in the background pressed twice sends one start; busy, it stays where the focus is, and nothing closes the dialog while it is out", async () => {
  const user = userEvent.setup();
  const { server, rework } = large();
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await tooLarge(user);
  server.startsHeld = true;
  const start = within(dialog).getByRole("button", { name: "Undo in the background" });

  await user.dblClick(start);
  await user.click(start);

  await waitFor(() => expect(starts(server.asked)).toEqual(["POST revert rework"]));
  const starting = within(dialog).getByRole("button", { name: "Starting…" });
  expect([
    starting.getAttribute("aria-disabled"),
    starting.getAttribute("aria-busy"),
    starting.hasAttribute("disabled"),
  ]).toEqual(["true", "true", false]);
  await user.keyboard("{Escape}");
  expect(screen.getByRole("alertdialog")).toBe(dialog);
  act(() => server.release());
  await waitFor(() =>
    expect(screen.getByRole("status").textContent).toBe(
      "Undoing this change in the background has started. See its progress"
    )
  );
  expect(starts(server.asked)).toEqual(["POST revert rework"]);
  expect(server.jobs).toHaveLength(1);
});

test("an undo in the background answered after the reader scrolled leaves the focus where the dialog gives it back: they did something", async () => {
  const user = userEvent.setup();
  const { server, rework } = large();
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await tooLarge(user);
  server.startsHeld = true;
  await user.click(within(dialog).getByRole("button", { name: "Undo in the background" }));
  await waitFor(() => expect(starts(server.asked)).toEqual(["POST revert rework"]));

  act(() => void window.dispatchEvent(new WheelEvent("wheel")));
  act(() => server.release());

  await waitFor(() => expect(screen.getByRole("status").textContent).toMatch(/has started/u));
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "Undo change" }));
});

// M8/P4A review A-M2: an undo in the request as a job writes the tree finds the nodes it wrote meanwhile changed.
test.each([
  {
    while: "a job that writes the tree under way",
    job: underWay(jobJSON(1, { kind: "revert", target_changeset_id: changesetId(9999), name: "import" }), 100, 1501),
    says: true,
  },
  {
    while: "only a job ended",
    job: jobJSON(1, { kind: "revert", target_changeset_id: changesetId(9999), name: "import" }),
    says: false,
  },
  // An export writes no tree (P4B review C-M2).
  { while: "only an export under way", job: underWay(jobJSON(1), 1, 4), says: false },
])(
  "an undo refused for its conflicts, $while among the jobs read again, says the conflicts may come from it: $says",
  async ({ job, says }) => {
    const user = userEvent.setup();
    const { server, rework } = large();
    rework.tooLarge = false;
    rework.conflicts = [{ node_id: notes.id, name: "Notes", reason: "changed" }];
    renderApp(changesetPath(rework.id), server.app);
    await user.click(await screen.findByRole("button", { name: "Undo change" }));
    const dialog = await screen.findByRole("alertdialog", { name: "Undo this change?" });
    // Started elsewhere since the page showed: the jobs are read as the undo is refused.
    server.jobs = [job];
    const listed = server.asked.filter((each) => each.startsWith("GET jobs")).length;

    await user.click(within(dialog).getByRole("button", { name: "Undo" }));

    const heading = await within(dialog).findByRole("heading", { level: 3 });
    expect(document.activeElement).toBe(heading);
    expect(server.asked.filter((each) => each.startsWith("GET jobs")).length).toBe(listed + 1);
    const note = within(dialog).queryByText(
      "An import, a restore or an undo is under way in this notebook: wait for it to end, or cancel it, then undo again. The conflicts below may come from it."
    );
    expect(note !== null).toBe(says);
    if (note !== null) {
      // Before the nodes listed.
      expect(
        note.compareDocumentPosition(within(dialog).getByRole("list", { name: "Nodes in the way" })) &
          Node.DOCUMENT_POSITION_FOLLOWING
      ).toBeTruthy();
    }
  }
);
