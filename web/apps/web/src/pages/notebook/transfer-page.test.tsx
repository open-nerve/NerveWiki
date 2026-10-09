import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import type { TransferFailure, TransferJob } from "../../services/transfer.service";
import { instanceJSON, json, notebookJSON, problem } from "../../test/fakes";
import { addressOf, cancelled, jobJSON, jobsServer, underWay } from "../../test/jobs-server";
import { bob, guide } from "../../test/page-server";
import { renderApp } from "../../test/render";
import { pollInterval } from "./transfer-page";

// A notebook's imports and exports (M7/P5 design 4.3, 4.4).

afterEach(() => vi.useRealTimers());

const settings = `/lab/notebooks/${notebookJSON.id}/settings`;
const transfer = `${settings}/transfer`;
const jobsPath = `GET /api/v0/notebooks/${notebookJSON.id}/transfer-jobs`;
const list = () => screen.findByRole("list", { name: "Recent jobs" });
const startsWith = (prefix: string) => (name: string) => name.startsWith(prefix);
/** The row of the job whose name starts with name: what it does. */
const row = async (name: string) => within(await list()).findByRole("listitem", { name: startsWith(name) });
/** The control of row named by its visible text and the row's name. */
const control = (of: HTMLElement, role: "button" | "link", text: string) =>
  within(of).queryByRole(role, { name: startsWith(`${text} `) });
/** The rows of the list, not the items of a report shown in one. */
async function items(): Promise<HTMLElement[]> {
  return [...(await list()).querySelectorAll<HTMLElement>(":scope > li")];
}
/** The rows of the list, by their place: each must be there. */
async function rowsAt(...places: number[]): Promise<HTMLElement[]> {
  const all = await items();
  return places.map((place) => {
    const item = all[place];
    if (item === undefined) {
      throw new Error(`no row ${place.toString()}`);
    }
    return item;
  });
}
/** What each row says, in order, as its paragraphs say it. */
async function rows(): Promise<string[][]> {
  return (await items()).map((item) => [...item.querySelectorAll("p")].map((line) => line.textContent ?? ""));
}
const listReads = (server: { asked: string[] }) => server.asked.filter((ask) => ask.startsWith("GET jobs")).length;
/** withAddress is the job 1 succeeded, its address expiring at expires. */
const withAddress = (expires: string) => jobJSON(1, { download: addressOf(jobJSON(1), expires) });
const started = "Started Oct 5, 2026, 8:00 AM";
const ended = "Ended Oct 5, 2026, 10:20 AM";

test("whoever sees the notebook comes to its imports and exports from its settings: the third section", async () => {
  const user = userEvent.setup();
  const { router } = renderApp(`${settings}/general`, jobsServer({ role: "reader" }).app);

  const nav = await screen.findByRole("navigation", { name: "Notebook settings" });
  expect(
    within(nav)
      .getAllByRole("link")
      .map((link) => link.textContent)
  ).toEqual(["General", "Members", "Import and export"]);
  await user.click(within(nav).getByRole("link", { name: "Import and export" }));

  expect(await screen.findByRole("heading", { level: 2, name: "Export" })).toBeTruthy();
  expect(router.state.location.pathname).toBe(transfer);
  expect(within(nav).getByRole("link", { name: "Import and export" }).getAttribute("aria-current")).toBe("page");
  expect(document.title).toBe("Import and export · Plans settings · Nerve Wiki");
  expect(await screen.findByText("No jobs yet.")).toBeTruthy();
  expect(screen.getByRole("button", { name: "Export the whole notebook" })).toBeTruthy();
});

test("each job says what it does, its state and progress, who started it, when, and its size", async () => {
  const jobs: TransferJob[] = [
    underWay(jobJSON(1), 0, 0),
    {
      ...underWay(jobJSON(2, { root_id: guide.id, name: "Guide", created_by: bob }), 1, 4),
      cancel_requested_at: "2026-10-05T09:11:00Z",
    },
    jobJSON(3, { result_bytes: 1536 }),
    jobJSON(4, { state: "failed", report: { ...jobJSON(4).report!, failure: "storage_full" } }),
    jobJSON(5, { state: "failed", report: { ...jobJSON(5).report!, failure: "gremlins" as TransferFailure } }),
    jobJSON(6, { state: "cancelled" }),
    jobJSON(7, { state: "expired" }),
    jobJSON(8, { kind: "import", name: "Vault" }),
  ];
  renderApp(transfer, jobsServer({ jobs }).app);

  expect(await rows()).toEqual([
    ["Export of the whole notebook", "Queued", started],
    ["Export of the page Guide", "Running · Cancelling…", `Started by Bob · ${started}`],
    ["Export of the whole notebook", "Done", `${started} · ${ended} · 1.5 KB · Kept until Oct 6, 2026, 10:20 AM`],
    ["Export of the whole notebook", "Failed · The server's storage is full.", `${started} · ${ended}`],
    ["Export of the whole notebook", "Failed · The job failed.", `${started} · ${ended}`],
    ["Export of the whole notebook", "Cancelled", `${started} · ${ended}`],
    ["Export of the whole notebook", "Expired", `${started} · ${ended} · 2 KB`],
    ["Import of Vault", "Done", `${started} · ${ended}`],
  ]);
  const [queued, running, done, expired, imported] = (await rowsAt(0, 1, 2, 6, 7)) as [
    HTMLElement,
    HTMLElement,
    HTMLElement,
    HTMLElement,
    HTMLElement,
  ];
  // Queued, its total not known yet: the progress is indeterminate, named by the state.
  expect(within(queued).getByRole("progressbar", { name: "Queued" }).hasAttribute("value")).toBe(false);
  const progress = within(running).getByRole("progressbar", { name: "Progress: 1 of 4" });
  expect([progress.getAttribute("value"), progress.getAttribute("max")]).toEqual(["1", "4"]);
  // Under way, a job cancels until its cancel is asked, and has no report yet.
  expect([control(queued, "button", "Cancel"), control(running, "button", "Cancel")].map(Boolean)).toEqual([
    true,
    false,
  ]);
  for (const under of [queued, running]) {
    expect([control(under, "button", "Report"), control(under, "link", "Download")]).toEqual([null, null]);
  }
  // An export that succeeded downloads at its address; one expired, and an import, have nothing to download.
  const download = control(done, "link", "Download");
  expect([download?.getAttribute("href"), download?.hasAttribute("download")]).toEqual([
    addressOf(jobJSON(3)).url,
    true,
  ]);
  expect([control(done, "button", "Cancel"), control(expired, "link", "Download")]).toEqual([null, null]);
  expect(control(imported, "link", "Download")).toBeNull();
});

test("a failure says why by its code, one the page does not know as a failure all the same", async () => {
  const failures = [
    "interrupted",
    "timeout",
    "forbidden",
    "root_not_found",
    "storage_full",
    "contributor_conflict",
    "internal",
    "gremlins",
  ];
  const jobs = failures.map((failure, i) =>
    jobJSON(i + 1, { state: "failed", report: { ...jobJSON(1).report!, failure: failure as TransferFailure } })
  );
  renderApp(transfer, jobsServer({ jobs }).app);

  expect((await rows()).map(([, state]) => state)).toEqual([
    "Failed · The job was interrupted: the server stopped or restarted, or the job stopped responding.",
    "Failed · The job ran past the server's time limit.",
    "Failed · The person who started it could no longer read the notebook.",
    "Failed · The page exported no longer exists.",
    "Failed · The server's storage is full.",
    "Failed · A file the server adds had the name of a page.",
    "Failed · The server failed.",
    "Failed · The job failed.",
  ]);
});

test("each row, and each of its controls, is named by what the job does, who started it when not the account, and when; twins by their id too", async () => {
  const jobs = [
    jobJSON(1),
    jobJSON(2, { root_id: guide.id, name: "Guide", created_by: bob, created_at: "2026-10-05T07:00:00Z" }),
    underWay(jobJSON(3), 0, 0),
    // Not twins: the same starter at another time; the same time by another starter.
    jobJSON(4, { created_at: "2026-10-05T09:00:00Z" }),
    jobJSON(5, { created_by: bob }),
  ];
  renderApp(transfer, jobsServer({ jobs }).app);

  const [mine, bobs, twin] = (await rowsAt(0, 1, 2)) as [HTMLElement, HTMLElement, HTMLElement];
  const at = "Oct 5, 2026, 8:00 AM";
  expect((await items()).map((item) => item.getAttribute("aria-label"))).toEqual([
    `Export of the whole notebook, ${at} (ID 000701)`,
    "Export of the page Guide, Bob, Oct 5, 2026, 7:00 AM",
    `Export of the whole notebook, ${at} (ID 000703)`,
    "Export of the whole notebook, Oct 5, 2026, 9:00 AM",
    `Export of the whole notebook, Bob, ${at}`,
  ]);
  expect([
    control(mine, "link", "Download")?.getAttribute("aria-label"),
    control(bobs, "button", "Report")?.getAttribute("aria-label"),
    control(twin, "button", "Cancel")?.getAttribute("aria-label"),
  ]).toEqual([
    `Download Export of the whole notebook, ${at} (ID 000701)`,
    "Report on Export of the page Guide, Bob, Oct 5, 2026, 7:00 AM",
    `Cancel Export of the whole notebook, ${at} (ID 000703)`,
  ]);
  // The row describes it by its state and its details.
  const described = (mine.getAttribute("aria-describedby") ?? "")
    .split(" ")
    .map((id) => document.getElementById(id)?.textContent);
  expect(described).toEqual(["Done", `${started} · ${ended} · 2 KB · Kept until Oct 6, 2026, 10:20 AM`]);
});

test("a queued job cancels at once, a running one says it is cancelling; Cancel gone, the focus is on the row", async () => {
  const user = userEvent.setup();
  const server = jobsServer({
    jobs: [underWay(jobJSON(1), 0, 0), underWay(jobJSON(2, { root_id: guide.id, name: "Guide" }), 1, 4)],
  });
  renderApp(transfer, server.app);
  const queued = await row("Export of the whole notebook");
  const running = await row("Export of the page Guide");

  await user.click(control(queued, "button", "Cancel") as HTMLElement);
  await waitFor(() => expect(document.activeElement).toBe(queued));
  await user.click(control(running, "button", "Cancel") as HTMLElement);
  await waitFor(() => expect(document.activeElement).toBe(running));

  expect((await rows()).map(([, state]) => state)).toEqual(["Cancelled", "Running · Cancelling…"]);
  expect(server.asked.filter((ask) => ask.startsWith("CANCEL"))).toEqual(["CANCEL Plans", "CANCEL Guide"]);
  expect([control(queued, "button", "Cancel"), control(running, "button", "Cancel")]).toEqual([null, null]);
});

test("Cancel, pressed again while it sends, sends once: busy, and still focusable", async () => {
  const user = userEvent.setup();
  let answer: (() => void) | undefined;
  const job = underWay(jobJSON(1), 0, 0);
  const server = jobsServer({
    jobs: [job],
    answers: {
      "POST /api/v0/transfer-jobs/*/cancel": () => {
        server.asked.push("CANCEL");
        return new Promise<Response>((resolve) => {
          answer = () => {
            server.jobs = [cancelled(job)];
            resolve(json(cancelled(job)));
          };
        });
      },
    },
  });
  renderApp(transfer, server.app);
  const row1 = await row("Export of the whole notebook");
  const cancel = control(row1, "button", "Cancel") as HTMLElement;

  await user.click(cancel);
  await user.click(cancel);

  expect([
    cancel.getAttribute("aria-disabled"),
    cancel.getAttribute("aria-busy"),
    cancel.hasAttribute("disabled"),
  ]).toEqual(["true", "true", false]);
  expect(document.activeElement).toBe(cancel);
  act(() => answer?.());
  await waitFor(async () => expect((await rows()).map(([, state]) => state)).toEqual(["Cancelled"]));
  expect(server.asked.filter((ask) => ask === "CANCEL")).toHaveLength(1);
  expect(within(row1).queryByRole("alert")).toBeNull();
});

test("a cancel refused says why in the row, and the jobs are read again at once: the job ended meanwhile", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  const server = jobsServer({ jobs: [underWay(jobJSON(1), 1, 4)] });
  renderApp(transfer, server.app);
  const job = await row("Export of the whole notebook");
  // Read again as a second goes, the next read is a second away: one before it is the refusal's, while the click and
  // the refusal take less than that second of the test's time, as they do by tens of milliseconds.
  await act(() => vi.advanceTimersByTimeAsync(1_000));
  const before = listReads(server);

  server.jobs = [jobJSON(1)];
  await user.click(control(job, "button", "Cancel") as HTMLElement);

  expect((await within(job).findByRole("alert")).textContent).toBe("This job has already ended.");
  expect(listReads(server)).toBeGreaterThan(before);
  await waitFor(async () => expect((await rows()).map(([, state]) => state)).toEqual(["Done"]));
  expect(document.activeElement).toBe(job);
});

test("a download whose address the read again finds expired goes, the focus on its row", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-10-06T08:00:00Z"));
  const server = jobsServer({ jobs: [withAddress("2026-10-06T08:02:00Z")] });
  renderApp(transfer, server.app);
  const job = await row("Export of the whole notebook");
  act(() => control(job, "link", "Download")?.focus());

  server.jobs = [jobJSON(1, { state: "expired" })];
  await act(() => vi.advanceTimersByTimeAsync(60_000));

  await waitFor(() => expect(control(job, "link", "Download")).toBeNull());
  expect(document.activeElement).toBe(job);
});

test("the report of an ended job shows its counts and where the vault differs, read as it opens", async () => {
  const user = userEvent.setup();
  const counts = { pages: 5, attachments: 3, renamed: 2, missing: 1, skipped: 0 };
  const job = jobJSON(1, { report: { failure: null, counts } });
  const server = jobsServer({ jobs: [job] });
  server.problems.set(job.id, {
    problems: [
      { path: "Plans/a:b.md", code: "renamed", to: "Plans/a_b.md" },
      { path: "Plans/x.png", code: "file_missing", to: null },
      { path: "Plans/y", code: "gremlins" as "renamed", to: null },
    ],
    problems_truncated: true,
  });
  renderApp(transfer, server.app);
  const report = control(await row("Export of the whole notebook"), "button", "Report") as HTMLElement;
  expect(report.getAttribute("aria-expanded")).toBe("false");
  expect(server.asked).not.toContain("GET job Plans");

  await user.click(report);

  expect(report.getAttribute("aria-expanded")).toBe("true");
  const shown = document.getElementById(report.getAttribute("aria-controls") ?? "") as HTMLElement;
  expect(
    [...shown.querySelectorAll("dt")].map((term) => `${term.textContent}: ${term.nextElementSibling?.textContent}`)
  ).toEqual(["Pages: 5", "Attachments: 3", "Renamed: 2", "Missing files: 1"]);
  const problems = await within(shown).findByRole("list", { name: "Where the vault differs" });
  expect(
    within(problems)
      .getAllByRole("listitem")
      .map((item) => item.textContent)
  ).toEqual([
    "Plans/a:b.md is written as Plans/a_b.md: links to its old name do not reach it in Obsidian.",
    "Plans/x.png: the attachment's file was not in the storage, and is not in the zip.",
    "Plans/y differs in the vault.",
  ]);
  expect(within(shown).getByText("Only the first 1,000 are listed.")).toBeTruthy();
  expect(server.asked).toContain("GET job Plans");

  await user.click(report);
  expect(report.getAttribute("aria-expanded")).toBe("false");
  expect(report.hasAttribute("aria-controls")).toBe(false);
  expect(screen.queryByText("Only the first 1,000 are listed.")).toBeNull();
});

test("a report whose problems cannot be read says why, its counts shown; Try again reads it", async () => {
  const user = userEvent.setup();
  let down = true;
  const server = jobsServer({
    jobs: [jobJSON(1)],
    answers: {
      "GET /api/v0/transfer-jobs/*": () =>
        down
          ? Promise.reject(new TypeError("offline"))
          : json({ ...jobJSON(1), problems: [], problems_truncated: false }),
    },
  });
  renderApp(transfer, server.app);
  const job = await row("Export of the whole notebook");

  await user.click(control(job, "button", "Report") as HTMLElement);

  expect((await within(job).findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );
  expect(within(job).getByText("Pages")).toBeTruthy();
  down = false;
  await user.click(within(job).getByRole("button", { name: "Try again" }));
  await waitFor(() => expect(within(job).queryByRole("alert")).toBeNull());
  // Nothing befell the vault: no problems are listed.
  expect(within(job).queryByText("Where the vault differs")).toBeNull();
});

test("jobs that cannot be read say why, and Try again reads them; a read again that fails says so above the jobs held", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  const server = jobsServer({ jobs: [underWay(jobJSON(1), 1, 4)] });
  server.listDown = true;
  renderApp(transfer, server.app);
  const section = (await screen.findByRole("heading", { name: "Recent jobs" })).closest("section") as HTMLElement;

  expect((await within(section).findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );
  server.listDown = false;
  await user.click(within(section).getByRole("button", { name: "Try again" }));
  expect(await rows()).toHaveLength(1);

  // A read again fails: the job stays as it was read, and the section says why.
  server.listDown = true;
  await act(() => vi.advanceTimersByTimeAsync(1_000));
  expect((await within(section).findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );
  expect(await rows()).toHaveLength(1);
  server.listDown = false;
  server.jobs = [jobJSON(1)];
  await user.click(within(section).getByRole("button", { name: "Try again" }));
  await waitFor(async () => expect((await rows()).map(([, state]) => state)).toEqual(["Done"]));
  expect(within(section).queryByRole("alert")).toBeNull();
});

test("Load more adds the next page's jobs; with the last read, the focus goes to the first it added", async () => {
  const user = userEvent.setup();
  const server = jobsServer({
    jobs: [6, 5, 4, 3, 2, 1].map((n) => jobJSON(n)),
    pageSize: 2,
  });
  renderApp(transfer, server.app);
  expect(await rows()).toHaveLength(2);
  const more = screen.getByRole("button", { name: "Load more" });

  // Not the last page: the focus does not move, even from the body (a click that leaves it there, as Safari's).
  fireEvent.click(more);
  await waitFor(async () => expect(await rows()).toHaveLength(4));
  expect(document.activeElement).toBe(document.body);
  await user.click(more);

  await waitFor(async () => expect(await rows()).toHaveLength(6));
  expect(screen.queryByRole("button", { name: "Load more" })).toBeNull();
  await waitFor(async () => expect(document.activeElement).toBe((await items())[4]));
  expect(server.asked.filter((ask) => ask.startsWith("GET jobs"))).toEqual([
    "GET jobs 50",
    `GET jobs 50 after ${jobJSON(5).id}`,
    `GET jobs 50 after ${jobJSON(3).id}`,
  ]);
});

test("Load more, the last page adding none of its jobs, gives the focus to the list's last", async () => {
  const user = userEvent.setup();
  let admin = true;
  const server = jobsServer({
    answers: {
      // An admin no more, Ada sees her jobs alone: the page after the two held has none of them.
      [jobsPath]: (request) =>
        json(
          new URL(request.url).searchParams.has("cursor") && !admin
            ? { data: [], next_cursor: null }
            : { data: [jobJSON(3), jobJSON(2)], next_cursor: jobJSON(2).id }
        ),
    },
  });
  renderApp(transfer, server.app);
  expect(await rows()).toHaveLength(2);

  admin = false;
  await user.click(screen.getByRole("button", { name: "Load more" }));

  await waitFor(() => expect(screen.queryByRole("button", { name: "Load more" })).toBeNull());
  await waitFor(async () => expect(document.activeElement).toBe((await items())[1]));
});

test("Load more, the reader moving meanwhile, leaves the focus on the section's title as it goes", async () => {
  const user = userEvent.setup();
  let answer: (() => void) | undefined;
  const server = jobsServer({
    jobs: [jobJSON(2), jobJSON(1)],
    pageSize: 1,
    answers: {
      [jobsPath]: (request) => {
        const cursor = new URL(request.url).searchParams.get("cursor");
        if (cursor === null) {
          return json({ data: [jobJSON(2)], next_cursor: "1" });
        }
        return new Promise<Response>((resolve) => {
          answer = () => resolve(json({ data: [jobJSON(1)], next_cursor: null }));
        });
      },
    },
  });
  renderApp(transfer, server.app);
  await rows();

  const more = screen.getByRole("button", { name: "Load more" });
  await user.click(more);
  await waitFor(() => expect(answer).toBeDefined());
  // Busy, it stays focusable.
  expect([more.getAttribute("aria-disabled"), more.getAttribute("aria-busy"), more.hasAttribute("disabled")]).toEqual([
    "true",
    "true",
    false,
  ]);
  fireEvent.wheel(document.body);
  act(() => answer?.());

  await waitFor(async () => expect(await rows()).toHaveLength(2));
  expect(document.activeElement).toBe(screen.getByRole("heading", { name: "Recent jobs" }));
});

test("a page of jobs that cannot be read says why beside Load more, which tries again", async () => {
  const user = userEvent.setup();
  let down = true;
  renderApp(
    transfer,
    jobsServer({
      answers: {
        [jobsPath]: (request) => {
          const cursor = new URL(request.url).searchParams.get("cursor");
          if (cursor !== null && down) {
            return Promise.reject(new TypeError("offline"));
          }
          return json(
            cursor === null ? { data: [jobJSON(2)], next_cursor: "1" } : { data: [jobJSON(1)], next_cursor: null }
          );
        },
      },
    }).app
  );
  expect(await rows()).toHaveLength(1);

  await user.click(screen.getByRole("button", { name: "Load more" }));
  expect((await screen.findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );
  down = false;
  await user.click(screen.getByRole("button", { name: "Load more" }));

  await waitFor(async () => expect(await rows()).toHaveLength(2));
  expect(screen.queryByRole("alert")).toBeNull();
});

test("the jobs are read every second while one is under way, not once all ended, until a minute before an address expires", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-10-06T08:00:00Z"));
  const server = jobsServer({ jobs: [underWay(jobJSON(1), 1, 4)] });
  renderApp(transfer, server.app);
  await screen.findByRole("progressbar", { name: "Progress: 1 of 4" });
  const first = listReads(server);

  // Each second is a read: not one in two, as SWR's deduplication would make it.
  server.jobs = [underWay(jobJSON(1), 3, 4)];
  await act(() => vi.advanceTimersByTimeAsync(1_000));
  await act(() => vi.advanceTimersByTimeAsync(1_000));
  expect(listReads(server)).toBeGreaterThanOrEqual(first + 2);
  expect(screen.getByRole("progressbar", { name: "Progress: 3 of 4" })).toBeTruthy();

  // It succeeded: its address expires at nine.
  server.jobs = [withAddress("2026-10-06T09:00:00Z")];
  await act(() => vi.advanceTimersByTimeAsync(1_000));
  expect(control(await row("Export of the whole notebook"), "link", "Download")).not.toBeNull();
  const done = listReads(server);
  await act(() => vi.advanceTimersByTimeAsync(30_000));
  expect(listReads(server)).toBe(done);

  await act(() => vi.advanceTimersByTimeAsync(28 * 60_000));
  expect(listReads(server)).toBe(done);

  // A minute before nine, it is read again: its address signed anew.
  server.jobs = [withAddress("2026-10-06T10:00:00Z")];
  await act(() => vi.advanceTimersByTimeAsync(31 * 60_000));
  expect(listReads(server)).toBe(done + 1);
});

test("the jobs, read no more once all ended, are read every second again once an export starts", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  const server = jobsServer({ jobs: [jobJSON(1, { state: "failed" })] });
  renderApp(transfer, server.app);
  await row("Export of the whole notebook");
  await act(() => vi.advanceTimersByTimeAsync(5_000));
  const idle = listReads(server);
  expect(idle).toBe(1);

  await user.click(screen.getByRole("button", { name: "Export the whole notebook" }));
  await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Export" }));
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  await act(() => vi.advanceTimersByTimeAsync(1_000));
  await act(() => vi.advanceTimersByTimeAsync(1_000));

  expect(listReads(server)).toBeGreaterThanOrEqual(idle + 2);
});

test("pollInterval: a second while a job is under way; else a minute before the first address expires, from 30 seconds to an hour; else never", () => {
  const now = Date.parse("2026-10-06T08:00:00Z");

  expect(pollInterval({ active: true, jobs: [withAddress("2026-10-06T09:00:00Z")] }, now)).toBe(1_000);
  expect(
    pollInterval(
      { active: false, jobs: [withAddress("2026-10-06T10:00:00Z"), withAddress("2026-10-06T09:00:00Z")] },
      now
    )
  ).toBe(59 * 60_000);
  expect(pollInterval({ active: false, jobs: [withAddress("2026-10-06T08:01:00Z")] }, now)).toBe(30_000);
  expect(pollInterval({ active: false, jobs: [withAddress("2026-10-06T12:00:00Z")] }, now)).toBe(60 * 60_000);
  expect(
    pollInterval({ active: false, jobs: [withAddress("not a time"), withAddress("2026-10-06T09:00:00Z")] }, now)
  ).toBe(59 * 60_000);
  // No address: ended without one, expired, or an import.
  const over = (["failed", "cancelled", "expired"] as const).map((state) => jobJSON(1, { state }));
  expect(pollInterval({ active: false, jobs: [...over, jobJSON(2, { kind: "import" })] }, now)).toBe(0);
  expect(pollInterval({ active: false, jobs: [withAddress("not a time")] }, now)).toBe(0);
  expect(pollInterval({ active: false, jobs: undefined }, now)).toBe(0);
});

test("the whole notebook exports, and again: each job goes first in the list, its row taking the focus", async () => {
  const user = userEvent.setup();
  const server = jobsServer({ jobs: [jobJSON(1)] });
  renderApp(transfer, server.app);
  await list();

  for (const time of [1, 2]) {
    // oxlint-disable-next-line no-await-in-loop -- one export after the other
    await user.click(screen.getByRole("button", { name: "Export the whole notebook" }));
    // oxlint-disable-next-line no-await-in-loop -- its dialog
    const dialog = await screen.findByRole("alertdialog", { name: "Export Plans?" });
    expect(within(dialog).getByText(/can be downloaded for 24 hours;/)).toBeTruthy();
    // oxlint-disable-next-line no-await-in-loop -- confirmed
    await user.click(within(dialog).getByRole("button", { name: "Export" }));

    // oxlint-disable-next-line no-await-in-loop -- closed
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    // oxlint-disable-next-line no-await-in-loop -- its row
    await waitFor(async () => expect(document.activeElement).toBe((await items())[0]));
    // oxlint-disable-next-line no-await-in-loop -- the rows
    expect((await rows())[0]).toEqual(["Export of the whole notebook", "Queued", "Started Oct 6, 2026, 8:00 AM"]);
    expect(server.asked.filter((ask) => ask === "POST export notebook")).toHaveLength(time);
    // The job ends before the next export.
    server.jobs = server.jobs.map((job) => (job.state === "queued" ? cancelled(job) : job));
  }
});

test("the jobs whose read failed are read again as an export starts: no waiting for Try again", async () => {
  const user = userEvent.setup();
  const server = jobsServer({ jobs: [jobJSON(1)] });
  server.listDown = true;
  renderApp(transfer, server.app);
  await screen.findByRole("alert");
  server.listDown = false;

  await user.click(screen.getByRole("button", { name: "Export the whole notebook" }));
  await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Export" }));

  await waitFor(async () => expect((await rows()).map(([, state]) => state)).toEqual(["Queued", "Done"]));
  expect(screen.queryByRole("alert")).toBeNull();
});

test("exported while the jobs cannot be read, the focus goes to their title", async () => {
  const user = userEvent.setup();
  const server = jobsServer();
  server.listDown = true;
  renderApp(transfer, server.app);
  await screen.findByRole("alert");

  await user.click(screen.getByRole("button", { name: "Export the whole notebook" }));
  await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Export" }));

  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("heading", { name: "Recent jobs" })));
});

test.each([
  ["no whole hours", { ...instanceJSON, export_ttl_seconds: 5_400 }, /can be downloaded for 90 minutes;/],
  ["the instance not read", undefined, /can be downloaded for a while;/],
])("an export says how long it is kept: %s", async (_, info, said) => {
  const user = userEvent.setup();
  renderApp(
    transfer,
    jobsServer({
      answers: {
        "GET /api/v0/instance": () => (info === undefined ? problem(404, "not_found") : json(info)),
      },
    }).app
  );

  await user.click(await screen.findByRole("button", { name: "Export the whole notebook" }));

  expect(within(await screen.findByRole("alertdialog")).getByText(said)).toBeTruthy();
});

test.each([
  [
    "an export of hers under way",
    () => problem(409, "transfer.busy"),
    "You already have an export of this notebook under way. Wait for it to end, or cancel it.",
  ],
  [
    "the queue full",
    () => problem(503, "server_busy"),
    "Too many jobs are waiting on the server. Try again in a few minutes.",
  ],
  [
    "the storage full",
    () => problem(507, "storage_full"),
    "The server has no room for more files. Ask its administrator.",
  ],
  [
    "the notebook gone",
    () => problem(404, "notebook.not_found"),
    "This notebook does not exist, or you have no access to it.",
  ],
])("an export refused stays in the dialog and says why: %s", async (_, refusal, said) => {
  const user = userEvent.setup();
  let refuse: (() => void) | undefined;
  const server = jobsServer({
    answers: {
      [`POST /api/v0/notebooks/${notebookJSON.id}/exports`]: () =>
        new Promise<Response>((resolve) => {
          refuse = () => resolve(refusal());
        }),
    },
  });
  renderApp(transfer, server.app);

  await user.click(await screen.findByRole("button", { name: "Export the whole notebook" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Export Plans?" });
  await user.click(within(dialog).getByRole("button", { name: "Export" }));
  expect(await within(dialog).findByRole("button", { name: "Starting…" })).toBeTruthy();
  act(() => refuse?.());

  expect((await within(dialog).findByRole("alert")).textContent).toBe(said);
  expect(screen.getByRole("alertdialog")).toBe(dialog);
  expect(screen.getByText("No jobs yet.")).toBeTruthy();
});

test("reached from a page's export, the job's row takes the focus as it shows, once", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
  const job = underWay(jobJSON(1, { root_id: guide.id, name: "Guide" }), 0, 0);
  renderApp([{ pathname: transfer, state: { focusJob: job.id } }], jobsServer({ jobs: [jobJSON(2), job] }).app);

  const arrived = await row("Export of the page Guide");
  await waitFor(() => expect(document.activeElement).toBe(arrived));

  // The reader moves on: the job read again does not take the focus back.
  await user.click(screen.getByText(/^Download the notebook as a zip/));
  await act(() => vi.advanceTimersByTimeAsync(1_000));
  expect(document.activeElement).toBe(document.body);
});

test("reached from a page's export, a list read without the job gives up: the job shown later takes no focus", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const job = underWay(jobJSON(1), 0, 0);
  const server = jobsServer({ jobs: [underWay(jobJSON(2), 0, 0)] });
  renderApp([{ pathname: transfer, state: { focusJob: job.id } }], server.app);
  await row("Export of the whole notebook");

  server.jobs = [job, ...server.jobs];
  await act(() => vi.advanceTimersByTimeAsync(1_000));

  await waitFor(async () => expect(await rows()).toHaveLength(2));
  expect(document.activeElement).toBe(document.body);
});

test("reached from a page's export, the focus stays where the reader put it before the job showed", async () => {
  const user = userEvent.setup();
  const job = underWay(jobJSON(1), 0, 0);
  let answer: (() => void) | undefined;
  const server = jobsServer({
    jobs: [job],
    answers: {
      [jobsPath]: () =>
        new Promise<Response>((resolve) => {
          answer = () => resolve(json({ data: [job], next_cursor: null }));
        }),
    },
  });
  renderApp([{ pathname: transfer, state: { focusJob: job.id } }], server.app);
  const body = await screen.findByText(/^Download the notebook as a zip/);

  // The reader clicks the page as the jobs are read: the focus is on the body still.
  await user.click(body);
  expect(document.activeElement).toBe(document.body);
  await waitFor(() => expect(answer).toBeDefined());
  act(() => answer?.());

  await row("Export of the whole notebook");
  // The effects of the list shown have run.
  await act(async () => {});
  expect(document.activeElement).toBe(document.body);
});

test("reached from a page's export, the focus stays where it was put before the job showed", async () => {
  const job = underWay(jobJSON(1), 0, 0);
  let answer: (() => void) | undefined;
  const server = jobsServer({
    jobs: [job],
    answers: {
      [jobsPath]: () =>
        new Promise<Response>((resolve) => {
          answer = () => resolve(json({ data: [job], next_cursor: null }));
        }),
    },
  });
  renderApp([{ pathname: transfer, state: { focusJob: job.id } }], server.app);
  const exporting = await screen.findByRole("button", { name: "Export the whole notebook" });

  // Not by the reader's input: by the page, say.
  act(() => exporting.focus());
  await waitFor(() => expect(answer).toBeDefined());
  act(() => answer?.());

  await row("Export of the whole notebook");
  await act(async () => {});
  expect(document.activeElement).toBe(exporting);
});

test("from one notebook's jobs straight to another's, each reads its own", async () => {
  const atlas = { ...notebookJSON, id: "0199a2b4-0000-7000-8000-0000000000c2", name: "Atlas" };
  const server = jobsServer({
    jobs: [jobJSON(1)],
    answers: {
      "GET /api/v0/workspaces/lab/notebooks": () => json({ data: [notebookJSON, atlas] }),
      [`GET /api/v0/notebooks/${atlas.id}/transfer-jobs`]: () =>
        json({ data: [jobJSON(2, { kind: "import", name: "Vault", notebook_id: atlas.id })], next_cursor: null }),
    },
  });
  const { router } = renderApp(transfer, server.app);
  expect((await rows()).map(([title]) => title)).toEqual(["Export of the whole notebook"]);

  await act(() => router.navigate(`/lab/notebooks/${atlas.id}/settings/transfer`));

  await waitFor(async () => expect((await rows()).map(([title]) => title)).toEqual(["Import of Vault"]));
  expect(screen.getByRole("heading", { level: 1, name: "Atlas settings" })).toBeTruthy();
});
