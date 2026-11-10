import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { eventHandlers } from "../../events/handlers";
import type { TransferJob } from "../../services/transfer.service";
import type { Upgrade } from "../../services/upgrade.service";
import { eventServer, withEvents } from "../../test/event-server";
import { deferred } from "../../test/health-server";
import { json, notebookJSON, problem, workspaceJSON } from "../../test/fakes";
import { cancelled, reportOf, underWay } from "../../test/jobs-server";
import { guide, install, notes } from "../../test/page-server";
import { renderApp } from "../../test/render";
import {
  bobsUpgrade,
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

test("another admin is told whose agent writes and that they can abandon the upgrade, not the prompt", async () => {
  const server = upgradeServer({ upgrade: bobsUpgrade() });
  renderApp(upgradePath, server.app);

  const agent = await section("Connect an agent");
  expect(agent.textContent).toBe(
    "Connect an agentOnly the agent of Bob, who started the upgrade, can write this notebook; you can abandon the upgrade."
  );
  expect(within(agent).queryByText("upgrade")).toBeNull();
});

test.each([
  ["upgrading", true, true],
  ["reviewing", false, true],
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

test("abandoned, the page's read again still out, the focus is on the title, not on Abandon still shown", async () => {
  const user = userEvent.setup();
  const reread = deferred<void>();
  let answered = 0;
  const server: ReturnType<typeof upgradeServer> = upgradeServer({
    upgrade: upgradeJSON(),
    answers: {
      [`GET ${plans}/upgrade`]: async () => {
        server.asked.push("GET upgrade");
        if (++answered === 2) {
          await reread.promise;
        }
        return json({ ...server.upgrade, rollback_left: null });
      },
    },
  });
  renderApp(upgradePath, server.app);
  const button = await screen.findByRole("button", { name: abandon });

  await user.click(button);
  const dialog = await screen.findByRole("alertdialog", { name: "Abandon the upgrade of “Plans”?" });
  await user.click(within(dialog).getByRole("button", { name: "Abandon the upgrade" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(button.isConnected).toBe(true);
  expect(document.activeElement).toBe(title());
  reread.resolve();
  await section("Rollback");
  expect(button.isConnected).toBe(false);
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
