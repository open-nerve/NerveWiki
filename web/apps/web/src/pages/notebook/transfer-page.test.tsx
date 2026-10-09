import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import type { TransferFailure, TransferJob } from "../../services/transfer.service";
import { instanceJSON, json, notebookJSON, problem } from "../../test/fakes";
import { jobJSON, jobsServer, underWay } from "../../test/jobs-server";
import { bob, guide } from "../../test/page-server";
import { renderApp } from "../../test/render";
import { pollInterval } from "./transfer-page";

// A notebook's imports and exports (M7/P5 design 4.3, 4.4).

afterEach(() => vi.useRealTimers());

const settings = `/lab/notebooks/${notebookJSON.id}/settings`;
const transfer = `${settings}/transfer`;
const list = () => screen.findByRole("list", { name: "Recent jobs" });
/** The row of the job named name, by what it does. */
const row = async (name: string) => within(await list()).findByRole("listitem", { name });
/** What each row says, in order, as its paragraphs say it. */
async function rows(): Promise<string[][]> {
  return within(await list())
    .getAllByRole("listitem")
    .map((item) => [...item.querySelectorAll("p")].map((line) => line.textContent ?? ""));
}
const listReads = (server: { asked: string[] }) => server.asked.filter((ask) => ask.startsWith("GET jobs")).length;
const address = (job: TransferJob, expires = "2100-01-01T00:00:00Z") => ({
  url: `/api/v0/transfer-jobs/${job.id}/download?e=1&s=sig`,
  expires_at: expires,
});
/** withAddress is the job 1 succeeded, its address expiring at expires. */
const withAddress = (expires: string) => ({ ...jobJSON(1), download: address(jobJSON(1), expires) });

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
  const exported = jobJSON(3, { result_bytes: 1536 });
  const jobs: TransferJob[] = [
    underWay(jobJSON(1), 0, 0),
    {
      ...underWay(jobJSON(2, { root_id: guide.id, name: "Guide", created_by: bob }), 1, 4),
      cancel_requested_at: "2026-10-05T08:00:03Z",
    },
    { ...exported, download: address(exported) },
    jobJSON(4, { state: "failed", result_bytes: null, report: { ...exported.report!, failure: "storage_full" } }),
    jobJSON(5, {
      state: "failed",
      result_bytes: null,
      report: { ...exported.report!, failure: "gremlins" as TransferFailure },
    }),
    jobJSON(6, { state: "cancelled", result_bytes: null }),
    jobJSON(7, { state: "expired" }),
    jobJSON(8, { kind: "import", name: "Vault", result_bytes: null }),
  ];
  renderApp(transfer, jobsServer({ jobs }).app);

  const started = "Started Oct 5, 2026, 8:00 AM";
  const ended = "Ended Oct 5, 2026, 8:00 AM";
  expect(await rows()).toEqual([
    ["Export of the whole notebook", "Queued", started],
    ["Export of the page Guide", "Running · Cancelling…", `Started by Bob · ${started}`],
    ["Export of the whole notebook", "Done", `${started} · ${ended} · 1.5 KB · Kept until Oct 6, 2026, 8:00 AM`],
    ["Export of the whole notebook", "Failed · The server's storage is full.", `${started} · ${ended}`],
    ["Export of the whole notebook", "Failed · The job failed.", `${started} · ${ended}`],
    ["Export of the whole notebook", "Cancelled", `${started} · ${ended}`],
    ["Export of the whole notebook", "Expired", `${started} · ${ended} · 2 KB`],
    ["Import of Vault", "Done", `${started} · ${ended}`],
  ]);
  const [queued, running, done] = within(await list()).getAllByRole("listitem") as HTMLElement[];
  // Queued, its total not known yet: the progress is indeterminate, named by the state.
  const waiting = within(queued as HTMLElement).getByRole("progressbar", { name: "Queued" });
  expect(waiting.hasAttribute("value")).toBe(false);
  const progress = within(running as HTMLElement).getByRole("progressbar", { name: "Progress: 1 of 4" });
  expect([progress.getAttribute("value"), progress.getAttribute("max")]).toEqual(["1", "4"]);
  // Under way, a job cancels and has no report yet; an export that succeeded downloads at its address.
  for (const under of [queued, running] as HTMLElement[]) {
    expect(within(under).getByRole("button", { name: "Cancel" })).toBeTruthy();
    expect(within(under).queryByRole("button", { name: "Report" })).toBeNull();
    expect(within(under).queryByRole("link", { name: "Download" })).toBeNull();
  }
  const download = within(done as HTMLElement).getByRole("link", { name: "Download" });
  expect([download.getAttribute("href"), download.hasAttribute("download")]).toEqual([address(exported).url, true]);
  expect(within(done as HTMLElement).queryByRole("button", { name: "Cancel" })).toBeNull();
  expect(within(await row("Import of Vault")).queryByRole("link", { name: "Download" })).toBeNull();
});

test("a queued job cancels at once; a running one says it is cancelling", async () => {
  const user = userEvent.setup();
  const server = jobsServer({
    jobs: [underWay(jobJSON(1), 0, 0), underWay(jobJSON(2, { root_id: guide.id, name: "Guide" }), 1, 4)],
  });
  renderApp(transfer, server.app);

  await user.click(within(await row("Export of the whole notebook")).getByRole("button", { name: "Cancel" }));
  await user.click(within(await row("Export of the page Guide")).getByRole("button", { name: "Cancel" }));

  await waitFor(async () =>
    expect((await rows()).map(([, state]) => state)).toEqual(["Cancelled", "Running · Cancelling…"])
  );
  expect(server.asked.filter((ask) => ask.startsWith("CANCEL"))).toEqual(["CANCEL Plans", "CANCEL Guide"]);
  expect(within(await row("Export of the whole notebook")).queryByRole("button", { name: "Cancel" })).toBeNull();
});

test("a cancel refused says why in the row, and the jobs are read again: the job ended meanwhile", async () => {
  const user = userEvent.setup();
  const server = jobsServer({ jobs: [underWay(jobJSON(1), 1, 4)] });
  renderApp(transfer, server.app);
  const job = await row("Export of the whole notebook");

  server.jobs = [jobJSON(1)];
  await user.click(within(job).getByRole("button", { name: "Cancel" }));

  expect((await within(job).findByRole("alert")).textContent).toBe("This job has already ended.");
  await waitFor(async () => expect((await rows()).map(([, state]) => state)).toEqual(["Done"]));
});

test("the report of an ended job shows its counts and where the vault differs, read as it opens", async () => {
  const user = userEvent.setup();
  const counts = { pages: 4, attachments: 2, renamed: 1, missing: 1, skipped: 0 };
  const job = jobJSON(1, { report: { failure: null, counts } });
  const server = jobsServer({ jobs: [job] });
  server.problems.set(job.id, {
    problems: [
      { path: "Plans/a:b.md", code: "renamed", to: "Plans/a_b.md" },
      { path: "Plans/x.png", code: "file_missing", to: null },
    ],
    problems_truncated: true,
  });
  renderApp(transfer, server.app);
  const report = within(await row("Export of the whole notebook")).getByRole("button", { name: "Report" });
  expect(report.getAttribute("aria-expanded")).toBe("false");
  expect(server.asked).not.toContain("GET job Plans");

  await user.click(report);

  expect(report.getAttribute("aria-expanded")).toBe("true");
  const shown = document.getElementById(report.getAttribute("aria-controls") ?? "") as HTMLElement;
  expect(
    [...shown.querySelectorAll("dt")].map((term) => `${term.textContent}: ${term.nextElementSibling?.textContent}`)
  ).toEqual(["Pages: 4", "Attachments: 2", "Renamed: 1", "Missing files: 1"]);
  const problems = await within(shown).findByRole("list");
  expect(
    within(problems)
      .getAllByRole("listitem")
      .map((item) => item.textContent)
  ).toEqual([
    "Plans/a:b.md is written as Plans/a_b.md: links to its old name do not reach it in Obsidian.",
    "Plans/x.png: the attachment's file was not in the storage, and is not in the zip.",
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

  await user.click(within(job).getByRole("button", { name: "Report" }));

  expect((await within(job).findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );
  expect(within(job).getByText("Pages")).toBeTruthy();
  down = false;
  await user.click(within(job).getByRole("button", { name: "Try again" }));
  await waitFor(() => expect(within(job).queryByRole("alert")).toBeNull());
  // Nothing befell the vault: no problems are listed.
  expect(within(job).queryByText("What the vault does otherwise")).toBeNull();
});

test("Load more adds the next page's jobs; with the last read, the focus goes to the first it added", async () => {
  const user = userEvent.setup();
  const server = jobsServer({
    jobs: [jobJSON(3), jobJSON(2), jobJSON(1, { kind: "import", name: "Vault" })],
    pageSize: 2,
  });
  renderApp(transfer, server.app);
  expect(await rows()).toHaveLength(2);

  await user.click(screen.getByRole("button", { name: "Load more" }));

  const added = await row("Import of Vault");
  expect(await rows()).toHaveLength(3);
  expect(screen.queryByRole("button", { name: "Load more" })).toBeNull();
  await waitFor(() => expect(document.activeElement).toBe(added));
  expect(server.asked.filter((ask) => ask.startsWith("GET jobs"))).toEqual(["GET jobs 50", "GET jobs 50 after 2"]);
});

test("a page of jobs that cannot be read says why beside Load more, which tries again", async () => {
  const user = userEvent.setup();
  let down = true;
  renderApp(
    transfer,
    jobsServer({
      answers: {
        [`GET /api/v0/notebooks/${notebookJSON.id}/transfer-jobs`]: (request) => {
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
  const running = underWay(jobJSON(1), 1, 4);
  const server = jobsServer({ jobs: [running] });
  renderApp(transfer, server.app);
  await screen.findByRole("progressbar", { name: "Progress: 1 of 4" });
  const first = listReads(server);

  server.jobs = [underWay(jobJSON(1), 3, 4)];
  await act(() => vi.advanceTimersByTimeAsync(1_000));
  expect(await screen.findByRole("progressbar", { name: "Progress: 3 of 4" })).toBeTruthy();
  expect(listReads(server)).toBeGreaterThan(first);

  // It succeeded: its address expires at nine.
  server.jobs = [withAddress("2026-10-06T09:00:00Z")];
  await act(() => vi.advanceTimersByTimeAsync(1_000));
  expect(await screen.findByRole("link", { name: "Download" })).toBeTruthy();
  const ended = listReads(server);
  await act(() => vi.advanceTimersByTimeAsync(30_000));
  expect(listReads(server)).toBe(ended);

  await act(() => vi.advanceTimersByTimeAsync(28 * 60_000));
  expect(listReads(server)).toBe(ended);

  // A minute before nine, it is read again: its address signed anew.
  server.jobs = [withAddress("2026-10-06T10:00:00Z")];
  await act(() => vi.advanceTimersByTimeAsync(31 * 60_000));
  expect(listReads(server)).toBe(ended + 1);
});

test("pollInterval: a second while a job is under way; else a minute before the first address expires, at least 30 seconds; else never", () => {
  const now = Date.parse("2026-10-06T08:00:00Z");

  expect(pollInterval({ active: true, jobs: [withAddress("2026-10-06T09:00:00Z")] }, now)).toBe(1_000);
  expect(
    pollInterval(
      { active: false, jobs: [withAddress("2026-10-06T10:00:00Z"), withAddress("2026-10-06T09:00:00Z")] },
      now
    )
  ).toBe(59 * 60_000);
  expect(pollInterval({ active: false, jobs: [withAddress("2026-10-06T08:01:00Z")] }, now)).toBe(30_000);
  expect(pollInterval({ active: false, jobs: [jobJSON(1)] }, now)).toBe(0);
  expect(pollInterval({ active: false, jobs: undefined }, now)).toBe(0);
});

test("the whole notebook exports: the job goes first in the list, its row taking the focus", async () => {
  const user = userEvent.setup();
  const server = jobsServer({ jobs: [jobJSON(1)] });
  renderApp(transfer, server.app);
  await list();

  await user.click(screen.getByRole("button", { name: "Export the whole notebook" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Export Plans?" });
  expect(within(dialog).getByText(/can be downloaded for 24 hours;/)).toBeTruthy();
  await user.click(within(dialog).getByRole("button", { name: "Export" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  const [first] = within(await list()).getAllByRole("listitem");
  await waitFor(() => expect(document.activeElement).toBe(first));
  expect((await rows())[0]).toEqual(["Export of the whole notebook", "Queued", "Started Oct 6, 2026, 8:00 AM"]);
  expect(server.asked).toContain("POST export notebook");
});

test("an export kept a while that is no whole hours says it in minutes", async () => {
  const user = userEvent.setup();
  renderApp(
    transfer,
    jobsServer({ answers: { "GET /api/v0/instance": () => json({ ...instanceJSON, export_ttl_seconds: 5_400 }) } }).app
  );

  await user.click(await screen.findByRole("button", { name: "Export the whole notebook" }));

  expect(within(await screen.findByRole("alertdialog")).getByText(/can be downloaded for 90 minutes;/)).toBeTruthy();
});

test.each([
  [
    "an export of hers under way",
    problem(409, "transfer.busy"),
    "You already have an export of this notebook under way. Wait for it to end, or cancel it.",
  ],
  [
    "the queue full",
    problem(503, "server_busy"),
    "Too many jobs are waiting on the server. Try again in a few minutes.",
  ],
  ["the storage full", problem(507, "storage_full"), "The server has no room for more files. Ask its administrator."],
])("an export refused stays in the dialog and says why: %s", async (_, refusal, said) => {
  const user = userEvent.setup();
  const server = jobsServer({ answers: { [`POST /api/v0/notebooks/${notebookJSON.id}/exports`]: () => refusal } });
  renderApp(transfer, server.app);

  await user.click(await screen.findByRole("button", { name: "Export the whole notebook" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Export Plans?" });
  await user.click(within(dialog).getByRole("button", { name: "Export" }));

  expect((await within(dialog).findByRole("alert")).textContent).toBe(said);
  expect(screen.getByRole("alertdialog")).toBe(dialog);
  expect(screen.getByText("No jobs yet.")).toBeTruthy();
});

test("reached from a page's export, the job's row takes the focus as it shows", async () => {
  const job = underWay(jobJSON(1, { root_id: guide.id, name: "Guide" }), 0, 0);
  renderApp([{ pathname: transfer, state: { focusJob: job.id } }], jobsServer({ jobs: [jobJSON(2), job] }).app);

  const arrived = await row("Export of the page Guide");

  await waitFor(() => expect(document.activeElement).toBe(arrived));
});

test("reached from a page's export, the focus stays where the reader put it before the job showed", async () => {
  const user = userEvent.setup();
  const job = underWay(jobJSON(1), 0, 0);
  let answer: (() => void) | undefined;
  const server = jobsServer({
    jobs: [job],
    answers: {
      [`GET /api/v0/notebooks/${notebookJSON.id}/transfer-jobs`]: () =>
        new Promise<Response>((resolve) => {
          answer = () => resolve(json({ data: [job], next_cursor: null }));
        }),
    },
  });
  renderApp([{ pathname: transfer, state: { focusJob: job.id } }], server.app);
  const body = await screen.findByText(/^Download the notebook as a zip/);

  // The reader clicks the page as the jobs are read.
  await user.click(body);
  await waitFor(() => expect(answer).toBeDefined());
  act(() => answer?.());

  expect(document.activeElement).not.toBe(await row("Export of the whole notebook"));
});
