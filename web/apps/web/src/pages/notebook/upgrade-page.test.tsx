import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { eventHandlers } from "../../events/handlers";
import type { TransferJob } from "../../services/transfer.service";
import type { Upgrade, UpgradeReport } from "../../services/upgrade.service";
import { eventServer, withEvents } from "../../test/event-server";
import { deferred } from "../../test/health-server";
import { json, notebookJSON, problem, workspaceJSON } from "../../test/fakes";
import { cancelled, reportOf, underWay } from "../../test/jobs-server";
import { bob, guide, install, notes } from "../../test/page-server";
import { renderApp } from "../../test/render";
import { under } from "../../stores/transfer.store";
import {
  bobsUpgrade,
  reportJSON,
  rollbackJobJSON,
  seededAgents,
  seededRaw,
  upgradeJSON,
  upgradeServer,
} from "../../test/upgrade-server";

// The page of a notebook's upgrade under way (M11/P1 design 4.4): its
// admins' alone (v0.1 design 13.2, item 18).

const upgradePath = `/lab/notebooks/${notebookJSON.id}/upgrade`;
const plans = `/api/v0/notebooks/${notebookJSON.id}`;
const adminsOnly = "Only the notebook's admins can see its upgrade.";
const pageOf = (id: string) => `/lab/notebooks/${notebookJSON.id}/pages/${id}`;

afterEach(() => vi.useRealTimers());

test.each(["editor", "reader"] as const)(
  "the upgrade's page reads nothing for a notebook's %s: it says who can see it",
  async (role) => {
    const server = upgradeServer({ role, upgrade: bobsUpgrade() });
    renderApp(upgradePath, server.app);

    expect(await screen.findByRole("heading", { level: 1, name: "Upgrade to an LLM Wiki" })).toBeTruthy();
    expect(await screen.findByText(adminsOnly)).toBeTruthy();
    expect(server.asked).toEqual([]);
  }
);

test("an admin no longer one, refused, reads the notebooks again and is told who can see it", async () => {
  let reads = 0;
  let answer: (() => void) | undefined;
  const server: ReturnType<typeof upgradeServer> = upgradeServer({
    upgrade: bobsUpgrade(),
    answers: {
      "GET /api/v0/workspaces/lab/notebooks": () => {
        reads++;
        return json({ data: [server.notebook] });
      },
      [`GET ${plans}/upgrade`]: async () => {
        server.asked.push("GET upgrade");
        // Another admin made Ada an editor meanwhile.
        await new Promise<void>((resolve) => (answer = resolve));
        server.notebook = { ...server.notebook, role: "editor" };
        return problem(403, "forbidden");
      },
    },
  });
  renderApp(upgradePath, server.app);
  await waitFor(() => expect(answer).toBeTypeOf("function"));
  const before = reads;

  answer?.();

  expect(await screen.findByText(adminsOnly)).toBeTruthy();
  expect(reads).toBe(before + 1);
  expect(server.asked).toEqual(["GET upgrade"]);
});

test("an admin of a notebook with no upgrade under way is told so", async () => {
  const server = upgradeServer();
  renderApp(upgradePath, server.app);

  expect(await screen.findByText("This notebook has no upgrade under way.")).toBeTruthy();
  // Said as what is, not as a failure to load: nothing to try again.
  expect(screen.queryByRole("button", { name: "Try again" })).toBeNull();
  expect(screen.queryByRole("alert")).toBeNull();
  expect(server.asked).toEqual(["GET upgrade"]);
});

test("the upgrade's page says who started it and when, the source area it proposes and the pages it wrote, each a link; the tab is named for it", async () => {
  const server = upgradeServer({ nodes: [guide, install, notes, seededRaw, seededAgents], upgrade: bobsUpgrade() });
  renderApp(upgradePath, server.app);

  const state = (await screen.findByRole("heading", { level: 2, name: "Where it is" })).parentElement as HTMLElement;
  expect(within(state).getByText("Started by Bob on Oct 5, 2026, 8:00 AM.")).toBeTruthy();
  await within(state).findByRole("link", { name: "AGENTS" });
  const links = within(state)
    .getAllByRole("link")
    .map((link) => [link.textContent, link.getAttribute("href")]);
  expect(links).toEqual([
    ["raw", pageOf(seededRaw.id)],
    ["raw", pageOf(seededRaw.id)],
    ["AGENTS", pageOf(seededAgents.id)],
  ]);
  expect(state.textContent).toContain("Its source area: raw");
  expect(state.textContent).toContain("The pages it wrote: raw, AGENTS");
  await waitFor(() => expect(document.title).toBe("Upgrade to an LLM Wiki · Plans · Nerve Wiki"));
});

test("a page the upgrade wrote that the tree no longer has is named nowhere: its agent deleted it", async () => {
  const server = upgradeServer({ nodes: [guide, seededRaw], upgrade: bobsUpgrade() });
  renderApp(upgradePath, server.app);

  const state = (await screen.findByRole("heading", { level: 2, name: "Where it is" })).parentElement as HTMLElement;
  await within(state).findAllByRole("link", { name: "raw" });
  expect(within(state).queryByRole("link", { name: "AGENTS" })).toBeNull();
  expect(state.textContent).toContain("The pages it wrote: raw");
});

// What the page offers by the upgrade's state (decision 248): the agent's
// section while it is under way, the abandonment while it may be
// abandoned, a stopped rollback's continuation and end.

const abandon = "Abandon this upgrade…";
const continues = "Continue the rollback…";
const ends = "End the rollback, keeping the notebook as it is…";
const noneUnderWay = "This notebook has no upgrade under way.";
const rollingBanner = "This notebook's upgrade is being undone: it is read-only for now.";
const linksMayFail =
  "In pages not yet written back, links the system rewrote may lead nowhere; the health panel lists them.";
const nothingLeft = "Everything is back already: ending it is all that is left.";

/** rollbackOf is Plans' upgrade, Ada's, abandoned and rolling back in job. */
const rollbackOf = (job: TransferJob, more: Partial<Upgrade> = {}) =>
  upgradeJSON({ state: "rolling_back", ending: "abandoned", rollback_job_id: job.id, ...more });
/** stopped is the rollback's job n, cancelled as it ran, two of its four nodes put back. */
const stopped = (n: number) =>
  cancelled({ ...underWay(rollbackJobJSON(n), 2, 4), report: reportOf(null, { put_back: 2 }) });
/** How many times the server was asked what. */
const reads = (server: { asked: string[] }, what: string) => server.asked.filter((asked) => asked === what).length;
const title = () => screen.getByRole("heading", { level: 1, name: "Upgrade to an LLM Wiki" });
/** The section headed name. */
const section = async (name: string) =>
  (await screen.findByRole("heading", { level: 2, name })).parentElement as HTMLElement;
/** What a list of counts says: each term and its count. */
const counts = (holder: HTMLElement) =>
  [...holder.querySelectorAll("dt")].map((term) => [term.textContent, term.nextElementSibling?.textContent]);
const leftCounts = [
  ["Nodes not back in place", "2"],
  ["Pages not written back", "1"],
  ["New nodes still there", "3"],
];

test("the upgrade's initiator is told to connect their agent as the README says, and the prompt to give it", async () => {
  const server = upgradeServer({ upgrade: upgradeJSON() });
  renderApp(upgradePath, server.app);

  const agent = await section("Connect an agent");
  expect(agent.textContent).toContain(
    "Until the upgrade ends, only your agent writes this notebook, over MCP. Connect it to this notebook as the README's section on connecting an agent (MCP) says."
  );
  expect(within(agent).getByText("upgrade").tagName).toBe("CODE");
  expect(agent.textContent).toContain("The prompt to give it: upgrade");
});

test("another admin is told whose agent writes and that they can check, submit or abandon the upgrade, not the prompt", async () => {
  const server = upgradeServer({ upgrade: bobsUpgrade() });
  renderApp(upgradePath, server.app);

  const agent = await section("Connect an agent");
  expect(agent.textContent).toBe(
    "Connect an agentOnly the agent of Bob, who started the upgrade, can write this notebook; you can check the upgrade, submit it for review, or abandon it."
  );
  expect(within(agent).queryByText("upgrade")).toBeNull();
});

// In review, Reject stands for Abandon (decision 291).
test.each([
  ["upgrading", true, true],
  ["reviewing", false, false],
  ["rolling_back", false, false],
] as const)(
  "an upgrade %s: the agent's section %s, Abandon %s; a rollback's Continue and End only once it stopped",
  async (state, agent, abandons) => {
    const job = stopped(70);
    const server = upgradeServer({
      jobs: state === "rolling_back" ? [job] : [],
      upgrade: state === "rolling_back" ? rollbackOf(job) : upgradeJSON({ state }),
    });
    renderApp(upgradePath, server.app);

    await section("Where it is");
    if (state === "rolling_back") {
      await screen.findByRole("button", { name: ends });
    }
    expect(screen.queryByRole("heading", { level: 2, name: "Connect an agent" }) !== null).toBe(agent);
    expect(screen.queryByRole("button", { name: abandon }) !== null).toBe(abandons);
    expect(screen.queryByRole("button", { name: continues }) !== null).toBe(state === "rolling_back");
  }
);

test("abandoned, the upgrade rolls back: the page reads it again and shows its job, the banner follows, the focus is on the title", async () => {
  const user = userEvent.setup();
  const server = upgradeServer({ upgrade: upgradeJSON() });
  renderApp(upgradePath, server.app);

  await user.click(await screen.findByRole("button", { name: abandon }));
  const dialog = await screen.findByRole("alertdialog", { name: "Abandon the upgrade of “Plans”?" });
  expect(dialog.textContent).toContain(
    "A background job puts the notebook back as it was when the upgrade started: its pages and attachments go back to their places, names and content, and those created since go to the trash."
  );
  await user.click(within(dialog).getByRole("button", { name: "Abandon the upgrade" }));

  const rollback = await section("Rollback");
  const job = within(rollback).getByRole("listitem", { name: (name) => name.startsWith("Undo the upgrade") });
  expect(within(job).getByText("Queued")).toBeTruthy();
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(document.activeElement).toBe(title());
  expect(screen.queryByRole("heading", { level: 2, name: "Connect an agent" })).toBeNull();
  expect(screen.queryByRole("button", { name: abandon })).toBeNull();
  expect(screen.queryByRole("button", { name: continues })).toBeNull();
  expect(await screen.findByText(rollingBanner)).toBeTruthy();
  expect(server.asked.filter((asked) => asked !== "GET job upgrade")).toEqual([
    "GET upgrade",
    "ABANDON",
    "GET upgrade",
  ]);
});

/**
 * upgradeRead answers the upgrade's read as upgradeServer does: none under
 * way 404 wiki.upgrade_not_found; rolling back, what its rollback left
 * once its job is not under way; as said, an upgrade a test changes.
 */
function upgradeRead(server: ReturnType<typeof upgradeServer>, said = (upgrade: Upgrade) => upgrade): Response {
  const upgrade = server.upgrade;
  if (upgrade === undefined || server.notebook.upgrade === null) {
    return problem(404, "wiki.upgrade_not_found");
  }
  const rolling = server.jobs.some((job) => job.id === upgrade.rollback_job_id && under(job));
  return json(
    said(upgrade.state === "rolling_back" ? { ...upgrade, rollback_left: rolling ? null : server.left } : upgrade)
  );
}

/**
 * secondReadHeld is upgradeServer of options whose second read of the
 * upgrade, the one after a change, is answered once held resolves, as the
 * server holds the upgrade then.
 */
function secondReadHeld(options: Parameters<typeof upgradeServer>[0]) {
  const held = deferred<void>();
  let answered = 0;
  const server: ReturnType<typeof upgradeServer> = upgradeServer({
    ...options,
    answers: {
      [`GET ${plans}/upgrade`]: async () => {
        server.asked.push("GET upgrade");
        if (++answered === 2) {
          await held.promise;
        }
        return upgradeRead(server);
      },
    },
  });
  return { server, held };
}

/**
 * heldServer is upgradeServer of options whose reads of the notebooks,
 * logged as "GET notebooks (held)", and of the upgrade wait while the
 * test holds them (holdList, holdReads), until it lets them go (list,
 * reads); events is the stream's.
 */
function heldServer(options: Parameters<typeof upgradeServer>[0]) {
  const events = eventServer();
  const held = { holdList: false, holdReads: false, list: deferred<void>(), reads: deferred<void>() };
  const server: ReturnType<typeof upgradeServer> = upgradeServer({
    ...options,
    answers: {
      "GET /api/v0/events": events.answer,
      "GET /api/v0/workspaces/lab/notebooks": async () => {
        if (held.holdList) {
          server.asked.push("GET notebooks (held)");
          await held.list.promise;
        }
        return json({ data: [server.notebook] });
      },
      [`GET ${plans}/upgrade`]: async () => {
        server.asked.push("GET upgrade");
        if (held.holdReads) {
          await held.reads.promise;
        }
        return upgradeRead(server);
      },
    },
  });
  return { server, events, held };
}

test("abandoned, the page shows the rollback the abandonment answered while it reads the upgrade again; the focus is on the title", async () => {
  const user = userEvent.setup();
  const { server, held } = secondReadHeld({ upgrade: upgradeJSON() });
  renderApp(upgradePath, server.app);
  const button = await screen.findByRole("button", { name: abandon });

  await user.click(button);
  const dialog = await screen.findByRole("alertdialog", { name: "Abandon the upgrade of “Plans”?" });
  await user.click(within(dialog).getByRole("button", { name: "Abandon the upgrade" }));

  const rollback = await section("Rollback");
  expect(await within(rollback).findByText("Queued")).toBeTruthy();
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(document.activeElement).toBe(title());
  expect(button.isConnected).toBe(false);
  expect(screen.queryByRole("heading", { level: 2, name: "Connect an agent" })).toBeNull();
  expect(reads(server, "GET upgrade")).toBe(2);
  held.resolve();
  await act(() => held.promise);
  expect(within(await section("Rollback")).getByText("Queued")).toBeTruthy();
});

test("continued, the page shows the job the continuation answered while it reads the upgrade again; the focus is on the title", async () => {
  const user = userEvent.setup();
  const job = stopped(70);
  const { server, held } = secondReadHeld({ jobs: [job], upgrade: rollbackOf(job) });
  server.left = { nodes: 2, pages: 1, created: 3 };
  renderApp(upgradePath, server.app);
  const button = await screen.findByRole("button", { name: continues });

  await user.click(button);
  const dialog = await screen.findByRole("alertdialog", { name: "Continue the rollback of “Plans”?" });
  await user.click(within(dialog).getByRole("button", { name: "Continue the rollback" }));

  const rollback = await section("Rollback");
  expect(await within(rollback).findByText("Queued")).toBeTruthy();
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(document.activeElement).toBe(title());
  expect(button.isConnected).toBe(false);
  expect(counts(rollback)).toEqual([]);
  expect(reads(server, "GET upgrade")).toBe(2);
  held.resolve();
  await act(() => held.promise);
  expect(within(await section("Rollback")).getByText("Queued")).toBeTruthy();
});

test("ended, the page's read again still out, the focus is on the title, not on End still shown", async () => {
  const user = userEvent.setup();
  const job = stopped(70);
  const { server, held } = secondReadHeld({ jobs: [job], upgrade: rollbackOf(job) });
  server.left = { nodes: 2, pages: 1, created: 3 };
  renderApp(upgradePath, server.app);
  const button = await screen.findByRole("button", { name: ends });

  await user.click(button);
  const dialog = await screen.findByRole("alertdialog", { name: "End the rollback of “Plans”?" });
  await user.click(within(dialog).getByRole("button", { name: "End the rollback" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(button.isConnected).toBe(true);
  expect(document.activeElement).toBe(title());
  held.resolve();
  expect(await screen.findByText(noneUnderWay)).toBeTruthy();
  expect(button.isConnected).toBe(false);
});

test("abandoned, the read again shows what the server says then, not the abandonment's answer", async () => {
  const user = userEvent.setup();
  const server: ReturnType<typeof upgradeServer> = upgradeServer({
    upgrade: upgradeJSON(),
    answers: {
      [`GET ${plans}/upgrade`]: () => {
        server.asked.push("GET upgrade");
        // Read again after the abandonment, the upgrade names its initiator otherwise.
        return upgradeRead(server, (upgrade) =>
          reads(server, "GET upgrade") === 1
            ? upgrade
            : { ...upgrade, started_by: { ...upgrade.started_by, display_name: "Zed" } }
        );
      },
    },
  });
  renderApp(upgradePath, server.app);

  await user.click(await screen.findByRole("button", { name: abandon }));
  const dialog = await screen.findByRole("alertdialog", { name: "Abandon the upgrade of “Plans”?" });
  await user.click(within(dialog).getByRole("button", { name: "Abandon the upgrade" }));

  expect(await screen.findByText("Started by Zed on Oct 5, 2026, 8:00 AM.")).toBeTruthy();
  expect(reads(server, "GET upgrade")).toBe(2);
});

/**
 * abandonedAfterEvent abandons Plans' upgrade on its page, live, its
 * answer waiting for the notebooks' read again; meanwhile the server moves
 * on (move), and the mode event the abandonment published is read first,
 * which the page shows (answered). Then the abandonment answers, the
 * upgrade's reads held: the page shows what the cache holds.
 */
async function abandonedAfterEvent(
  move: (server: ReturnType<typeof upgradeServer>) => void,
  answered: () => Promise<unknown>
) {
  const user = userEvent.setup();
  const { server, events, held } = heldServer({ upgrade: upgradeJSON() });
  renderApp(upgradePath, withEvents(server.app), { eventHandlers });
  await section("Where it is");
  await waitFor(() => expect(events.streams).toHaveLength(1));
  await user.click(await screen.findByRole("button", { name: abandon }));
  const dialog = await screen.findByRole("alertdialog", { name: "Abandon the upgrade of “Plans”?" });
  held.holdList = true;
  await user.click(within(dialog).getByRole("button", { name: "Abandon the upgrade" }));
  await waitFor(() => expect(server.asked).toContain("GET notebooks (held)"));
  move(server);
  events.last().send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id });
  await answered();
  held.holdReads = true;
  const before = reads(server, "GET upgrade");
  held.list.resolve();
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  await waitFor(() => expect(reads(server, "GET upgrade")).toBeGreaterThan(before));
  return held;
}

test("abandoned while a mode event's read answers first that the rollback stopped: what is left and Continue stay, the abandonment's answer, older, not put over them", async () => {
  const held = await abandonedAfterEvent(
    (server) => {
      server.left = { nodes: 2, pages: 1, created: 3 };
      server.rollbackJob((job) => cancelled(job));
    },
    // The dialog, still open, hides the rest from the accessibility tree.
    () => screen.findByRole("button", { name: continues, hidden: true })
  );

  const rollback = await section("Rollback");
  expect(within(rollback).getByRole("button", { name: continues })).toBeTruthy();
  expect(counts(rollback)).toEqual(leftCounts);
  held.reads.resolve();
  await act(() => held.reads.promise);
  expect(within(await section("Rollback")).getByRole("button", { name: continues })).toBeTruthy();
});

test("abandoned while a mode event's read answers first that the rollback ended: none under way stays, the abandonment's answer not put over it", async () => {
  const held = await abandonedAfterEvent(
    (server) => server.rolledBack(),
    () => screen.findByText(noneUnderWay)
  );

  expect(screen.getByText(noneUnderWay)).toBeTruthy();
  expect(screen.queryByRole("heading", { level: 2, name: "Rollback" })).toBeNull();
  held.reads.resolve();
  await act(() => held.reads.promise);
  expect(screen.getByText(noneUnderWay)).toBeTruthy();
});

test("abandoned, the page left before the answer: come back to, the reads held, it shows the rollback answered, not the upgrade as it was", async () => {
  const user = userEvent.setup();
  const { server, held } = heldServer({ upgrade: upgradeJSON() });
  const { router } = renderApp(upgradePath, server.app);
  await user.click(await screen.findByRole("button", { name: abandon }));
  const dialog = await screen.findByRole("alertdialog", { name: "Abandon the upgrade of “Plans”?" });
  held.holdList = true;
  await user.click(within(dialog).getByRole("button", { name: "Abandon the upgrade" }));
  await waitFor(() => expect(server.asked).toContain("GET notebooks (held)"));
  const home = `/lab/notebooks/${notebookJSON.id}`;
  await act(() => router.navigate(home));
  await screen.findByRole("heading", { level: 1, name: "Plans" });
  // Held, a read the page makes as it comes back (past SWR's two seconds of deduplication) shows nothing yet.
  held.holdReads = true;
  held.list.resolve();
  expect(await screen.findByText(rollingBanner)).toBeTruthy();

  await act(() => router.navigate(upgradePath));

  const rollback = await section("Rollback");
  expect(await within(rollback).findByText("Queued")).toBeTruthy();
  expect(screen.queryByRole("button", { name: abandon })).toBeNull();
  held.reads.resolve();
  await act(() => held.reads.promise);
  expect(within(await section("Rollback")).getByText("Queued")).toBeTruthy();
});

test("a rollback's job under way is read every second; seen stopped, the upgrade is read again once, the page staying: what is left, Continue and End; then nothing is read", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const job = underWay(rollbackJobJSON(70), 1, 4);
  const server = upgradeServer({ jobs: [job], upgrade: rollbackOf(job) });
  server.left = { nodes: 2, pages: 1, created: 3 };
  renderApp(upgradePath, server.app);
  const rollback = await section("Rollback");
  await within(rollback).findByRole("progressbar", { name: "Progress: 1 of 4" });
  expect(screen.queryByRole("button", { name: ends })).toBeNull();
  const upgradeReads = reads(server, "GET upgrade");
  const first = reads(server, "GET job upgrade");

  // Each second is a read: not one in two, as SWR's deduplication would make it.
  server.rollbackJob((held) => underWay(held, 3, 4));
  await act(() => vi.advanceTimersByTimeAsync(1_000));
  await act(() => vi.advanceTimersByTimeAsync(1_000));
  expect(reads(server, "GET job upgrade")).toBeGreaterThanOrEqual(first + 2);
  expect(within(rollback).getByRole("progressbar", { name: "Progress: 3 of 4" })).toBeTruthy();
  expect(reads(server, "GET upgrade")).toBe(upgradeReads);

  // Cancelled through the API, the job stops, which no event tells.
  server.rollbackJob(() => stopped(70));
  await act(() => vi.advanceTimersByTimeAsync(1_000));

  await within(rollback).findByRole("button", { name: ends });
  expect(within(rollback).getByRole("button", { name: continues })).toBeTruthy();
  expect(counts(rollback)).toEqual(leftCounts);
  expect(within(rollback).getByText("Cancelled")).toBeTruthy();
  expect(screen.getByRole("heading", { level: 2, name: "Rollback" }).parentElement).toBe(rollback);
  expect(reads(server, "GET upgrade")).toBe(upgradeReads + 1);
  const ended = reads(server, "GET job upgrade");
  await act(() => vi.advanceTimersByTimeAsync(5_000));
  expect(reads(server, "GET job upgrade")).toBe(ended);
  expect(reads(server, "GET upgrade")).toBe(upgradeReads + 1);
  expect(screen.queryByRole("button", { name: abandon })).toBeNull();
});

test("a rollback's job seen done, everything put back, reads the upgrade again: it ended, and none is under way", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const job = underWay(rollbackJobJSON(70), 1, 4);
  const server = upgradeServer({ jobs: [job], upgrade: rollbackOf(job) });
  renderApp(upgradePath, server.app);
  await screen.findByRole("progressbar", { name: "Progress: 1 of 4" });

  server.rolledBack();
  await act(() => vi.advanceTimersByTimeAsync(1_000));

  expect(await screen.findByText(noneUnderWay)).toBeTruthy();
  expect(reads(server, "GET upgrade")).toBe(2);
});

test("a rollback's job seen stopped, the upgrade's read again failing: it is read once, and the page offers to try again", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const job = underWay(rollbackJobJSON(70), 1, 4);
  let answered = 0;
  const server: ReturnType<typeof upgradeServer> = upgradeServer({
    jobs: [job],
    upgrade: rollbackOf(job),
    answers: {
      [`GET ${plans}/upgrade`]: () => {
        server.asked.push("GET upgrade");
        return ++answered === 1 ? json(rollbackOf(job)) : problem(500, "internal_error");
      },
    },
  });
  renderApp(upgradePath, server.app);
  await screen.findByRole("progressbar", { name: "Progress: 1 of 4" });

  server.rollbackJob(() => stopped(70));
  await act(() => vi.advanceTimersByTimeAsync(1_000));

  expect(await screen.findByRole("button", { name: "Try again" })).toBeTruthy();
  // Short of SWR's first retry, five seconds after the failure.
  await act(() => vi.advanceTimersByTimeAsync(3_000));
  expect(reads(server, "GET upgrade")).toBe(2);
});

test("a rollback's job seen stopped reads the upgrade again once, however often the page renders before the read answers", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const job = underWay(rollbackJobJSON(70), 1, 4);
  const { server, held } = secondReadHeld({ jobs: [job], upgrade: rollbackOf(job) });
  server.left = { nodes: 2, pages: 1, created: 3 };
  renderApp(upgradePath, server.app);
  await screen.findByRole("progressbar", { name: "Progress: 1 of 4" });

  server.rollbackJob(() => stopped(70));
  await act(() => vi.advanceTimersByTimeAsync(1_000));
  await waitFor(() => expect(reads(server, "GET upgrade")).toBe(2));
  // The page renders again, its read still out: its language changes.
  act(() => server.app.preferences.setLocale("zh-CN"));
  await screen.findByRole("heading", { level: 1, name: "升级为 LLM Wiki" });
  await act(() => vi.advanceTimersByTimeAsync(1_000));
  expect(reads(server, "GET upgrade")).toBe(2);

  held.resolve();
  expect(await within(await section("回滚")).findByRole("button", { name: "结束回滚，保留现在的样子…" })).toBeTruthy();
  expect(reads(server, "GET upgrade")).toBe(2);
});

test("a rollback stopped as the page opens: its job is read once and the upgrade once", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const job = stopped(70);
  const server = upgradeServer({ jobs: [job], upgrade: rollbackOf(job) });
  server.left = { nodes: 2, pages: 1, created: 3 };
  renderApp(upgradePath, server.app);

  const rollback = await section("Rollback");
  await within(rollback).findByText("Cancelled");
  expect(counts(rollback)).toEqual(leftCounts);
  await act(() => vi.advanceTimersByTimeAsync(5_000));
  expect([reads(server, "GET upgrade"), reads(server, "GET job upgrade")]).toEqual([1, 1]);
});

test("ending a stopped rollback: its dialog says what is left and that links may lead nowhere; done, none is under way, the banner gone, the focus on the title", async () => {
  const user = userEvent.setup();
  const job = stopped(70);
  const server = upgradeServer({ jobs: [job], upgrade: rollbackOf(job) });
  server.left = { nodes: 2, pages: 1, created: 3 };
  renderApp(upgradePath, server.app);
  expect(await screen.findByText(rollingBanner)).toBeTruthy();

  await user.click(await screen.findByRole("button", { name: ends }));
  const dialog = await screen.findByRole("alertdialog", { name: "End the rollback of “Plans”?" });
  expect(counts(dialog)).toEqual(leftCounts);
  expect(within(dialog).getByText(linksMayFail)).toBeTruthy();
  expect(within(dialog).queryByText(nothingLeft)).toBeNull();
  await user.click(within(dialog).getByRole("button", { name: "End the rollback" }));

  expect(await screen.findByText(noneUnderWay)).toBeTruthy();
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(document.activeElement).toBe(title());
  await waitFor(() => expect(screen.queryByText(rollingBanner)).toBeNull());
  expect(server.asked.filter((asked) => asked !== "GET job upgrade")).toEqual([
    "GET upgrade",
    "END ROLLBACK",
    "GET upgrade",
  ]);
});

test("a stopped rollback that left nothing: its end's dialog says ending is all that is left, no counts", async () => {
  const user = userEvent.setup();
  const job = stopped(70);
  const server = upgradeServer({ jobs: [job], upgrade: rollbackOf(job) });
  renderApp(upgradePath, server.app);

  await user.click(await screen.findByRole("button", { name: ends }));
  const dialog = await screen.findByRole("alertdialog", { name: "End the rollback of “Plans”?" });

  expect(within(dialog).getByText(nothingLeft)).toBeTruthy();
  expect(counts(dialog)).toEqual([]);
  expect(within(dialog).queryByText(linksMayFail)).toBeNull();
});

test.each([
  ["a node not back in place", { nodes: 1, pages: 0, created: 0 }],
  ["a page not written back", { nodes: 0, pages: 1, created: 0 }],
  ["the raw and AGENTS its start wrote", { nodes: 0, pages: 0, created: 2 }],
])(
  "a stopped rollback that left only %s: its end's dialog lists the three counts and that links may lead nowhere",
  async (_, left) => {
    const user = userEvent.setup();
    const job = stopped(70);
    const server = upgradeServer({ jobs: [job], upgrade: rollbackOf(job) });
    server.left = left;
    renderApp(upgradePath, server.app);

    await user.click(await screen.findByRole("button", { name: ends }));
    const dialog = await screen.findByRole("alertdialog", { name: "End the rollback of “Plans”?" });

    expect(counts(dialog)).toEqual([
      ["Nodes not back in place", `${left.nodes}`],
      ["Pages not written back", `${left.pages}`],
      ["New nodes still there", `${left.created}`],
    ]);
    expect(within(dialog).getByText(linksMayFail)).toBeTruthy();
    expect(within(dialog).queryByText(nothingLeft)).toBeNull();
  }
);

test("continuing a stopped rollback starts its job anew: Continue and End go, its new job shows, the focus is on the title", async () => {
  const user = userEvent.setup();
  const job = stopped(70);
  const server = upgradeServer({ jobs: [job], upgrade: rollbackOf(job) });
  server.left = { nodes: 2, pages: 1, created: 3 };
  renderApp(upgradePath, server.app);

  await user.click(await screen.findByRole("button", { name: continues }));
  const dialog = await screen.findByRole("alertdialog", { name: "Continue the rollback of “Plans”?" });
  await user.click(within(dialog).getByRole("button", { name: "Continue the rollback" }));

  const rollback = await section("Rollback");
  await within(rollback).findByText("Queued");
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(document.activeElement).toBe(title());
  expect(within(rollback).queryByRole("button", { name: continues })).toBeNull();
  expect(within(rollback).queryByRole("button", { name: ends })).toBeNull();
  expect(counts(rollback)).toEqual([]);
  expect(server.asked).toContain("ABANDON");
});

test.each([
  [abandon, "Abandon the upgrade of “Plans”?"],
  [continues, "Continue the rollback of “Plans”?"],
  [ends, "End the rollback of “Plans”?"],
])("%s, its dialog cancelled, has the focus back", async (button, name) => {
  const user = userEvent.setup();
  const job = stopped(70);
  const server = upgradeServer(
    button === abandon ? { upgrade: upgradeJSON() } : { jobs: [job], upgrade: rollbackOf(job) }
  );
  renderApp(upgradePath, server.app);

  const trigger = await screen.findByRole("button", { name: button });
  await user.click(trigger);
  const dialog = await screen.findByRole("alertdialog", { name });
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(document.activeElement).toBe(trigger);
  expect(server.asked.filter((asked) => asked !== "GET job upgrade")).toEqual(["GET upgrade"]);
});

test("ending a rollback another admin continued meanwhile is refused as under way: the dialog says so, the upgrade read again shows the new job; closed, the focus is on the title", async () => {
  const user = userEvent.setup();
  const job = stopped(70);
  const server = upgradeServer({ jobs: [job], upgrade: rollbackOf(job) });
  renderApp(upgradePath, server.app);
  await user.click(await screen.findByRole("button", { name: ends }));
  const dialog = await screen.findByRole("alertdialog", { name: "End the rollback of “Plans”?" });
  const again = underWay(rollbackJobJSON(71, { created_by: { user_id: "bob", display_name: "Bob" } }), 0, 0);
  server.jobs = [again, job];
  server.withUpgrade(rollbackOf(again));

  await user.click(within(dialog).getByRole("button", { name: "End the rollback" }));

  expect(await within(dialog).findByText("The rollback is under way already.")).toBeTruthy();
  await waitFor(() => expect(reads(server, "GET upgrade")).toBe(2));
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(document.activeElement).toBe(title());
  const rollback = await section("Rollback");
  expect(within(rollback).getByText("Queued")).toBeTruthy();
  expect(within(rollback).queryByRole("button", { name: ends })).toBeNull();
});

test("abandoning an upgrade another admin ended meanwhile is refused for its state: the dialog stays and says so, the page read again says none is under way, the banner gone", async () => {
  const user = userEvent.setup();
  const server = upgradeServer({ upgrade: bobsUpgrade() });
  renderApp(upgradePath, server.app);
  await user.click(await screen.findByRole("button", { name: abandon }));
  const dialog = await screen.findByRole("alertdialog", { name: "Abandon the upgrade of “Plans”?" });
  server.withUpgrade(bobsUpgrade({ state: "rolled_back", ending: "abandoned", rollback_complete: true }));

  await user.click(within(dialog).getByRole("button", { name: "Abandon the upgrade" }));

  expect(await within(dialog).findByText("The upgrade's current state does not allow this.")).toBeTruthy();
  await waitFor(() => expect(screen.queryByText("Any admin of the notebook can abandon this upgrade.")).toBeNull());
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(document.activeElement).toBe(title());
  expect(screen.getByText(noneUnderWay)).toBeTruthy();
});

test("ending a rollback as an admin no longer one is refused: the dialog stays and says so, the page says who can see it and reads no more; closed, the focus is on the title", async () => {
  const user = userEvent.setup();
  const job = stopped(70);
  const server = upgradeServer({ jobs: [job], upgrade: rollbackOf(job) });
  renderApp(upgradePath, server.app);
  await user.click(await screen.findByRole("button", { name: ends }));
  const dialog = await screen.findByRole("alertdialog", { name: "End the rollback of “Plans”?" });
  // Another admin made Ada an editor meanwhile.
  server.notebook = { ...server.notebook, role: "editor" };

  await user.click(within(dialog).getByRole("button", { name: "End the rollback" }));

  expect(await within(dialog).findByText("You do not have permission to do this.")).toBeTruthy();
  await waitFor(() => expect(screen.getByText(adminsOnly, { exact: true })).toBeTruthy());
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(document.activeElement).toBe(title());
  expect(server.asked.filter((asked) => asked !== "GET job upgrade")).toEqual(["GET upgrade", "END ROLLBACK"]);
});

// The mode event and each connection read the upgrade again (M11/P1
// design 4.4, v0.1 design 13.2, item 16).

/**
 * live renders the upgrade's page over Plans with upgrade and its event
 * stream, connected; the notebooks' reads are logged among what the
 * server was asked, as "GET notebooks".
 */
async function live(upgrade: Upgrade) {
  const events = eventServer();
  const server: ReturnType<typeof upgradeServer> = upgradeServer({
    upgrade,
    answers: {
      "GET /api/v0/events": events.answer,
      "GET /api/v0/workspaces/lab/notebooks": () => {
        server.asked.push("GET notebooks");
        return json({ data: [server.notebook] });
      },
    },
  });
  renderApp(upgradePath, withEvents(server.app), { eventHandlers });
  await section("Where it is");
  await waitFor(() => expect(events.streams).toHaveLength(1));
  await waitFor(() => expect(server.sent).toContain("GET nodes"));
  server.asked.length = 0;
  server.sent.length = 0;
  return { server, events };
}

test("a mode event reads the upgrade again: abandoned by another admin, the page shows its rollback, without Abandon", async () => {
  const { server, events } = await live(bobsUpgrade());
  const job = underWay(rollbackJobJSON(70), 0, 0);
  server.jobs = [job];
  server.withUpgrade(bobsUpgrade({ state: "rolling_back", ending: "abandoned", rollback_job_id: job.id }));

  events.last().send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id });

  const rollback = await section("Rollback");
  expect(await within(rollback).findByText("Queued")).toBeTruthy();
  expect(screen.queryByRole("button", { name: abandon })).toBeNull();
  expect(server.asked.filter((asked) => asked !== "GET job upgrade").toSorted()).toEqual([
    "GET notebooks",
    "GET upgrade",
  ]);
});

test("each connection reads the upgrade again, once the notebooks are read and shown", async () => {
  const { server, events } = await live(bobsUpgrade());
  server.withUpgrade(bobsUpgrade({ state: "reviewing" }));

  events.last().hello();

  await waitFor(() => expect(server.asked).toEqual(["GET notebooks", "GET upgrade"]));
  expect(
    await screen.findByText(
      "This notebook's upgrade is submitted and waits for an admin's review: it is read-only for now."
    )
  ).toBeTruthy();
  expect(screen.queryByRole("heading", { level: 2, name: "Connect an agent" })).toBeNull();
});

test("a connection that finds the account no longer the notebook's admin reads no upgrade: the page says who can see it", async () => {
  const { server, events } = await live(bobsUpgrade());
  server.notebook = { ...server.notebook, role: "editor" };

  events.last().hello();

  expect(await screen.findByText(adminsOnly)).toBeTruthy();
  // The tree, read again at the upgrade's level, is the last of the connection's reads.
  await waitFor(() => expect(server.sent).toContain("GET nodes"));
  expect(server.asked).toEqual(["GET notebooks"]);
});

// The check and the submission (M11/P2 design 4.3, decisions 259, 264):
// the check and Submit for review while upgrading, the report kept in
// review; the submission's first step shows the upgrade reviewing, its
// dialog staying until the answer.

const submits = "Submit for review…";
const notReady = "The check did not pass, and the upgrade is upgrading again. The check below is read again.";
const pending = "Being checked, or the last submission did not finish.";
const passedAt = "Every check passed: the upgrade can be submitted for review. Checked on Oct 6, 2026, 9:30 AM.";
const reviewingBanner =
  "This notebook's upgrade is submitted and waits for an admin's review: it is read-only for now.";
/** What the check's result says, the rest of the page hidden by a dialog or not. */
const resultOf = (check: HTMLElement) =>
  within(check).getByRole("status", { name: "The check's result", hidden: true }).textContent;
/** The check's section, hidden by a dialog or not. */
const checkOf = async () =>
  (await screen.findByRole("heading", { level: 2, name: "Check", hidden: true })).closest("section") as HTMLElement;
const checks = (server: { asked: string[] }) => reads(server, "CHECK");

const approves = "Approve…";
const sendsBack = "Send back for more work…";
const rejects = "Reject…";

/** The order of the upgrade page's sections and of its buttons below the state, by their names. */
function sectionsInOrder(): string[] {
  const headings = ["Where it is", "Connect an agent", "Check", "Check report", "Rollback"];
  const buttons = [submits, "Abandon this upgrade…", approves, sendsBack, rejects];
  return [...document.querySelectorAll("h2, button")].flatMap((each) => {
    const name = each.textContent ?? "";
    return (each.tagName === "H2" ? headings : buttons).includes(name) ? [name] : [];
  });
}

// In review, Approve once the report is kept (decision 288), Send back and Reject, not Abandon (decision 291).
test.each([
  ["upgrading", true, false, ["Where it is", "Connect an agent", "Check", submits, "Abandon this upgrade…"]],
  ["reviewing", false, true, ["Where it is", "Check report", approves, sendsBack, rejects]],
  ["reviewing without a report", false, true, ["Where it is", "Check report", sendsBack, rejects]],
  ["rolling_back", false, false, ["Where it is", "Rollback"]],
] as const)(
  "an upgrade %s: the check and Submit for review %s, the report %s, the sections and buttons in order",
  async (state, checking, reported, order) => {
    const job = stopped(70);
    const server = upgradeServer({
      jobs: state === "rolling_back" ? [job] : [],
      upgrade:
        state === "rolling_back"
          ? rollbackOf(job)
          : state === "reviewing without a report"
            ? upgradeJSON({ state: "reviewing", submitted_by: bob, submitted_at: "2026-10-06T09:40:00Z" })
            : upgradeJSON({ state, report: reportJSON() }),
    });
    renderApp(upgradePath, server.app);

    await section("Where it is");
    if (state === "rolling_back") {
      await screen.findByRole("button", { name: ends });
    }
    expect(screen.queryByRole("heading", { level: 2, name: "Check" }) !== null).toBe(checking);
    expect(screen.queryByRole("button", { name: "Check" }) !== null).toBe(checking);
    expect(screen.queryByRole("button", { name: submits }) !== null).toBe(checking);
    expect(screen.queryByRole("heading", { level: 2, name: "Check report" }) !== null).toBe(reported);
    expect(sectionsInOrder()).toEqual(order);
    expect(server.asked.filter((asked) => asked !== "GET job upgrade")).toEqual(["GET upgrade"]);
  }
);

/**
 * A report every value of which is its own: the gates passed, the hints'
 * counts, capped or not, their first items, as many as the server keeps of a
 * rule (at most five: all of a count of five or fewer).
 */
const keptReport = reportJSON({
  computed_at: "2026-10-06T09:41:00Z",
  gates: {
    agents: { passed: true },
    source_root: { passed: true },
    reserved_titles: { passed: true, node_ids: [] },
    frontmatter: { passed: true },
    sources: { passed: true },
    links: { passed: true, pages_worse: 0, node_ids: [] },
    preserved: { passed: true, nodes: 8, nodes_kept: 8, pages: 5, pages_kept: 5, node_ids: [] },
    index: { passed: true },
  },
  counts: {
    sources: 31,
    moved_into_sources: 32,
    wiki_pages: 33,
    new_pages: 34,
    changesets: 35,
    wanted_before: { count: 36, count_capped: false },
    wanted_now: { count: 37, count_capped: true },
  },
  // In another order than shown.
  rules: (
    [
      {
        rule: "wanted",
        count: 41,
        count_capped: true,
        items: [
          { rule: "wanted", item_key: `w:${"a".repeat(64)}`, target: "Missing", target_cut: false, pages: 2 },
          { rule: "wanted", item_key: `w:${"c".repeat(64)}`, target: "Other", target_cut: false, pages: 3 },
          { rule: "wanted", item_key: `w:${"e".repeat(64)}`, target: "Roadmap", target_cut: false, pages: 4 },
          { rule: "wanted", item_key: `w:${"f".repeat(64)}`, target: "Glossary", target_cut: false, pages: 6 },
          { rule: "wanted", item_key: `w:${"9".repeat(64)}`, target: "Changelog", target_cut: false, pages: 7 },
        ],
      },
      {
        rule: "orphans",
        count: 1,
        count_capped: false,
        items: [{ rule: "orphans", item_key: `p:${notes.id}`, page_id: notes.id }],
      },
      {
        rule: "ambiguous",
        count: 2,
        count_capped: true,
        items: [
          {
            rule: "ambiguous",
            item_key: `a:${guide.id}:${"b".repeat(64)}`,
            page_id: guide.id,
            target: "Plan",
            target_cut: false,
          },
          {
            rule: "ambiguous",
            item_key: `a:${install.id}:${"e".repeat(64)}`,
            page_id: install.id,
            target: "Setup",
            target_cut: false,
          },
        ],
      },
      { rule: "invalid_frontmatter", count: 0, count_capped: false, items: [] },
      {
        rule: "missing_type",
        count: 3,
        count_capped: false,
        items: [notes, guide, install].map((page) => ({
          rule: "missing_type" as const,
          item_key: `p:${page.id}`,
          page_id: page.id,
        })),
      },
      {
        rule: "missing_summary",
        count: 4,
        count_capped: false,
        items: [guide, seededRaw, install, notes].map((page) => ({
          rule: "missing_summary" as const,
          item_key: `p:${page.id}`,
          page_id: page.id,
        })),
      },
      {
        rule: "undigested_sources",
        count: 6,
        count_capped: false,
        items: [
          {
            rule: "undigested_sources",
            item_key: `n:${install.id}`,
            node_id: install.id,
            kind: "page",
            bytes: 2048,
            created_at: "2026-10-03T08:00:00Z",
          },
          {
            rule: "undigested_sources",
            item_key: `n:${notes.id}`,
            node_id: notes.id,
            kind: "asset",
            bytes: 10240,
            created_at: "2026-10-04T09:00:00Z",
          },
          {
            rule: "undigested_sources",
            item_key: `n:${guide.id}`,
            node_id: guide.id,
            kind: "page",
            bytes: 512,
            created_at: "2026-10-05T10:00:00Z",
          },
          {
            rule: "undigested_sources",
            item_key: `n:${seededRaw.id}`,
            node_id: seededRaw.id,
            kind: "page",
            bytes: 3072,
            created_at: "2026-10-05T11:00:00Z",
          },
          {
            rule: "undigested_sources",
            item_key: `n:${seededAgents.id}`,
            node_id: seededAgents.id,
            kind: "asset",
            bytes: 20480,
            created_at: "2026-10-05T12:00:00Z",
          },
        ],
      },
      {
        rule: "invalid_sources",
        count: 5,
        count_capped: false,
        items: (
          [
            [install, "Broken", "d"],
            [guide, "Gone", "1"],
            [notes, "Lost", "2"],
            [seededAgents, "Moved", "3"],
            [install, "Renamed", "4"],
          ] as const
        ).map(([page, target, hex]) => ({
          rule: "invalid_sources" as const,
          item_key: `s:${page.id}:${hex.repeat(64)}`,
          page_id: page.id,
          target,
          target_cut: false,
          not_link: false,
        })),
      },
    ] satisfies UpgradeReport["rules"]
  ).toReversed(),
});

/** Each card under holder: its heading's level and text, its count (after its description), its items, whether it offers More. */
const cardsIn = (holder: Element) =>
  [...holder.querySelectorAll<HTMLElement>("section[aria-labelledby]")].map((rule) => [
    rule.querySelector("h2, h3, h4")?.tagName,
    rule.querySelector("h2, h3, h4")?.textContent,
    rule.querySelectorAll("p")[1]?.textContent,
    [...rule.querySelectorAll("li")].map((item) => item.textContent),
    within(rule).queryByRole("button", { name: "More" }) !== null,
  ]);

test("in review, the page shows the report kept: each gate, each count, each rule a card of its count and first items, More where the report keeps fewer than there are, the rules without an issue in a disclosure", async () => {
  const server = upgradeServer({
    nodes: [guide, install, notes, seededRaw, seededAgents],
    upgrade: upgradeJSON({ state: "reviewing", report: keptReport }),
  });
  renderApp(upgradePath, server.app);

  const report = await section("Check report");
  await within(report).findAllByText("Guide/Install.md");
  expect(report.querySelector("p")?.textContent).toBe(
    "The check at its submission (Oct 6, 2026, 9:41 AM): every check passed."
  );
  const gates = within(report).getByRole("list", { name: "The checks" });
  expect([...gates.children].map((gate) => [...gate.children].map((part) => part.textContent))).toEqual([
    ["The AGENTS page at the root has a body: passed"],
    ["The source root is a page, not the root's AGENTS: passed"],
    ["No node at the root is named index, log, index.md or log.md: passed"],
    [
      "Each page outside the source area, the source root and the root's AGENTS included, has frontmatter that parses; each except these two has a type and a summary: passed",
    ],
    ["Each sources reference leads to a node of the source area: passed"],
    ["No page the upgrade started with leads to more pages that do not exist: passed"],
    [
      "Each node the upgrade started with is there, and no page's body changed: passed",
      "Nodes kept: 8 of 8; pages kept: 5 of 5.",
    ],
    ["The link index is fresh: passed"],
  ]);
  expect(counts(report)).toEqual([
    ["Nodes in the source area", "31"],
    ["Nodes it started with, moved into the source area", "32"],
    ["Pages outside the source area", "33"],
    ["Pages created since the upgrade started", "34"],
    ["Changes of the upgrade", "35"],
    ["Wanted pages as the upgrade started", "36"],
    ["Wanted pages now", "37+"],
  ]);
  const clear = report.querySelector("details") as HTMLElement;
  expect(clear.querySelector("summary")?.textContent).toBe("No issues (1 checks)");
  expect(cardsIn(clear)).toEqual([["H3", "Invalid frontmatter", "0 issues", [], false]]);
  expect(cardsIn(report)).toEqual([
    [
      "H3",
      "Wanted pages",
      "41+ issues",
      [
        "MissingReferenced by 2 pages",
        "OtherReferenced by 3 pages",
        "RoadmapReferenced by 4 pages",
        "GlossaryReferenced by 6 pages",
        "ChangelogReferenced by 7 pages",
      ],
      true,
    ],
    ["H3", "Orphan pages", "1 issue", ["Notes.md"], false],
    ["H3", "Ambiguous links", "2+ issues", ["Guide.mdPlan", "Guide/Install.mdSetup"], true],
    ["H3", "Missing type", "3 issues", ["Notes.md", "Guide.md", "Guide/Install.md"], false],
    ["H3", "Missing summary", "4 issues", ["Guide.md", "raw.md", "Guide/Install.md", "Notes.md"], false],
    [
      "H3",
      "Undigested sources",
      "6 issues",
      [
        "Guide/Install.mdPage2 KBCreated Oct 3, 2026, 8:00 AM",
        "Notes.mdAttachment10 KBCreated Oct 4, 2026, 9:00 AM",
        "Guide.mdPage512 BCreated Oct 5, 2026, 10:00 AM",
        "raw.mdPage3 KBCreated Oct 5, 2026, 11:00 AM",
        "AGENTS.mdAttachment20 KBCreated Oct 5, 2026, 12:00 PM",
      ],
      true,
    ],
    [
      "H3",
      "Invalid sources",
      "5 issues",
      ["Guide/Install.mdBroken", "Guide.mdGone", "Notes.mdLost", "AGENTS.mdMoved", "Guide/Install.mdRenamed"],
      false,
    ],
    ["H3", "Invalid frontmatter", "0 issues", [], false],
  ]);
  // Shown, the report reads nothing more: its rules' items come from it.
  expect(server.asked.filter((asked) => asked !== "GET job upgrade")).toEqual(["GET upgrade"]);
});

test("in review without a report, the page says it is being checked, or that the last submission did not finish", async () => {
  const server = upgradeServer({ upgrade: upgradeJSON({ state: "reviewing", submitted_by: bob }) });
  renderApp(upgradePath, server.app);

  const report = await section("Check report");
  expect(report.textContent).toBe(`Check report${pending}`);
});

/**
 * submitting renders the upgrade's page of Plans, upgrading, live, its
 * stream connected, whose submission ends as outcome, its second step
 * waiting for checked, its second mode event and its answer for the
 * promises given; it opens the submission's dialog.
 */
async function submitting(
  outcome: (typeof upgradeServer extends (...args: never[]) => infer S ? S : never)["submission"]["outcome"],
  waits: { mode?: Promise<void>; answered?: Promise<void> } = {}
) {
  const user = userEvent.setup();
  const events = eventServer();
  const server = upgradeServer({ upgrade: upgradeJSON(), events });
  const checked = deferred<void>();
  server.submission = { ...server.submission, outcome, checked: checked.promise, ...waits };
  renderApp(upgradePath, withEvents(server.app), { eventHandlers });
  await section("Where it is");
  await waitFor(() => expect(events.streams).toHaveLength(1));
  await user.click(await screen.findByRole("button", { name: submits }));
  const dialog = await screen.findByRole("alertdialog", { name: "Submit for review?" });
  // Submitting is no destructive change (decision 259): its confirmation is not styled as one.
  expect(within(dialog).getByRole("button", { name: "Submit for review" }).className).not.toMatch(/bg-destructive/);
  expect(dialog.textContent).toContain(
    "Once submitted, this notebook is frozen for everyone, the initiator's agent too. The system checks it again from that moment: once every check passes, it is in review, waiting for an admin's approval; otherwise it is upgrading again."
  );
  return { user, server, checked, dialog };
}

/** firstStep confirms the submission: its first step's mode event shows the upgrade reviewing, its check gone, the dialog waiting. */
async function firstStep(user: ReturnType<typeof userEvent.setup>, dialog: HTMLElement) {
  await user.click(within(dialog).getByRole("button", { name: "Submit for review" }));
  expect(await screen.findByText(pending)).toBeTruthy();
  expect(screen.queryByRole("heading", { level: 2, name: "Check", hidden: true })).toBeNull();
  expect(screen.queryByRole("button", { name: submits, hidden: true })).toBeNull();
  expect(within(dialog).getByRole("button", { name: "Submitting…" })).toBeTruthy();
}

test.each([
  ["wiki.upgrade_not_ready", "not_ready", notReady],
  [
    "wiki.upgrade_stale_index",
    "stale_index",
    "The link index is out of date: ask the server's administrator to run nervewiki reindex, then try again.",
  ],
] as const)(
  "submitted, refused as %s before its second step's mode event: the refusal reads the upgrade again, upgrading, its check back, read once; closed, the focus is on Submit for review",
  async (_, outcome, said) => {
    const mode = deferred<void>();
    const { user, server, checked, dialog } = await submitting(outcome, { mode: mode.promise });
    await firstStep(user, dialog);
    expect(checks(server)).toBe(0);

    checked.resolve();

    expect(await within(dialog).findByText(said)).toBeTruthy();
    const check = await checkOf();
    await waitFor(() => expect(resultOf(check)).toBe(passedAt));
    expect(checks(server)).toBe(1);
    expect(screen.queryByText(pending)).toBeNull();
    mode.resolve();
    await waitFor(() => expect(reads(server, "GET upgrade")).toBe(4));
    expect(checks(server)).toBe(1);
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    expect(document.activeElement).toBe(screen.getByRole("button", { name: submits }));
  }
);

test("submitted and not ready, its second step's mode event read before the refusal: the check is back, and read once the refusal comes", async () => {
  const answered = deferred<void>();
  const { user, server, checked, dialog } = await submitting("not_ready", { answered: answered.promise });
  await firstStep(user, dialog);

  checked.resolve();

  const check = await checkOf();
  expect(resultOf(check)).toBe("");
  expect(checks(server)).toBe(0);
  answered.resolve();
  expect(await within(dialog).findByText(notReady)).toBeTruthy();
  await waitFor(() => expect(resultOf(check)).toBe(passedAt));
  expect(checks(server)).toBe(1);
});

test("submitted and passed, the reads of the upgrade after the first held: the answer shows the report at once, the dialog closed, the focus on the title; the banner follows", async () => {
  const user = userEvent.setup();
  const events = eventServer();
  const held = deferred<void>();
  const server: ReturnType<typeof upgradeServer> = upgradeServer({
    upgrade: upgradeJSON(),
    events,
    answers: {
      [`GET ${plans}/upgrade`]: async () => {
        server.asked.push("GET upgrade");
        if (reads(server, "GET upgrade") > 1) {
          await held.promise;
        }
        return upgradeRead(server);
      },
    },
  });
  server.submission = { ...server.submission, report: keptReport };
  renderApp(upgradePath, withEvents(server.app), { eventHandlers });
  await section("Where it is");
  await waitFor(() => expect(events.streams).toHaveLength(1));
  await user.click(await screen.findByRole("button", { name: submits }));
  const dialog = await screen.findByRole("alertdialog", { name: "Submit for review?" });

  await user.click(within(dialog).getByRole("button", { name: "Submit for review" }));

  const kept = "The check at its submission (Oct 6, 2026, 9:41 AM): every check passed.";
  expect(await screen.findByText(kept)).toBeTruthy();
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(document.activeElement).toBe(title());
  expect(reads(server, "GET upgrade")).toBeGreaterThanOrEqual(2);
  expect(screen.queryByRole("heading", { level: 2, name: "Check" })).toBeNull();
  expect(screen.queryByRole("button", { name: submits })).toBeNull();
  expect(await screen.findByText(reviewingBanner)).toBeTruthy();
  held.resolve();
  await act(() => held.promise);
  expect(screen.getByText(kept)).toBeTruthy();
  expect(server.asked.filter((asked) => asked === "SUBMIT")).toHaveLength(1);
});

test("submitting an upgrade another admin submitted meanwhile is refused for its state: the page read again shows it in review; closed, the focus is on the title", async () => {
  const user = userEvent.setup();
  const server = upgradeServer({ upgrade: upgradeJSON() });
  renderApp(upgradePath, server.app);
  await user.click(await screen.findByRole("button", { name: submits }));
  const dialog = await screen.findByRole("alertdialog", { name: "Submit for review?" });
  server.withUpgrade(
    upgradeJSON({ state: "reviewing", submitted_by: bob, submitted_at: "2026-10-06T09:35:00Z", report: keptReport })
  );

  await user.click(within(dialog).getByRole("button", { name: "Submit for review" }));

  expect(await within(dialog).findByText("The upgrade's current state does not allow this.")).toBeTruthy();
  expect(
    await screen.findByText("The check at its submission (Oct 6, 2026, 9:41 AM): every check passed.")
  ).toBeTruthy();
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(document.activeElement).toBe(title());
  expect(checks(server)).toBe(0);
});

test.each([
  ["busy", "The server is busy. Try again in a moment."],
  ["forbidden", "You do not have permission to do this."],
] as const)("a submission refused as %s: the dialog says so, the check is not read again", async (refused, said) => {
  const user = userEvent.setup();
  const server = upgradeServer({ upgrade: upgradeJSON() });
  server.submission = { ...server.submission, outcome: "busy" };
  renderApp(upgradePath, server.app);
  await user.click(await screen.findByRole("button", { name: submits }));
  const dialog = await screen.findByRole("alertdialog", { name: "Submit for review?" });
  if (refused === "forbidden") {
    // Another admin made Ada an editor meanwhile.
    server.notebook = { ...server.notebook, role: "editor" };
  }

  await user.click(within(dialog).getByRole("button", { name: "Submit for review" }));

  expect(await within(dialog).findByText(said)).toBeTruthy();
  if (refused === "forbidden") {
    await waitFor(() => expect(screen.getByText(adminsOnly, { exact: true })).toBeTruthy());
  }
  expect([reads(server, "SUBMIT"), checks(server)]).toEqual([1, 0]);
});

// The review (M11/P3 design 4.3, decisions 288, 289, 291): Approve…, once
// the report is kept, Send back for more work… and Reject…, each through
// its dialog.

/** reviewed is Plans' upgrade in review, submitted by Bob, its report keptReport unless said. */
const reviewed = (more: Partial<Upgrade> = {}) =>
  upgradeJSON({
    state: "reviewing",
    submitted_by: bob,
    submitted_at: "2026-10-06T09:40:00Z",
    report: keptReport,
    ...more,
  });
const reviewTree = [guide, install, notes, seededRaw, seededAgents];
const stateRefused = "The upgrade's current state does not allow this.";
const upgradingBanner = "This notebook is being upgraded to an LLM Wiki: it is read-only for now.";
const approveTitle = "Approve the upgrade of “Plans”?";
const sendBackTitle = "Send back the upgrade of “Plans”?";
const rejectTitle = "Reject the upgrade of “Plans”?";
const approveBody = (root: string) =>
  `The notebook becomes an LLM Wiki: the pages and attachments under its source area (${root}) are read-only, AGENTS and the source root are protected, and the generated index and log appear in the page tree. There is no undoing the approval as a whole: the upgrade's changesets can still be reverted one by one in the history, as an LLM Wiki's rules allow; to make it a standard notebook again, downgrade it.`;
const stillChecked = "The previous submission is still being checked: try again in a moment.";

/** jobRowOf is what a job's row says (M11/P3 design 4.3): its name, its texts in order, its progress, its controls. */
const jobRowOf = (row: HTMLElement) => ({
  name: row.getAttribute("aria-label"),
  texts: within(row)
    .queryAllByText((_, element) => element !== null && element.children.length === 0 && element.textContent !== "")
    .map((element) => element.textContent),
  progress: within(row).getByRole("progressbar").getAttribute("aria-label"),
  buttons: within(row)
    .queryAllByRole("button")
    .map((button) => button.getAttribute("aria-label")),
});

/** The row of the rollback a rejection queued: started by the account, so not said by whom. */
const rejectedJobRow = {
  name: "Undo the upgrade, Oct 6, 2026, 8:00 AM",
  texts: ["Undo the upgrade", "Queued", "Started Oct 6, 2026, 8:00 AM", "Cancel"],
  progress: "Queued",
  buttons: ["Cancel Undo the upgrade, Oct 6, 2026, 8:00 AM"],
};
/** What the upgrade's page asked the server, but the rollback's job's reads. */
const askedOf = (server: { asked: string[] }) => server.asked.filter((asked) => asked !== "GET job upgrade");

/** opened presses the button named button and answers the dialog named name. */
async function opened(user: ReturnType<typeof userEvent.setup>, button: string, name: string) {
  await user.click(await screen.findByRole("button", { name: button }));
  return screen.findByRole("alertdialog", { name });
}

/** dismissed closes dialog with Cancel. */
async function dismissed(user: ReturnType<typeof userEvent.setup>, dialog: HTMLElement) {
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
}

test("the review's dialogs: Approve, plain, names the source area it makes read only by its path; Send back, plain; Reject, destructive, says what its rollback does, and that one stopped goes on or ends; closed, the focus is on the button", async () => {
  const user = userEvent.setup();
  const server = upgradeServer({ nodes: reviewTree, upgrade: reviewed() });
  renderApp(upgradePath, server.app);
  await waitFor(() => expect(server.sent).toContain("GET nodes"));
  const dialogs = [
    [approves, approveTitle, "Approve the upgrade", false, approveBody("raw.md")],
    [
      sendsBack,
      sendBackTitle,
      "Send it back",
      false,
      "The upgrade is upgrading again, and its initiator's agent can go on with it; this report is not kept, and the upgrade is to be submitted again.",
    ],
    [
      rejects,
      rejectTitle,
      "Reject the upgrade",
      true,
      "A background job puts the notebook back as it was when the upgrade started: its pages and attachments go back to their places, names and content, and those created since go to the trash. The notebook stays read-only until the rollback ends. A rollback that stops can be continued, or ended, keeping the notebook as it is then.",
    ],
  ] as const;

  const seen: unknown[] = [];
  for (const [button, name, confirm, , body] of dialogs) {
    // oxlint-disable-next-line no-await-in-loop -- one dialog at a time
    const dialog = await opened(user, button, name);
    seen.push([
      within(dialog).getByRole("button", { name: confirm }).className.includes("bg-destructive"),
      within(dialog).getByText(body) !== null,
    ]);
    // oxlint-disable-next-line no-await-in-loop -- one dialog at a time
    await dismissed(user, dialog);
    expect(document.activeElement).toBe(screen.getByRole("button", { name: button }));
  }

  expect(seen).toEqual(dialogs.map(([, , , destructive]) => [destructive, true]));
  expect(askedOf(server)).toEqual(["GET upgrade"]);
});

test("the approval's dialog, the tree not read yet, names the proposed source root", async () => {
  const user = userEvent.setup();
  const tree = deferred<void>();
  const server: ReturnType<typeof upgradeServer> = upgradeServer({
    nodes: reviewTree,
    upgrade: reviewed(),
    answers: {
      [`GET ${plans}/nodes`]: async () => {
        server.sent.push("GET nodes");
        await tree.promise;
        return json({ data: server.nodes });
      },
    },
  });
  renderApp(upgradePath, server.app);
  await waitFor(() => expect(server.sent).toContain("GET nodes"));

  const dialog = await opened(user, approves, approveTitle);

  expect(within(dialog).getByText(approveBody("the proposed source root"))).toBeTruthy();
  tree.resolve();
  expect(await within(dialog).findByText(approveBody("raw.md"))).toBeTruthy();
});

test("approved, the page goes to the notebook's home, its title focused, Plans an LLM Wiki without its banner; the upgrade, no longer under way, is not read again, by the approval's mode event nor by a connection after it", async () => {
  const user = userEvent.setup();
  // The stream is the test's: the mode event comes once the approval is answered, as when it is late.
  const events = eventServer();
  const server = upgradeServer({
    nodes: reviewTree,
    upgrade: reviewed(),
    answers: { "GET /api/v0/events": events.answer },
  });
  const { router } = renderApp(upgradePath, withEvents(server.app), { eventHandlers });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  const dialog = await opened(user, approves, approveTitle);

  await user.click(within(dialog).getByRole("button", { name: "Approve the upgrade" }));

  const home = await screen.findByRole("heading", { level: 1, name: "Plans" });
  expect(router.state.location.pathname).toBe(`/lab/notebooks/${notebookJSON.id}`);
  await waitFor(() => expect(document.activeElement).toBe(home));
  expect(screen.queryByRole("alertdialog")).toBeNull();
  const tree = screen.getByRole("navigation", { name: "Pages of Plans" });
  expect(await within(tree).findByRole("link", { name: "Directory" })).toBeTruthy();
  expect(screen.queryByText(reviewingBanner)).toBeNull();
  expect(askedOf(server)).toEqual(["GET upgrade", "APPROVE"]);

  // The approval's mode event, then a connection: each reads what is shown again, the tree last, not the upgrade.
  const treeReads = server.sent.filter((sent) => sent === "GET nodes").length;
  events.last().send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id });
  events.last().hello();
  await waitFor(() => expect(server.sent.filter((sent) => sent === "GET nodes").length).toBeGreaterThan(treeReads));
  expect(askedOf(server)).toEqual(["GET upgrade", "APPROVE"]);
});

test("approved, then Back to the upgrade's page: it shows no review while it reads the upgrade, and then that none is under way", async () => {
  const user = userEvent.setup();
  const { server, held } = secondReadHeld({ nodes: reviewTree, upgrade: reviewed() });
  const { router } = renderApp(upgradePath, server.app);
  await screen.findByRole("button", { name: approves });
  // Past SWR's two seconds of deduplication since the page read the upgrade, as a reader of the report is.
  await act(() => new Promise((resolve) => setTimeout(resolve, 2_100)));
  const dialog = await opened(user, approves, approveTitle);
  await user.click(within(dialog).getByRole("button", { name: "Approve the upgrade" }));
  await screen.findByRole("heading", { level: 1, name: "Plans" });

  await act(() => router.navigate(-1));

  await screen.findByRole("heading", { level: 1, name: "Upgrade to an LLM Wiki" });
  await waitFor(() => expect(reads(server, "GET upgrade")).toBe(2));
  for (const name of [approves, sendsBack, rejects]) {
    expect(screen.queryByRole("button", { name })).toBeNull();
  }
  expect(screen.queryByRole("heading", { level: 2, name: "Check report" })).toBeNull();
  held.resolve();
  expect(await screen.findByText(noneUnderWay)).toBeTruthy();
});

test("approved, the upgrade's page goes without showing the upgrade loading: it leaves the cache once the page is gone", async () => {
  const user = userEvent.setup();
  const server = upgradeServer({ nodes: reviewTree, upgrade: reviewed() });
  renderApp(upgradePath, server.app);
  const dialog = await opened(user, approves, approveTitle);
  const loading: string[] = [];
  const observer = new MutationObserver(() => {
    if (document.querySelector("h1")?.textContent !== "Upgrade to an LLM Wiki") return;
    for (const output of document.querySelectorAll("output")) {
      if (output.textContent === "Loading…") loading.push(output.textContent);
    }
  });
  observer.observe(document.body, { subtree: true, childList: true, characterData: true });

  await user.click(within(dialog).getByRole("button", { name: "Approve the upgrade" }));

  await screen.findByRole("heading", { level: 1, name: "Plans" });
  observer.disconnect();
  expect(loading).toEqual([]);
});

test("approved once the page is left, its answer late: the account stays where it went", async () => {
  const user = userEvent.setup();
  const answered = deferred<void>();
  const server: ReturnType<typeof upgradeServer> = upgradeServer({
    nodes: reviewTree,
    upgrade: reviewed(),
    answers: {
      [`POST ${plans}/upgrade/approve`]: async () => {
        server.asked.push("APPROVE");
        await answered.promise;
        const approved: Upgrade = { ...reviewed(), state: "approved" };
        server.withUpgrade(approved);
        return json(approved);
      },
    },
  });
  const { router } = renderApp(upgradePath, server.app);
  const dialog = await opened(user, approves, approveTitle);
  await user.click(within(dialog).getByRole("button", { name: "Approve the upgrade" }));
  await waitFor(() => expect(server.asked).toContain("APPROVE"));
  const elsewhere = `/lab/notebooks/${notebookJSON.id}/settings/general`;
  await act(() => router.navigate(elsewhere));
  await waitFor(() => expect(router.state.location.pathname).toBe(elsewhere));

  answered.resolve();
  await act(() => answered.promise);
  await act(() => new Promise((resolve) => setTimeout(resolve, 100)));

  expect(router.state.location.pathname).toBe(elsewhere);
});

test.each([
  [
    "a gate checked again not passing",
    "Checked again before the approval, a check did not pass; the upgrade is still in review: send it back, or reject it.",
    true,
  ],
  ["busy", "The server is busy. Try again in a moment.", false],
  ["in another state", stateRefused, true],
] as const)(
  "an approval refused as %s: the dialog says so, the upgrade read again where that says the page is behind; closed, the focus is on Approve, or on the title once it is gone",
  async (refused, said, reread) => {
    const user = userEvent.setup();
    const server = upgradeServer({ nodes: reviewTree, upgrade: reviewed() });
    server.approval = refused === "busy" ? "busy" : "not_ready";
    const { router } = renderApp(upgradePath, server.app);
    const dialog = await opened(user, approves, approveTitle);
    if (refused === "in another state") {
      // Another admin sent it back meanwhile.
      server.withUpgrade(upgradeJSON());
    }

    await user.click(within(dialog).getByRole("button", { name: "Approve the upgrade" }));

    expect(await within(dialog).findByText(said)).toBeTruthy();
    await waitFor(() =>
      expect(askedOf(server)).toEqual(["GET upgrade", "APPROVE", ...(reread ? ["GET upgrade"] : [])])
    );
    if (refused === "in another state") {
      expect(await screen.findByRole("button", { name: submits, hidden: true })).toBeTruthy();
    }
    expect(router.state.location.pathname).toBe(upgradePath);
    await dismissed(user, dialog);
    expect(document.activeElement).toBe(
      refused === "in another state" ? title() : screen.getByRole("button", { name: approves })
    );
    expect(askedOf(server)).toEqual(["GET upgrade", "APPROVE", ...(reread ? ["GET upgrade"] : [])]);
  }
);

test("rejected, the upgrade rolls back, rejected: the page shows its job queued, the banner follows, the focus is on the title", async () => {
  const user = userEvent.setup();
  const server = upgradeServer({ nodes: reviewTree, upgrade: reviewed() });
  renderApp(upgradePath, server.app);
  const dialog = await opened(user, rejects, rejectTitle);

  await user.click(within(dialog).getByRole("button", { name: "Reject the upgrade" }));

  const rollback = await section("Rollback");
  const job = within(rollback).getByRole("listitem", { name: (name) => name.startsWith("Undo the upgrade") });
  expect(jobRowOf(job)).toEqual(rejectedJobRow);
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(document.activeElement).toBe(title());
  expect(screen.queryByRole("button", { name: rejects })).toBeNull();
  expect(screen.queryByRole("button", { name: abandon })).toBeNull();
  expect(await screen.findByText(rollingBanner)).toBeTruthy();
  expect(server.upgrade?.ending).toBe("rejected");
  expect(askedOf(server)).toEqual(["GET upgrade", "REJECT", "GET upgrade"]);
});

test("rejected, the page shows the rollback the rejection answered while it reads the upgrade again: its job, Reject gone", async () => {
  const user = userEvent.setup();
  const { server, held } = secondReadHeld({ nodes: reviewTree, upgrade: reviewed() });
  renderApp(upgradePath, server.app);
  const dialog = await opened(user, rejects, rejectTitle);

  await user.click(within(dialog).getByRole("button", { name: "Reject the upgrade" }));

  const rollback = await section("Rollback");
  const job = await within(rollback).findByRole("listitem", { name: (name) => name.startsWith("Undo the upgrade") });
  expect(jobRowOf(job)).toEqual(rejectedJobRow);
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(screen.queryByRole("button", { name: rejects })).toBeNull();
  expect(screen.queryByRole("heading", { level: 2, name: "Check report" })).toBeNull();
  expect(reads(server, "GET upgrade")).toBe(2);
  held.resolve();
  await act(() => held.promise);
  expect(within(await section("Rollback")).getByText("Queued")).toBeTruthy();
});

test("sent back, the page shows the upgrade the send-back answered while it reads it again: the agent's section, Send back gone", async () => {
  const user = userEvent.setup();
  const { server, held } = secondReadHeld({ nodes: reviewTree, upgrade: reviewed() });
  renderApp(upgradePath, server.app);
  const dialog = await opened(user, sendsBack, sendBackTitle);

  await user.click(within(dialog).getByRole("button", { name: "Send it back" }));

  expect(await screen.findByRole("heading", { level: 2, name: "Connect an agent" })).toBeTruthy();
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(screen.queryByRole("button", { name: sendsBack })).toBeNull();
  expect(screen.getByRole("button", { name: submits })).toBeTruthy();
  expect(reads(server, "GET upgrade")).toBe(2);
  held.resolve();
  await act(() => held.promise);
  expect(screen.getByRole("heading", { level: 2, name: "Connect an agent" })).toBeTruthy();
});

test("sent back, the upgrade is upgrading again: its agent's section and Submit for review are back, the banner follows, the focus is on the title", async () => {
  const user = userEvent.setup();
  const server = upgradeServer({ nodes: reviewTree, upgrade: reviewed() });
  renderApp(upgradePath, server.app);
  const dialog = await opened(user, sendsBack, sendBackTitle);

  await user.click(within(dialog).getByRole("button", { name: "Send it back" }));

  expect(await screen.findByRole("heading", { level: 2, name: "Connect an agent" })).toBeTruthy();
  expect(screen.getByRole("button", { name: submits })).toBeTruthy();
  expect(screen.queryByRole("heading", { level: 2, name: "Check report" })).toBeNull();
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(document.activeElement).toBe(title());
  expect(await screen.findByText(upgradingBanner)).toBeTruthy();
  expect(askedOf(server)).toEqual(["GET upgrade", "SEND BACK", "GET upgrade"]);
});

test("checked, submitted and sent back, the check shows nothing of before: Check is to be pressed again", async () => {
  const user = userEvent.setup();
  const server = upgradeServer({ nodes: reviewTree, upgrade: upgradeJSON() });
  renderApp(upgradePath, server.app);
  const check = await checkOf();
  await user.click(within(check).getByRole("button", { name: "Check" }));
  await waitFor(() => expect(resultOf(check)).toBe(passedAt));
  const submit = await opened(user, submits, "Submit for review?");
  await user.click(within(submit).getByRole("button", { name: "Submit for review" }));
  await section("Check report");

  const dialog = await opened(user, sendsBack, sendBackTitle);
  await user.click(within(dialog).getByRole("button", { name: "Send it back" }));

  const again = await checkOf();
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(resultOf(again)).toBe("");
  expect(within(again).getByRole("button", { name: "Check" })).toBeTruthy();
  expect(within(again).queryByRole("list", { name: "The checks" })).toBeNull();
  expect(checks(server)).toBe(1);
});

test("checked, submitted, and sent back by another admin, the mode event reading it again: the check shows nothing of before (decision 300)", async () => {
  const user = userEvent.setup();
  const events = eventServer();
  const server = upgradeServer({ nodes: reviewTree, upgrade: upgradeJSON(), events });
  renderApp(upgradePath, withEvents(server.app), { eventHandlers });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  const check = await checkOf();
  await user.click(within(check).getByRole("button", { name: "Check" }));
  await waitFor(() => expect(resultOf(check)).toBe(passedAt));
  const submit = await opened(user, submits, "Submit for review?");
  await user.click(within(submit).getByRole("button", { name: "Submit for review" }));
  await section("Check report");
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());

  // Another admin sent it back.
  server.withUpgrade(upgradeJSON());
  events.last().send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id });

  const again = await checkOf();
  await waitFor(() => expect(screen.queryByRole("heading", { level: 2, name: "Check report" })).toBeNull());
  expect(resultOf(again)).toBe("");
  expect(within(again).getByRole("button", { name: "Check" })).toBeTruthy();
  expect(within(again).queryByRole("list", { name: "The checks" })).toBeNull();
  expect(checks(server)).toBe(1);
});

test("checked, then submitted by the agent and taken over and submitted again, each still checked, and sent back by another admin, the mode events reading it: the check shows nothing of before (decision 300)", async () => {
  const user = userEvent.setup();
  const events = eventServer();
  const server = upgradeServer({ nodes: reviewTree, upgrade: upgradeJSON(), events });
  renderApp(upgradePath, withEvents(server.app), { eventHandlers });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  const check = await checkOf();
  await user.click(within(check).getByRole("button", { name: "Check" }));
  await waitFor(() => expect(resultOf(check)).toBe(passedAt));
  const mode = () => events.last().send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id });

  // The agent submitted it, its report not kept yet.
  server.withUpgrade(reviewed({ report: null }));
  mode();
  await screen.findByText(pending);
  // Taken over and submitted again: another submission, its report not kept yet either.
  server.withUpgrade(reviewed({ report: null, submitted_at: "2026-10-06T09:50:00Z" }));
  mode();
  await waitFor(() => expect(reads(server, "GET upgrade")).toBe(3));
  // That read shown before the next one is sent, which would drop its answer.
  await act(() => new Promise((resolve) => setTimeout(resolve, 100)));
  // Another admin sent it back.
  server.withUpgrade(upgradeJSON());
  mode();

  const again = await checkOf();
  await waitFor(() => expect(screen.queryByText(pending)).toBeNull());
  expect(resultOf(again)).toBe("");
  expect(within(again).getByRole("button", { name: "Check" })).toBeTruthy();
  expect(within(again).queryByRole("list", { name: "The checks" })).toBeNull();
  expect(checks(server)).toBe(1);
});

/**
 * inReviewChecked renders the upgrade's page of Plans in review without a report, its submission still checked, and
 * opens Send back's dialog; given events, its stream connected.
 */
async function inReviewChecked(
  events?: ReturnType<typeof eventServer>,
  answers: (
    server: () => ReturnType<typeof upgradeServer>
  ) => NonNullable<Parameters<typeof upgradeServer>[0]>["answers"] = () => ({})
) {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const server: ReturnType<typeof upgradeServer> = upgradeServer({
    nodes: reviewTree,
    upgrade: reviewed({ report: null }),
    events,
    answers: answers(() => server),
  });
  const view =
    events === undefined
      ? renderApp(upgradePath, server.app)
      : renderApp(upgradePath, withEvents(server.app), { eventHandlers });
  await screen.findByText(pending);
  if (events !== undefined) {
    await waitFor(() => expect(events.streams).toHaveLength(1));
  }
  expect(screen.queryByRole("button", { name: approves })).toBeNull();
  const dialog = await opened(user, sendsBack, sendBackTitle);
  return { ...view, user, server, dialog };
}

test("sent back while its submission is still checked, refused: the dialog says so, not the server's words; the upgrade is read again, then once more after 20 seconds, and no more", async () => {
  const { user, server, dialog } = await inReviewChecked();

  await user.click(within(dialog).getByRole("button", { name: "Send it back" }));

  expect(await within(dialog).findByText(stillChecked)).toBeTruthy();
  expect(within(dialog).queryByText(stateRefused)).toBeNull();
  await waitFor(() => expect(reads(server, "GET upgrade")).toBe(2));
  await act(() => vi.advanceTimersByTimeAsync(19_000));
  expect(reads(server, "GET upgrade")).toBe(2);
  await act(() => vi.advanceTimersByTimeAsync(1_500));
  await waitFor(() => expect(reads(server, "GET upgrade")).toBe(3));
  await act(() => vi.advanceTimersByTimeAsync(60_000));
  expect(askedOf(server)).toEqual(["GET upgrade", "SEND BACK", "GET upgrade", "GET upgrade"]);
  expect(within(dialog).getByText(stillChecked)).toBeTruthy();
});

test("sent back while its submission is still checked, refused, the page left and come back to: the page reads the upgrade as it opens, and not again after 20 seconds", async () => {
  const { user, server, dialog, router } = await inReviewChecked();
  await user.click(within(dialog).getByRole("button", { name: "Send it back" }));
  await within(dialog).findByText(stillChecked);
  await waitFor(() => expect(reads(server, "GET upgrade")).toBe(2));

  await act(() => router.navigate(`/lab/notebooks/${notebookJSON.id}`));
  await screen.findByRole("heading", { level: 1, name: "Plans" });
  await act(() => vi.advanceTimersByTimeAsync(5_000));
  expect(askedOf(server)).toEqual(["GET upgrade", "SEND BACK", "GET upgrade"]);
  await act(() => router.navigate(upgradePath));
  await screen.findByText(pending);
  await waitFor(() => expect(reads(server, "GET upgrade")).toBe(3));
  await act(() => vi.advanceTimersByTimeAsync(60_000));

  expect(askedOf(server)).toEqual(["GET upgrade", "SEND BACK", "GET upgrade", "GET upgrade"]);
});

test("sent back while its submission is still checked, refused, then its report kept, shown by the mode event: the dialog still says it was checked; nothing is read after 20 seconds", async () => {
  const events = eventServer();
  const { user, server, dialog } = await inReviewChecked(events);
  await user.click(within(dialog).getByRole("button", { name: "Send it back" }));
  await within(dialog).findByText(stillChecked);
  await waitFor(() => expect(reads(server, "GET upgrade")).toBe(2));

  server.withUpgrade(reviewed());
  events.last().send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id });
  await screen.findByRole("button", { name: approves, hidden: true });
  // What the refusal was, as the upgrade was read again after it, the dialog still says (design review P3C-N14).
  expect(within(dialog).getByRole("alert").textContent).toBe(stillChecked);
  await act(() => vi.advanceTimersByTimeAsync(60_000));

  expect(askedOf(server).filter((asked) => asked.startsWith("GET upgrade") || asked === "SEND BACK")).toEqual([
    "GET upgrade",
    "SEND BACK",
    "GET upgrade",
    "GET upgrade",
  ]);
});

test("sent back while its submission was still checked, refused as another admin rejected it meanwhile, the read again held: the dialog says nothing of the refusal until it is read, and then not that it is checked", async () => {
  const held = deferred<void>();
  let gets = 0;
  const { user, server, dialog } = await inReviewChecked(undefined, (answering) => ({
    [`GET ${plans}/upgrade`]: async () => {
      answering().asked.push("GET upgrade");
      if (++gets === 2) {
        await held.promise;
      }
      return upgradeRead(answering());
    },
  }));
  const job = underWay(rollbackJobJSON(71), 0, 0);
  server.jobs = [job];
  server.withUpgrade(reviewed({ report: null, state: "rolling_back", ending: "rejected", rollback_job_id: job.id }));

  await user.click(within(dialog).getByRole("button", { name: "Send it back" }));
  await waitFor(() => expect(gets).toBe(2));
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));

  expect(within(dialog).queryByRole("alert")).toBeNull();
  expect(within(dialog).getByRole("button", { name: "Sending back…" })).toBeTruthy();
  held.resolve();
  expect(await within(dialog).findByText(stateRefused)).toBeTruthy();
  expect(within(dialog).queryByText(stillChecked)).toBeNull();
});

test("sent back while its submission was still checked, refused as another admin rejected it meanwhile, its read again overtaken by the rejection's mode event: read once more, the dialog says only that the state does not allow it", async () => {
  const events = eventServer();
  const gates: Array<ReturnType<typeof deferred<void>>> = [];
  const { user, server, dialog } = await inReviewChecked(events, (answering) => ({
    [`GET ${plans}/upgrade`]: async () => {
      answering().asked.push("GET upgrade");
      const read = upgradeRead(answering());
      if (reads(answering(), "GET upgrade") > 1) {
        const gate = deferred<void>();
        gates.push(gate);
        await gate.promise;
      }
      return read;
    },
  }));
  const job = underWay(rollbackJobJSON(71), 0, 0);
  server.jobs = [job];
  server.withUpgrade(reviewed({ report: null, state: "rolling_back", ending: "rejected", rollback_job_id: job.id }));
  const said: string[] = [];
  const observer = new MutationObserver(() => {
    const text = dialog.querySelector('[role="alert"]')?.textContent;
    if (text && said.at(-1) !== text) said.push(text);
  });
  observer.observe(dialog, { subtree: true, childList: true, characterData: true });

  await user.click(within(dialog).getByRole("button", { name: "Send it back" }));
  // The send-back's read again is out; the rejection's mode event comes late, its read after it.
  await waitFor(() => expect(gates).toHaveLength(1));
  events.last().send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id });
  await waitFor(() => expect(gates).toHaveLength(2));
  // The read again answers first, the cache as before: SWR drops its answer, the mode event's read being newer.
  gates[0]?.resolve();
  await waitFor(() => expect(gates).toHaveLength(3));
  gates[1]?.resolve();
  gates[2]?.resolve();

  expect(await screen.findByRole("heading", { level: 2, name: "Rollback", hidden: true })).toBeTruthy();
  expect(await within(dialog).findByText(stateRefused)).toBeTruthy();
  observer.disconnect();
  expect(said).toEqual([stateRefused]);
  expect(reads(server, "GET upgrade")).toBe(4);
});

test("sent back while the page shows its submission still checked, the server having kept its report and another admin approved it meanwhile: read again, none is under way, and the dialog says the state does not allow it; nothing is read after 20 seconds", async () => {
  const { user, server, dialog } = await inReviewChecked();
  server.withUpgrade(reviewed({ state: "approved" }));

  await user.click(within(dialog).getByRole("button", { name: "Send it back" }));

  expect(await screen.findByText(noneUnderWay)).toBeTruthy();
  expect(await within(dialog).findByText(stateRefused)).toBeTruthy();
  expect(within(dialog).queryByText(stillChecked)).toBeNull();
  await act(() => vi.advanceTimersByTimeAsync(25_000));
  expect(askedOf(server)).toEqual(["GET upgrade", "SEND BACK", "GET upgrade"]);
});

test("sent back while its submission is still checked, refused; its report kept, sent back by another admin and submitted again, the new submission still checked: nothing is read after 20 seconds, none refused for it", async () => {
  const events = eventServer();
  const { user, server, dialog } = await inReviewChecked(events);
  await user.click(within(dialog).getByRole("button", { name: "Send it back" }));
  await within(dialog).findByText(stillChecked);
  await waitFor(() => expect(reads(server, "GET upgrade")).toBe(2));
  await dismissed(user, dialog);
  const mode = () => events.last().send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id });
  server.withUpgrade(reviewed());
  mode();
  await screen.findByRole("button", { name: approves });
  server.withUpgrade(upgradeJSON());
  mode();
  await screen.findByRole("button", { name: submits });
  server.withUpgrade(reviewed({ report: null, submitted_at: "2026-10-06T10:40:00Z" }));
  mode();
  await screen.findByText(pending);
  await waitFor(() => expect(reads(server, "GET upgrade")).toBe(5));

  await act(() => vi.advanceTimersByTimeAsync(25_000));

  expect(reads(server, "GET upgrade")).toBe(5);
});

test("sent back while its submission is still checked, refused, then sent back again, the network failing: the next send-back clears the read once more, nothing is read after 20 seconds", async () => {
  let sends = 0;
  const { user, server, dialog } = await inReviewChecked(undefined, (answering) => ({
    [`POST ${plans}/upgrade/send-back`]: () => {
      answering().asked.push("SEND BACK");
      if (++sends === 1) {
        return problem(409, "wiki.upgrade_state", {
          detail: "The previous submission is still being checked: check again in a moment.",
        });
      }
      throw new TypeError("offline");
    },
  }));
  await user.click(within(dialog).getByRole("button", { name: "Send it back" }));
  await within(dialog).findByText(stillChecked);
  await waitFor(() => expect(reads(server, "GET upgrade")).toBe(2));
  await act(() => vi.advanceTimersByTimeAsync(10_000));

  await user.click(within(dialog).getByRole("button", { name: "Send it back" }));

  expect(await within(dialog).findByText("Cannot reach the server. Check the connection and try again.")).toBeTruthy();
  await act(() => vi.advanceTimersByTimeAsync(25_000));
  expect(askedOf(server)).toEqual(["GET upgrade", "SEND BACK", "GET upgrade", "SEND BACK"]);
});

test("sent back while its submission is still checked, the network failing: the dialog says so; the upgrade is not read again, after 20 seconds neither", async () => {
  const { user, server, dialog } = await inReviewChecked(undefined, (held) => ({
    [`POST ${plans}/upgrade/send-back`]: () => {
      held().asked.push("SEND BACK");
      throw new TypeError("offline");
    },
  }));

  await user.click(within(dialog).getByRole("button", { name: "Send it back" }));

  expect(await within(dialog).findByText("Cannot reach the server. Check the connection and try again.")).toBeTruthy();
  expect(within(dialog).queryByText(stillChecked)).toBeNull();
  await act(() => vi.advanceTimersByTimeAsync(25_000));
  expect(askedOf(server)).toEqual(["GET upgrade", "SEND BACK"]);
});

test("sent back while its submission was still checked, refused as another admin rejected it meanwhile: the dialog says the state does not allow it, as the upgrade read again rolls back; nothing is read after 20 seconds", async () => {
  const { user, server, dialog } = await inReviewChecked();
  const job = underWay(rollbackJobJSON(71), 0, 0);
  server.jobs = [job];
  server.withUpgrade(reviewed({ report: null, state: "rolling_back", ending: "rejected", rollback_job_id: job.id }));

  await user.click(within(dialog).getByRole("button", { name: "Send it back" }));

  // The dialog stays, the page behind it.
  expect(await screen.findByRole("heading", { level: 2, name: "Rollback", hidden: true })).toBeTruthy();
  expect(await within(dialog).findByText(stateRefused)).toBeTruthy();
  expect(within(dialog).queryByText(stillChecked)).toBeNull();
  await act(() => vi.advanceTimersByTimeAsync(25_000));
  expect(askedOf(server)).toEqual(["GET upgrade", "SEND BACK", "GET upgrade"]);
});

test("sent back with its report, refused as another admin approved it meanwhile: the dialog says the state does not allow it, and the page that none is under way", async () => {
  const user = userEvent.setup();
  const server = upgradeServer({ nodes: reviewTree, upgrade: reviewed() });
  renderApp(upgradePath, server.app);
  const dialog = await opened(user, sendsBack, sendBackTitle);
  server.withUpgrade(reviewed({ state: "approved" }));

  await user.click(within(dialog).getByRole("button", { name: "Send it back" }));

  expect(await within(dialog).findByText(stateRefused)).toBeTruthy();
  expect(await screen.findByText(noneUnderWay)).toBeTruthy();
  expect(within(dialog).queryByText(stillChecked)).toBeNull();
  expect(askedOf(server)).toEqual(["GET upgrade", "SEND BACK", "GET upgrade"]);
});
