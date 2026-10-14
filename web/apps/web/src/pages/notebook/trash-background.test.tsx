import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { eventHandlers } from "../../events/handlers";
import type { NotebookRole } from "../../services/notebook.service";
import { eventServer, withEvents } from "../../test/event-server";
import { json, notebookJSON, problem, workspaceJSON, type Answer } from "../../test/fakes";
import { jobJSON, jobsServer, reportOf } from "../../test/jobs-server";
import { assetNode, guide, install, linux, notes } from "../../test/page-server";
import { readAgain } from "../../test/page-panel";
import { renderApp } from "../../test/render";
import type { FakeTrashItem } from "../../test/trash-server";

// A trash item too large to restore here, restored in the background from the trash and from its preview (M8/P4
// design 4.2).

afterEach(() => {
  vi.useRealTimers();
});

const home = `/lab/notebooks/${notebookJSON.id}`;
const trashPath = `${home}/trash`;
const previewPath = (id: string) => `${trashPath}/${id}`;
const transfer = `${home}/settings/transfer`;
const restores = `POST /api/v0/notebooks/${notebookJSON.id}/trash-restores`;

/**
 * large is jobsServer with three items in its trash, newest first: the
 * attachment a.png of Guide; Notes, at the top level; and Linux, under
 * Guide / Install, of a history too long to restore here. Ada's role in
 * Plans is role, answers answering instead of it.
 */
function large(role: NotebookRole = "admin", answers: Record<string, Answer> = {}) {
  const picture = assetNode(20, "a.png", guide);
  const server = jobsServer({ role, answers, nodes: [guide, install, linux, notes, picture] });
  const item = server.trashNode(linux.id);
  item.history = 400_001;
  server.trashNode(notes.id);
  server.trashNode(picture.id);
  return { server, item };
}

/** gate is a wait a test opens: what waits for its promise goes on once open is called. */
function gate() {
  let open!: () => void;
  const promise = new Promise<void>((resolve) => {
    open = resolve;
  });
  return { promise, open };
}

const tooLarge =
  "“Linux” is too large to restore here: it holds more pages, attachments, content, links or versions than one restore brings back. Pages: 1, attachments: 0.";
const started = "Restoring “Linux” in the background has started. See its progress";

/** restoreOf is the Restore of the item named name, the first of its rows. */
const restoreOf = (name: string) => screen.getAllByRole("button", { name: new RegExp(`^Restore “${name}” \\(`) })[0];

/** startsOf are the restores in the background that went out. */
const startsOf = (asked: string[]) => asked.filter((each) => each.startsWith("POST restore"));

/** olderRestore is Ada's restore of Linux in the background, made and ended long before: none of a press now. */
const olderRestore = () =>
  jobJSON(9, {
    kind: "trash_restore",
    root_id: linux.id,
    name: "Linux",
    state: "cancelled",
    progress: { done: 0, total: 103 },
    report: reportOf(null),
  });

/** refused restores Linux, once the trash lists it: too large, it answers Restore in the background. */
async function refused(user: ReturnType<typeof userEvent.setup>): Promise<HTMLElement> {
  await screen.findByRole("list", { name: "Trash" });
  await user.click(restoreOf("Linux") as HTMLElement);
  await waitFor(() => expect(screen.getByRole("alert").textContent).toBe(tooLarge));
  return screen.getByRole("button", { name: "Restore “Linux” in the background (From: Guide / Install)" });
}

test("a restore refused as too large says so above the list, with Restore in the background, named by where the item was; the row's Restore stays", async () => {
  const user = userEvent.setup();
  const { server } = large();
  renderApp(trashPath, server.app);

  const start = await refused(user);

  expect(start.textContent).toBe("Restore “Linux” in the background");
  expect(restoreOf("Linux")).toBeTruthy();
  expect(startsOf(server.asked)).toEqual([]);
  expect(screen.getByRole("status").textContent).toBe("");
});

test("Restore in the background tells twins apart as their rows' Restore does: by when each was deleted", async () => {
  const user = userEvent.setup();
  const again = { ...notes, id: "0199a2b4-0000-7000-8000-000000000140" };
  const server = jobsServer({ nodes: [guide, notes] });
  server.trashNode(notes.id);
  server.nodes = [...server.nodes, again];
  server.trashNode(again.id).history = 400_001;
  renderApp(trashPath, server.app);
  await screen.findByRole("list", { name: "Trash" });

  await user.click(
    screen.getByRole("button", {
      name: /^Restore “Notes” \(From: the notebook's top level, deleted Oct 4, 2026, 8:02 AM\)$/u,
    })
  );

  const start = await screen.findByRole("button", {
    name: "Restore “Notes” in the background (From: the notebook's top level, deleted Oct 4, 2026, 8:02 AM)",
  });
  expect(start.textContent).toBe("Restore “Notes” in the background");
});

test("Restore in the background tells twins deleted in one minute apart by the end of their ids", async () => {
  const user = userEvent.setup();
  const again = { ...notes, id: "0199a2b4-0000-7000-8000-000000000140" };
  const server = jobsServer({ nodes: [guide, notes] });
  server.trashNode(notes.id).deletedAt = "2026-10-04T08:01:05.000Z";
  server.nodes = [...server.nodes, again];
  const item = server.trashNode(again.id);
  item.deletedAt = "2026-10-04T08:01:40.000Z";
  item.history = 400_001;
  renderApp(trashPath, server.app);
  await screen.findByRole("list", { name: "Trash" });

  await user.click(
    screen.getByRole("button", {
      name: "Restore “Notes” (From: the notebook's top level, deleted Oct 4, 2026, 8:01 AM, ID 000140)",
    })
  );

  expect(
    await screen.findByRole("button", {
      name: "Restore “Notes” in the background (From: the notebook's top level, deleted Oct 4, 2026, 8:01 AM, ID 000140)",
    })
  ).toBeTruthy();
});

test("Restore in the background starts the item's restore: the line says so, leading to its progress, the title taking the focus, described by it; the refusal goes; the trash is read again; there, the job's row takes the focus", async () => {
  const user = userEvent.setup();
  const { server } = large();
  renderApp(trashPath, server.app);
  const start = await refused(user);
  const before = server.sent.length;

  await user.click(start);

  const status = screen.getByRole("status");
  await waitFor(() => expect(status.textContent).toBe(started));
  const heading = screen.getByRole("heading", { level: 1, name: "Trash" });
  expect(document.activeElement).toBe(heading);
  expect(heading.getAttribute("aria-describedby")).toBe(status.id);
  expect(screen.queryByRole("alert")).toBeNull();
  expect(screen.queryByRole("button", { name: /in the background/u })).toBeNull();
  expect(startsOf(server.asked)).toEqual(["POST restore Linux"]);
  const job = server.jobs[0];
  expect([job?.kind, job?.state, job?.root_id, job?.name]).toEqual(["trash_restore", "queued", linux.id, "Linux"]);
  await waitFor(() => expect(server.sent.slice(before)).toContain("GET trash"));
  const progress = within(status).getByRole("link", { name: "See its progress" });
  expect(progress.getAttribute("href")).toBe(transfer);

  await user.click(progress);
  const row = await screen.findByRole("listitem", {
    name: (name) => name.startsWith("Restore of Linux from the trash, "),
  });
  await waitFor(() => expect(document.activeElement).toBe(row));
});

test.each<[string, (server: ReturnType<typeof large>["server"]) => void, string, boolean]>([
  [
    "a job that writes the tree under way, transfer.busy",
    (server) => {
      server.busy = true;
    },
    "An import, a restore or an undo is under way in this notebook: wait for it to end, then try again.",
    true,
  ],
  [
    "the queue full, server_busy",
    (server) => {
      server.queueFull = true;
    },
    "Too many jobs are waiting on the server. Try again in a few minutes.",
    true,
  ],
  [
    "restored elsewhere meanwhile, page.not_in_trash",
    (server) => void server.restoreElsewhere(linux.id),
    "“Linux” changed meanwhile: the trash shows it as it is now.",
    false,
  ],
  [
    "past its time, page.restore_expired",
    (server) => {
      (server.trash.get(linux.id) as FakeTrashItem).expired = true;
    },
    "“Linux” can no longer be restored: its time in the trash is over.",
    false,
  ],
  [
    "deleted again with the page above it, page.not_trash_root",
    (server) => {
      server.restoreElsewhere(linux.id);
      server.trashNode(install.id);
    },
    "“Linux” was deleted again with a page above it: restore that page, which holds it now.",
    false,
  ],
])(
  "a restore in the background refused as %s says why above the list; offered still, its button keeps the focus, else it gives the focus to the title",
  async (_, refuse, text, offered) => {
    const user = userEvent.setup();
    const { server } = large();
    renderApp(trashPath, server.app);
    const start = await refused(user);
    refuse(server);

    await user.click(start);

    await waitFor(() => expect(screen.getByRole("alert").textContent).toBe(text));
    expect(startsOf(server.asked)).toEqual(["POST restore Linux"]);
    expect(screen.getByRole("status").textContent).toBe("");
    expect(server.jobs).toEqual([]);
    if (!offered) {
      await waitFor(() => expect(screen.queryByRole("button", { name: /in the background/u })).toBeNull());
      expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Trash" }));
      return;
    }
    expect(document.activeElement).toBe(start);
    await waitFor(() => expect(start.getAttribute("aria-busy")).toBeNull());
    Object.assign(server, { busy: false, queueFull: false });
    await user.click(start);
    await waitFor(() => expect(screen.getByRole("status").textContent).toBe(started));
  }
);

test("a restore in the background refused, the role taken away meanwhile, says why, and reads the notebooks again: a reader by now, the Restores go, the focus to the title", async () => {
  const user = userEvent.setup();
  let role: NotebookRole = "admin";
  const reads: string[] = [];
  const { server } = large("admin", {
    "GET /api/v0/workspaces/lab/notebooks": () => {
      reads.push("GET notebooks");
      return json({ data: [{ ...notebookJSON, role }] });
    },
    [restores]: () => problem(403, "forbidden"),
  });
  renderApp(trashPath, server.app);
  const start = await refused(user);
  role = "reader";
  reads.length = 0;

  await user.click(start);

  await waitFor(() => expect(screen.getByRole("alert").textContent).toBe("You do not have permission to do this."));
  await waitFor(() => expect(screen.queryByRole("button", { name: /^Restore/u })).toBeNull());
  expect(reads).toEqual(["GET notebooks"]);
  expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Trash" }));
});

test("a restore in the background refused as its notebook is gone says why, and reads the notebooks again", async () => {
  const user = userEvent.setup();
  const reads: string[] = [];
  const { server } = large("admin", {
    "GET /api/v0/workspaces/lab/notebooks": () => {
      reads.push("GET notebooks");
      return json({ data: [notebookJSON] });
    },
  });
  renderApp(trashPath, server.app);
  const start = await refused(user);
  server.notebookGone = true;
  reads.length = 0;

  await user.click(start);

  await waitFor(() =>
    expect(screen.getByRole("alert").textContent).toBe("This notebook does not exist, or you have no access to it.")
  );
  expect(reads).toEqual(["GET notebooks"]);
});

test("a writer made a reader as the refusal shows no longer has Restore in the background", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  let role: NotebookRole = "admin";
  const { server } = large("admin", {
    "GET /api/v0/workspaces/lab/notebooks": () => json({ data: [{ ...notebookJSON, role }] }),
  });
  renderApp(trashPath, server.app);
  await refused(user);

  role = "reader";
  await readAgain();

  await waitFor(() => expect(screen.queryByRole("button", { name: /in the background/u })).toBeNull());
  expect(screen.getByRole("alert").textContent).toBe(tooLarge);
});

// M8/P4 design 4.2, review B-M6.
test.each([0, 300, 3000])(
  "a restore in the background whose answer was lost reads the jobs again: its job there, made by a server's clock %i ms behind the browser's, the line says it started, leading to its progress, the title taking the focus",
  async (behind) => {
    const user = userEvent.setup();
    const { server } = large();
    renderApp(trashPath, server.app);
    const start = await refused(user);
    Object.assign(server, { lost: "network", clockBehind: behind });

    await user.click(start);

    const status = screen.getByRole("status");
    await waitFor(() => expect(status.textContent).toBe(started));
    expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Trash" }));
    expect(within(status).getByRole("link", { name: "See its progress" }).getAttribute("href")).toBe(transfer);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(server.asked.filter((each) => each.startsWith("GET jobs"))).toHaveLength(1);
  }
);

test("a restore in the background whose answer was lost, none of the jobs read again its own (Ada's restore of it ended long before), says it did not start; still offered, as busy as the jobs are read", async () => {
  const user = userEvent.setup();
  const { server } = large("admin", { [restores]: () => problem(500, "internal_error") });
  server.jobs = [olderRestore()];
  renderApp(trashPath, server.app);
  const start = await refused(user);
  const listed = gate();
  server.listWaits = listed.promise;

  await user.click(start);

  await waitFor(() => expect(server.asked.filter((each) => each.startsWith("GET jobs"))).toHaveLength(1));
  expect(start.getAttribute("aria-busy")).toBe("true");
  act(() => listed.open());
  await waitFor(() =>
    expect(screen.getByRole("status").textContent).toBe(
      "Restoring “Linux” in the background did not start: you can try again."
    )
  );
  expect(screen.getByRole("alert").textContent).toBe(tooLarge);
  expect(document.activeElement).toBe(start);
  await waitFor(() => expect(start.getAttribute("aria-busy")).toBeNull());
});

test("a restore in the background whose answer was lost, the jobs not read, says it is not known whether it started, leading to the jobs", async () => {
  const user = userEvent.setup();
  const { server } = large();
  renderApp(trashPath, server.app);
  const start = await refused(user);
  Object.assign(server, { lost: "500", listDown: true });

  await user.click(start);

  const status = screen.getByRole("status");
  await waitFor(() =>
    expect(status.textContent).toBe(
      "Whether restoring “Linux” in the background started is not known: the jobs could not be read. See the jobs"
    )
  );
  expect(within(status).getByRole("link", { name: "See the jobs" }).getAttribute("href")).toBe(transfer);
  expect(screen.getByRole("button", { name: /in the background/u })).toBe(start);
});

test("Restore in the background pressed twice sends one start: busy while it is out", async () => {
  const user = userEvent.setup();
  const { server } = large();
  renderApp(trashPath, server.app);
  const start = await refused(user);
  server.startsHeld = true;

  await user.dblClick(start);
  await user.click(start);

  await waitFor(() => expect(startsOf(server.asked)).toEqual(["POST restore Linux"]));
  expect([
    start.getAttribute("aria-busy"),
    start.getAttribute("aria-disabled"),
    start.hasAttribute("disabled"),
  ]).toEqual(["true", "true", false]);
  act(() => server.release());
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe(started));
  expect(startsOf(server.asked)).toEqual(["POST restore Linux"]);
});

test("a restore in the background answered after the reader moved the focus leaves it where they put it", async () => {
  const user = userEvent.setup();
  const { server } = large();
  renderApp(trashPath, server.app);
  const start = await refused(user);
  server.startsHeld = true;
  await user.click(start);
  await waitFor(() => expect(startsOf(server.asked)).toEqual(["POST restore Linux"]));

  const next = restoreOf("Notes") as HTMLElement;
  act(() => next.focus());
  act(() => server.release());

  await waitFor(() => expect(screen.getByRole("status").textContent).toBe(started));
  expect(document.activeElement).toBe(next);
});

test("Restore in the background pressed by a click that leaves the focus where it was, as Safari's, gives the title the focus as it starts", async () => {
  const user = userEvent.setup();
  const { server } = large();
  renderApp(trashPath, server.app);
  const start = await refused(user);
  act(() => (document.activeElement as HTMLElement).blur());

  fireEvent.click(start);

  await waitFor(() => expect(screen.getByRole("status").textContent).toBe(started));
  expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Trash" }));
});

test("a restore here after one refused as too large says nothing of it: the refusal and its offer go as it is sent", async () => {
  const user = userEvent.setup();
  const { server } = large();
  renderApp(trashPath, server.app);
  await refused(user);
  server.restoresHeld = true;

  await user.click(restoreOf("Notes") as HTMLElement);

  expect(screen.queryByRole("alert")).toBeNull();
  expect(screen.queryByRole("button", { name: /in the background/u })).toBeNull();
  act(() => server.release());
  await waitFor(() => expect(screen.getByRole("status").textContent).toContain("Restored “Notes”"));
});

test("the trash arrived at from a restore started in the background in an item's preview says so, its title taking the focus", async () => {
  const user = userEvent.setup();
  const { server, item } = large();
  const { router } = renderApp(previewPath(linux.id), server.app);
  const restore = await screen.findByRole("button", { name: "Restore “Linux” (From: Guide / Install)" });

  await user.click(restore);

  expect((await screen.findByRole("alert")).textContent).toBe(tooLarge);
  const start = screen.getByRole("button", { name: "Restore “Linux” in the background (From: Guide / Install)" });
  expect(start.textContent).toBe("Restore “Linux” in the background");
  expect(document.activeElement).toBe(restore);
  await user.click(start);

  const heading = await screen.findByRole("heading", { level: 1, name: "Trash" });
  expect(router.state.location.pathname).toBe(trashPath);
  await waitFor(() => expect(document.activeElement).toBe(heading));
  const status = screen.getByRole("status");
  expect(status.textContent).toBe(started);
  expect(heading.getAttribute("aria-describedby")).toBe(status.id);
  expect(within(status).getByRole("link", { name: "See its progress" }).getAttribute("href")).toBe(transfer);
  expect(startsOf(server.asked)).toEqual(["POST restore Linux"]);
  // Still in the trash until the job's first batch.
  expect(server.trash.get(linux.id)).toBe(item);
});

test.each<[string, (server: ReturnType<typeof large>["server"]) => void, string]>([
  [
    "a job that writes the tree under way",
    (server) => {
      server.busy = true;
    },
    "An import, a restore or an undo is under way in this notebook: wait for it to end, then try again.",
  ],
  [
    "the queue full",
    (server) => {
      server.queueFull = true;
    },
    "Too many jobs are waiting on the server. Try again in a few minutes.",
  ],
])(
  "a restore in the background from the preview refused as %s says why above it, the refusal describing its title; offered still, its button keeps the focus; the item is read again",
  async (_, refuse, text) => {
    const user = userEvent.setup();
    const { server } = large();
    renderApp(previewPath(linux.id), server.app);
    await user.click(await screen.findByRole("button", { name: "Restore “Linux” (From: Guide / Install)" }));
    const start = await screen.findByRole("button", { name: /^Restore “Linux” in the background/u });
    refuse(server);
    const before = server.sent.length;

    await user.click(start);

    await waitFor(() => expect(screen.getByRole("alert").textContent).toBe(text));
    const heading = screen.getByRole("heading", { level: 1, name: "Linux" });
    expect(heading.getAttribute("aria-describedby")).toBe(screen.getByRole("alert").id);
    expect(screen.getByRole("button", { name: /in the background/u })).toBe(start);
    expect(document.activeElement).toBe(start);
    await waitFor(() => expect(server.sent.slice(before)).toContain("GET trash item Linux"));
  }
);

// As a restore from the preview refused (M8/P2B review A-M1): what the start was refused for is said where the item
// was, the trash's title taking the focus (P4B review A-M2, C-M1).
test.each([
  {
    meanwhile: "restored elsewhere",
    change: (server: ReturnType<typeof large>["server"]) => void server.restoreElsewhere(linux.id),
    refusal: "“Linux” changed meanwhile: the trash shows it as it is now.",
    text: "This is no longer in the trash.",
  },
  {
    meanwhile: "past its time",
    change: (server: ReturnType<typeof large>["server"]) => {
      (server.trash.get(linux.id) as FakeTrashItem).expired = true;
    },
    refusal: "“Linux” can no longer be restored: its time in the trash is over.",
    text: "This can no longer be restored: its time in the trash is over.",
  },
])(
  "a restore in the background from the preview of an item $meanwhile says why, and what became of it, the trash's title taking the focus, described by the refusal",
  async ({ change, refusal, text }) => {
    const user = userEvent.setup();
    const { server } = large();
    renderApp(previewPath(linux.id), server.app);
    await user.click(await screen.findByRole("button", { name: "Restore “Linux” (From: Guide / Install)" }));
    const start = await screen.findByRole("button", { name: /^Restore “Linux” in the background/u });
    change(server);

    await user.click(start);

    expect(await screen.findByText(text)).toBeTruthy();
    const alert = screen.getByRole("alert");
    expect(alert.textContent).toBe(refusal);
    const heading = screen.getByRole("heading", { level: 1, name: "Trash" });
    await waitFor(() => expect(document.activeElement).toBe(heading));
    expect(heading.getAttribute("aria-describedby")).toBe(alert.id);
    expect(screen.queryByRole("button", { name: /^Restore/u })).toBeNull();
  }
);

test("a restore in the background from the preview of an item deleted again with a page above it goes to that item's preview, which says why, its title taking the focus", async () => {
  const user = userEvent.setup();
  const { server } = large();
  const { router } = renderApp(previewPath(linux.id), server.app);
  await user.click(await screen.findByRole("button", { name: "Restore “Linux” (From: Guide / Install)" }));
  const start = await screen.findByRole("button", { name: /^Restore “Linux” in the background/u });
  server.restoreElsewhere(linux.id);
  server.trashNode(install.id);

  await user.click(start);

  const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
  expect(router.state.location.pathname).toBe(previewPath(install.id));
  const alert = screen.getByRole("alert");
  expect(alert.textContent).toBe(
    "What you opened was deleted again with a page above it: it is part of this item now, and is restored with it."
  );
  await waitFor(() => expect(document.activeElement).toBe(heading));
  expect(heading.getAttribute("aria-describedby")).toBe(alert.id);
});

test("a restore in the background from the preview refused, the role taken away meanwhile, says why: a reader by now, the Restores go, the focus to the title", async () => {
  const user = userEvent.setup();
  let role: NotebookRole = "admin";
  const { server } = large("admin", {
    "GET /api/v0/workspaces/lab/notebooks": () => json({ data: [{ ...notebookJSON, role }] }),
    [restores]: () => problem(403, "forbidden"),
  });
  renderApp(previewPath(linux.id), server.app);
  await user.click(await screen.findByRole("button", { name: "Restore “Linux” (From: Guide / Install)" }));
  const start = await screen.findByRole("button", { name: /^Restore “Linux” in the background/u });
  role = "reader";

  await user.click(start);

  expect((await screen.findByRole("alert")).textContent).toBe("You do not have permission to do this.");
  await waitFor(() => expect(screen.queryByRole("button", { name: /^Restore/u })).toBeNull());
  expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Linux" }));
});

test("a restore in the background from the preview refused as its notebook is gone reads it and the notebooks again: the item is no page", async () => {
  const user = userEvent.setup();
  const reads: string[] = [];
  const { server } = large("admin", {
    "GET /api/v0/workspaces/lab/notebooks": () => {
      reads.push("GET notebooks");
      return json({ data: [notebookJSON] });
    },
  });
  renderApp(previewPath(linux.id), server.app);
  await user.click(await screen.findByRole("button", { name: "Restore “Linux” (From: Guide / Install)" }));
  const start = await screen.findByRole("button", { name: /^Restore “Linux” in the background/u });
  server.notebookGone = true;
  reads.length = 0;

  await user.click(start);

  // The item read again, of a notebook gone, is no page, as after a restore here refused so.
  expect(await screen.findByRole("heading", { level: 1, name: "Page not found" })).toBeTruthy();
  expect(reads).toEqual(["GET notebooks"]);
});

test("a restore in the background from the preview whose answer was lost, none of the jobs its own (Ada's restore of it ended long before), says it did not start above it, still offered", async () => {
  const user = userEvent.setup();
  const { server } = large("admin", { [restores]: () => problem(502, "bad_gateway") });
  server.jobs = [olderRestore()];
  renderApp(previewPath(linux.id), server.app);
  await user.click(await screen.findByRole("button", { name: "Restore “Linux” (From: Guide / Install)" }));
  const start = await screen.findByRole("button", { name: /^Restore “Linux” in the background/u });

  await user.click(start);

  await waitFor(() =>
    expect(screen.getByRole("alert").textContent).toBe(
      "Restoring “Linux” in the background did not start: you can try again."
    )
  );
  expect(screen.getByRole("button", { name: /in the background/u })).toBe(start);
  expect(document.activeElement).toBe(start);
});

test("a restore in the background from the preview whose answer was lost, the jobs not read, says so above it, leading to the jobs", async () => {
  const user = userEvent.setup();
  const { server } = large();
  renderApp(previewPath(linux.id), server.app);
  await user.click(await screen.findByRole("button", { name: "Restore “Linux” (From: Guide / Install)" }));
  const start = await screen.findByRole("button", { name: /^Restore “Linux” in the background/u });
  Object.assign(server, { lost: "500", listDown: true });

  await user.click(start);

  const alert = await screen.findByRole("alert", {
    name: undefined,
  });
  await waitFor(() =>
    expect(alert.textContent).toBe(
      "Whether restoring “Linux” in the background started is not known: the jobs could not be read. See the jobs"
    )
  );
  expect(within(alert).getByRole("link", { name: "See the jobs" }).getAttribute("href")).toBe(transfer);
});

// M8/P2B review A-I1: what the job's first batch does reaches the preview as the start's outcome is found out.
test("a restore in the background from the preview whose answer was lost keeps the item shown as the jobs are read, though the job's first batch took it from the trash; then goes back to the trash, which says it started", async () => {
  const user = userEvent.setup();
  const events = eventServer();
  const { server } = large("admin", { "GET /api/v0/events": events.answer });
  const { router } = renderApp(previewPath(linux.id), withEvents(server.app), { eventHandlers });
  await user.click(await screen.findByRole("button", { name: "Restore “Linux” (From: Guide / Install)" }));
  const start = await screen.findByRole("button", { name: /^Restore “Linux” in the background/u });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  events.last().hello();
  await act(() => new Promise((resolve) => setTimeout(resolve, 100)));
  const listed = gate();
  Object.assign(server, { lost: "500", listWaits: listed.promise });
  const itemReads = () => server.sent.filter((each) => each === "GET trash item Linux").length;

  await user.click(start);
  await waitFor(() => expect(server.asked.filter((each) => each.startsWith("GET jobs"))).toHaveLength(1));
  // The job's first batch restores the item, its event read as the jobs are.
  server.restoreElsewhere(linux.id);
  const before = itemReads();
  events.last().send("pages", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, tree: true, pages: [] });
  await waitFor(() => expect(itemReads()).toBe(before + 1));
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  expect(screen.getByRole("heading", { level: 1, name: "Linux" })).toBeTruthy();
  expect(screen.queryByText("This is no longer in the trash.")).toBeNull();
  act(() => listed.open());

  const heading = await screen.findByRole("heading", { level: 1, name: "Trash" });
  expect(router.state.location.pathname).toBe(trashPath);
  await waitFor(() => expect(document.activeElement).toBe(heading));
  expect(screen.getByRole("status").textContent).toBe(started);
});

test("Restore in the background pressed twice on the preview sends one start", async () => {
  const user = userEvent.setup();
  const { server } = large();
  renderApp(previewPath(linux.id), server.app);
  await user.click(await screen.findByRole("button", { name: "Restore “Linux” (From: Guide / Install)" }));
  const start = await screen.findByRole("button", { name: /^Restore “Linux” in the background/u });
  server.startsHeld = true;

  await user.dblClick(start);

  await waitFor(() => expect(startsOf(server.asked)).toEqual(["POST restore Linux"]));
  expect(start.getAttribute("aria-busy")).toBe("true");
  act(() => server.release());
  expect(await screen.findByRole("heading", { level: 1, name: "Trash" })).toBeTruthy();
  expect(startsOf(server.asked)).toEqual(["POST restore Linux"]);
});
