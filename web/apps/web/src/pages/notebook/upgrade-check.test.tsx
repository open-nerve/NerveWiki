import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { eventHandlers } from "../../events/handlers";
import type { UpgradeCheck } from "../../services/upgrade.service";
import { eventServer, withEvents } from "../../test/event-server";
import { json, notebookJSON, problem, workspaceJSON } from "../../test/fakes";
import { deferred } from "../../test/health-server";
import { guide, install, notes, pageNode } from "../../test/page-server";
import { renderApp } from "../../test/render";
import {
  checkJSON,
  missingTypes,
  ruleOf,
  seededAgents,
  seededRaw,
  upgradeJSON,
  upgradeServer,
} from "../../test/upgrade-server";

// The check of an upgrade on its page (M11/P2 design 4.3, decision 259):
// read when asked, a generation of the page's entry at a time.

afterEach(() => vi.useRealTimers());

const home = `/lab/notebooks/${notebookJSON.id}`;
const upgradePath = `${home}/upgrade`;
const plans = `/api/v0/notebooks/${notebookJSON.id}`;
const indexPage = pageNode(30, "index");
const gone = pageNode(31, "Gone");
const notePages = [1, 2, 3, 4, 5, 6, 7].map((n) => pageNode(40 + n, `Note ${n}`));
const tree = [guide, install, notes, seededRaw, seededAgents, indexPage, ...notePages];

/** checkSection is the upgrade page's check, by its heading. */
async function checkSection(): Promise<HTMLElement> {
  return (await screen.findByRole("heading", { level: 2, name: "Check" })).closest("section") as HTMLElement;
}
/** What the check's result says. */
const resultOf = (section: HTMLElement) =>
  within(section).getByRole("status", { name: "The check's result" }).textContent;
/** How many times the server was asked what. */
const asked = (server: { asked: string[] }, what: string) => server.asked.filter((each) => each === what).length;
const checks = (server: { asked: string[] }) => asked(server, "CHECK");

/** server is the upgrade server of Plans, upgrading, its tree tree, its check check unless said. */
function serverOf(check: UpgradeCheck = checkJSON(), answers: Parameters<typeof upgradeServer>[0] = {}) {
  const server = upgradeServer({ nodes: tree, upgrade: upgradeJSON(), ...answers });
  server.check = check;
  return server;
}

/** opened renders the upgrade's page of server, its tree read, and answers its check. */
async function opened(server: ReturnType<typeof upgradeServer>) {
  const view = renderApp(upgradePath, server.app);
  const section = await checkSection();
  await waitFor(() => expect(server.sent).toContain("GET nodes"));
  return { ...view, section };
}

/** checked presses Check in section and waits until its result says said. */
async function checked(user: ReturnType<typeof userEvent.setup>, section: HTMLElement, said: string) {
  await user.click(within(section).getByRole("button", { name: "Check" }));
  await waitFor(() => expect(resultOf(section)).toBe(said));
}

const passedAt = "Every check passed: the upgrade can be submitted for review. Checked on Oct 6, 2026, 9:30 AM.";

test("the check reads nothing as the page opens; Check reads it once, its result said, and is Check again", async () => {
  const user = userEvent.setup();
  const server = serverOf();
  const { section } = await opened(server);

  expect(server.asked).toEqual(["GET upgrade"]);
  expect(resultOf(section)).toBe("");
  expect(within(section).queryByRole("list", { name: "The checks" })).toBeNull();

  await checked(user, section, passedAt);
  expect(server.asked).toEqual(["GET upgrade", "CHECK"]);
  expect(within(section).getByRole("button", { name: "Check again" })).toBeTruthy();
  expect(within(section).queryByRole("button", { name: "Check" })).toBeNull();
});

// Each gate, passed or not, with what it names (M11 design 4.7). Five
// checks, each gate's column of results its own, none the same in all.

type Gate = keyof UpgradeCheck["gates"];
const gateKeys: Gate[] = [
  "agents",
  "source_root",
  "reserved_titles",
  "frontmatter",
  "sources",
  "links",
  "preserved",
  "index",
];
const gateText: Record<Gate, string> = {
  agents: "The AGENTS page at the root has a body",
  source_root: "The source root is a page, not the root's AGENTS",
  reserved_titles: "No node at the root is named index, log, index.md or log.md",
  frontmatter:
    "Each page outside the source area, the source root and the root's AGENTS included, has frontmatter that parses; each except these two has a type and a summary",
  sources: "Each sources reference leads to a node of the source area",
  links: "No page the upgrade started with leads to more pages that do not exist",
  preserved: "Each node the upgrade started with is there, and no page's body changed",
  index: "The link index is fresh",
};
const first: Record<Gate, boolean> = {
  agents: true,
  source_root: true,
  reserved_titles: true,
  frontmatter: true,
  sources: false,
  links: false,
  preserved: false,
  index: false,
};
const rows: [string, Record<Gate, boolean>, string][] = [
  ["the first", first, "4 checks did not pass."],
  [
    "the second",
    {
      agents: true,
      source_root: false,
      reserved_titles: false,
      frontmatter: true,
      sources: true,
      links: true,
      preserved: false,
      index: false,
    },
    "4 checks did not pass.",
  ],
  [
    "the third",
    {
      agents: false,
      source_root: true,
      reserved_titles: false,
      frontmatter: true,
      sources: true,
      links: false,
      preserved: true,
      index: false,
    },
    "4 checks did not pass.",
  ],
  [
    "every one",
    {
      agents: true,
      source_root: true,
      reserved_titles: true,
      frontmatter: true,
      sources: true,
      links: true,
      preserved: true,
      index: true,
    },
    "Every check passed: the upgrade can be submitted for review.",
  ],
  [
    "the frontmatter alone",
    {
      agents: true,
      source_root: true,
      reserved_titles: true,
      frontmatter: false,
      sources: true,
      links: true,
      preserved: true,
      index: true,
    },
    "1 check did not pass.",
  ],
];

/** checkOf is a check whose gates passed as passed says, each failing one naming what it names. */
function checkOf(passed: Record<Gate, boolean>): UpgradeCheck {
  return checkJSON({
    stale_pages: passed.index ? 0 : 4,
    passed: gateKeys.every((key) => passed[key]),
    gates: {
      agents: { passed: passed.agents },
      source_root: { passed: passed.source_root },
      reserved_titles: {
        passed: passed.reserved_titles,
        node_ids: passed.reserved_titles ? [] : [indexPage.id, gone.id],
      },
      frontmatter: { passed: passed.frontmatter },
      sources: { passed: passed.sources },
      links: passed.links
        ? { passed: true, pages_worse: 0, node_ids: [] }
        : { passed: false, pages_worse: 3, node_ids: [notes.id, guide.id] },
      preserved: passed.preserved
        ? { passed: true, nodes: 9, nodes_kept: 9, pages: 6, pages_kept: 6, node_ids: [] }
        : { passed: false, nodes: 9, nodes_kept: 7, pages: 6, pages_kept: 5, node_ids: [install.id] },
      index: { passed: passed.index },
    },
  });
}

/** What the page says of each gate: its line, then what it adds, a list of nodes as their names. */
function gateLines(section: HTMLElement) {
  const list = within(section).getByRole("list", { name: "The checks" });
  return [...list.children].map((gate) =>
    [...gate.children].map((part) =>
      part.tagName === "UL" ? [...part.children].map((node) => node.textContent) : part.textContent
    )
  );
}

test.each(rows)(
  "a check whose gates passed as %s: each gate's line, what it names; the result",
  async (_, passed, said) => {
    const user = userEvent.setup();
    const { section } = await opened(serverOf(checkOf(passed)));

    await checked(user, section, `${said} Checked on Oct 6, 2026, 9:30 AM.`);

    expect(gateLines(section)).toEqual(
      gateKeys.map((key) => {
        const line = `${gateText[key]}: ${passed[key] ? "passed" : "not passed"}`;
        switch (key) {
          case "reserved_titles":
            return passed[key] ? [line] : [line, ["index.md", "(no longer there)"]];
          case "links":
            return passed[key]
              ? [line]
              : [
                  line,
                  "3 pages lead to more pages that do not exist than when the upgrade started.",
                  ["Notes.md", "Guide.md"],
                ];
          case "preserved":
            return passed[key]
              ? [line, "Nodes kept: 9 of 9; pages kept: 6 of 6."]
              : [line, "Nodes kept: 7 of 9; pages kept: 5 of 6.", ["Guide/Install.md"]];
          case "index":
            return passed[key]
              ? [line]
              : [line, "4 pages have an outdated index. Ask the server administrator to run nervewiki reindex."];
          default:
            return [line];
        }
      })
    );
  }
);

test("a gate's node leads to it: to its page, by its path", async () => {
  const user = userEvent.setup();
  const { section } = await opened(serverOf(checkOf(first)));
  await checked(user, section, `4 checks did not pass. Checked on Oct 6, 2026, 9:30 AM.`);

  const links = within(within(section).getByRole("list", { name: "The checks" }))
    .getAllByRole("link")
    .map((link) => [link.textContent, link.getAttribute("href")]);
  expect(links).toEqual([
    ["Notes.md", `${home}/pages/${notes.id}`],
    ["Guide.md", `${home}/pages/${guide.id}`],
    ["Guide/Install.md", `${home}/pages/${install.id}`],
  ]);
});

/** What a list of counts says: each term and its count. */
const countsOf = (holder: HTMLElement) =>
  [...holder.querySelectorAll("dt")].map((term) => [term.textContent, term.nextElementSibling?.textContent]);

test.each([
  [
    "before capped, now not",
    {
      sources: 11,
      moved_into_sources: 12,
      wiki_pages: 13,
      new_pages: 14,
      changesets: 15,
      wanted_before: { count: 16, count_capped: true },
      wanted_now: { count: 17, count_capped: false },
    },
    ["11", "12", "13", "14", "15", "16+", "17"],
  ],
  [
    "now capped, before not",
    {
      sources: 2101,
      moved_into_sources: 22,
      wiki_pages: 23,
      new_pages: 24,
      changesets: 25,
      wanted_before: { count: 26, count_capped: false },
      wanted_now: { count: 10000, count_capped: true },
    },
    ["2,101", "22", "23", "24", "25", "26", "10,000+"],
  ],
] as const)("a check's counts, the wanted pages %s: each count as it says", async (_, counts, shown) => {
  const user = userEvent.setup();
  const { section } = await opened(serverOf(checkJSON({ counts })));

  await checked(user, section, passedAt);

  expect(countsOf(section)).toEqual(
    [
      "Nodes in the source area",
      "Nodes it started with, moved into the source area",
      "Pages outside the source area",
      "Pages created since the upgrade started",
      "Changes of the upgrade",
      "Wanted pages as the upgrade started",
      "Wanted pages now",
    ].map((term, at) => [term, shown[at]])
  );
});

/** What the cards under holder say: each heading's level and text, and its count. */
const cardsOf = (holder: HTMLElement) =>
  [...holder.querySelectorAll<HTMLElement>("section[aria-labelledby]")].map((each) => {
    const heading = each.querySelector("h2, h3, h4");
    return [heading?.tagName, heading?.textContent, each.querySelectorAll("p")[1]?.textContent];
  });

/** A check whose four rules have issues, the wanted pages' and the ambiguous links' capped, seven missing types. */
function rulesCheck(): UpgradeCheck {
  const types = missingTypes(notePages.map((page) => page.id));
  return checkJSON({
    rules: [
      ruleOf("wanted", 31, [], { count_capped: false }),
      ruleOf("orphans", 0),
      ruleOf("ambiguous", 2, [], { count_capped: true }),
      ruleOf("invalid_frontmatter", 0),
      ruleOf("missing_type", 7, types.slice(0, 5)),
      ruleOf("missing_summary", 1),
      ruleOf("undigested_sources", 0),
      ruleOf("invalid_sources", 0),
    ],
  });
}

test("a check's rules with issues are cards headed under the check, the others in a disclosure; each says its count", async () => {
  const user = userEvent.setup();
  const { section } = await opened(serverOf(rulesCheck()));

  await checked(user, section, passedAt);

  const details = section.querySelector("details") as HTMLElement;
  expect(details.querySelector("summary")?.textContent).toBe("No issues (4 checks)");
  expect(cardsOf(details)).toEqual([
    ["H3", "Orphan pages", "0 issues"],
    ["H3", "Invalid frontmatter", "0 issues"],
    ["H3", "Undigested sources", "0 issues"],
    ["H3", "Invalid sources", "0 issues"],
  ]);
  expect(cardsOf(section).filter((shown) => !cardsOf(details).some((clear) => clear[1] === shown[1]))).toEqual([
    ["H3", "Wanted pages", "31 issues"],
    ["H3", "Ambiguous links", "2+ issues"],
    ["H3", "Missing type", "7 issues"],
    ["H3", "Missing summary", "1 issue"],
  ]);
});

/** The card of the check headed name. */
function cardOf(section: HTMLElement, name: string) {
  return within(within(section).getByRole("heading", { level: 3, name }).closest("section") as HTMLElement);
}

test("a card's More reads the check's next items of its rule after its cursor, not the health check's", async () => {
  const user = userEvent.setup();
  const server = serverOf(rulesCheck());
  const types = missingTypes(notePages.map((page) => page.id));
  server.checkItems = { missing_type: types };
  const { section } = await opened(server);
  await checked(user, section, passedAt);
  const missing = cardOf(section, "Missing type");
  expect(missing.getAllByRole("listitem").map((item) => item.textContent)).toEqual([
    "Note 1.md",
    "Note 2.md",
    "Note 3.md",
    "Note 4.md",
    "Note 5.md",
  ]);

  await user.click(missing.getByRole("button", { name: "More" }));

  await waitFor(() => expect(missing.getAllByRole("listitem")).toHaveLength(7));
  expect(missing.getAllByRole("listitem").map((item) => item.textContent)).toEqual([
    "Note 1.md",
    "Note 2.md",
    "Note 3.md",
    "Note 4.md",
    "Note 5.md",
    "Note 6.md",
    "Note 7.md",
  ]);
  expect(server.asked).toEqual(["GET upgrade", "CHECK", `CHECK missing_type ${types[4]?.cursor}`]);
  expect(missing.queryByRole("button", { name: "More" })).toBeNull();
});

test("a card's More refused as the upgrade no longer under way says the upgrade's state changed and reads it again: it ended", async () => {
  const user = userEvent.setup();
  const reread = deferred<void>();
  let reads = 0;
  const server: ReturnType<typeof upgradeServer> = serverOf(rulesCheck(), {
    answers: {
      [`GET ${plans}/upgrade`]: async () => {
        server.asked.push("GET upgrade");
        if (++reads === 2) {
          await reread.promise;
          return problem(404, "wiki.upgrade_not_found");
        }
        return json(server.upgrade);
      },
    },
  });
  const { section } = await opened(server);
  await checked(user, section, passedAt);
  // Another admin abandoned it meanwhile, and its rollback ended.
  server.withUpgrade(upgradeJSON({ state: "rolled_back", ending: "abandoned", rollback_complete: true }));

  await user.click(cardOf(section, "Missing type").getByRole("button", { name: "More" }));

  expect(await within(section).findByText("The upgrade's state changed.")).toBeTruthy();
  await waitFor(() => expect(asked(server, "GET upgrade")).toBe(2));
  expect((cardOf(section, "Missing type").getByRole("group") as HTMLFieldSetElement).disabled).toBe(true);
  reread.resolve();
  expect(await screen.findByText("This notebook has no upgrade under way.")).toBeTruthy();
  expect(screen.queryByRole("heading", { level: 2, name: "Check" })).toBeNull();
});

test("a check refused as busy is not tried again on its own: Try again reads it", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const server = serverOf();
  server.checkBusy = true;
  const { section } = await opened(server);

  await user.click(within(section).getByRole("button", { name: "Check" }));

  expect(await within(section).findByText("The server is busy. Try again in a moment.")).toBeTruthy();
  await act(() => vi.advanceTimersByTimeAsync(300_000));
  expect(checks(server)).toBe(1);
  server.checkBusy = false;
  await user.click(within(section).getByRole("button", { name: "Try again" }));
  await waitFor(() => expect(resultOf(section)).toBe(passedAt));
  expect(checks(server)).toBe(2);
});

test("a check refused as forbidden reads the notebooks again in the public provider: the account no longer the admin, the page says who can see it", async () => {
  const user = userEvent.setup();
  let lists = 0;
  const server: ReturnType<typeof upgradeServer> = serverOf(checkJSON(), {
    answers: {
      "GET /api/v0/workspaces/lab/notebooks": () => {
        lists++;
        return json({ data: [server.notebook] });
      },
    },
  });
  const { section } = await opened(server);
  const before = lists;
  // Another admin made Ada an editor meanwhile.
  server.notebook = { ...server.notebook, role: "editor" };

  await user.click(within(section).getByRole("button", { name: "Check" }));

  expect(await screen.findByText("Only the notebook's admins can see its upgrade.")).toBeTruthy();
  expect(lists).toBe(before + 1);
  expect(checks(server)).toBe(1);
});

test("a card's More refused as forbidden reads the notebooks again too: the account no longer the admin, the page says who can see it", async () => {
  const user = userEvent.setup();
  let lists = 0;
  const server: ReturnType<typeof upgradeServer> = serverOf(rulesCheck(), {
    answers: {
      "GET /api/v0/workspaces/lab/notebooks": () => {
        lists++;
        return json({ data: [server.notebook] });
      },
    },
  });
  server.checkItems = { missing_type: missingTypes(notePages.map((page) => page.id)) };
  const { section } = await opened(server);
  await checked(user, section, passedAt);
  const before = lists;
  // Another admin made Ada an editor meanwhile.
  server.notebook = { ...server.notebook, role: "editor" };

  await user.click(cardOf(section, "Missing type").getByRole("button", { name: "More" }));

  expect(await screen.findByText("Only the notebook's admins can see its upgrade.")).toBeTruthy();
  expect(lists).toBe(before + 1);
  expect(checks(server)).toBe(1);
});

test("checked again, the last check's request is aborted, and its answer, late, is not shown", async () => {
  const user = userEvent.setup();
  const late = deferred<Response>();
  const signals: AbortSignal[] = [];
  const server: ReturnType<typeof upgradeServer> = serverOf(checkJSON(), {
    answers: {
      [`GET ${plans}/upgrade/check`]: (request) => {
        server.asked.push("CHECK");
        signals.push(request.signal);
        return signals.length === 1 ? late.promise : json(checkJSON({ computed_at: "2026-10-06T10:45:00Z" }));
      },
    },
  });
  const { section } = await opened(server);
  await user.click(within(section).getByRole("button", { name: "Check" }));
  await waitFor(() => expect(signals).toHaveLength(1));
  expect(resultOf(section)).toBe("Checking…");

  await user.click(within(section).getByRole("button", { name: "Check again" }));

  const again = "Every check passed: the upgrade can be submitted for review. Checked on Oct 6, 2026, 10:45 AM.";
  await waitFor(() => expect(resultOf(section)).toBe(again));
  expect(signals.map((signal) => signal.aborted)).toEqual([true, false]);
  await act(async () => late.resolve(json(checkJSON())));
  expect(resultOf(section)).toBe(again);
  expect(checks(server)).toBe(2);
});

test.each([
  ["back", -1],
  ["by its link", upgradePath],
] as const)(
  "checked once, the page left and come back to %s is a new entry: its check is not read, nothing is shown; checked, it is read anew",
  async (_, to) => {
    const user = userEvent.setup();
    const server = serverOf();
    const { router, section } = await opened(server);
    await checked(user, section, passedAt);

    await act(() => router.navigate(home));
    await screen.findByRole("heading", { level: 1, name: "Plans" });
    await act(() => (typeof to === "number" ? router.navigate(to) : router.navigate(to)));

    const again = await checkSection();
    expect(resultOf(again)).toBe("");
    expect(within(again).getByRole("button", { name: "Check" })).toBeTruthy();
    expect(within(again).queryByRole("list", { name: "The checks" })).toBeNull();
    expect(checks(server)).toBe(1);
    server.check = checkJSON({ computed_at: "2026-10-06T11:15:00Z" });
    await checked(user, again, passedAt.replace("9:30", "11:15"));
    expect(checks(server)).toBe(2);
  }
);

test("checked once, the page's own link followed on it is a new entry: its check is not read, nothing is shown; checked, it is read anew", async () => {
  const user = userEvent.setup();
  const server = serverOf();
  const { router, section } = await opened(server);
  await checked(user, section, passedAt);

  await act(() => router.navigate(upgradePath));

  await waitFor(() =>
    expect(within(screen.getByRole("main")).queryByRole("button", { name: "Check again" })).toBeNull()
  );
  const again = await checkSection();
  expect(resultOf(again)).toBe("");
  expect(within(again).getByRole("button", { name: "Check" })).toBeTruthy();
  expect(checks(server)).toBe(1);
  server.check = checkJSON({ computed_at: "2026-10-06T11:15:00Z" });
  await checked(user, again, passedAt.replace("9:30", "11:15"));
  expect(checks(server)).toBe(2);
});

/**
 * live renders the upgrade's page of server with its event stream,
 * connected, and answers its check section; the test sends the mode
 * events another's submission would.
 */
async function live(server: ReturnType<typeof upgradeServer>, events: ReturnType<typeof eventServer>) {
  renderApp(upgradePath, withEvents(server.app), { eventHandlers });
  const section = await checkSection();
  await waitFor(() => expect(events.streams).toHaveLength(1));
  await waitFor(() => expect(server.sent).toContain("GET nodes"));
  return section;
}
const mode = (events: ReturnType<typeof eventServer>) =>
  events.last().send("mode", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id });

test("a check under way as another's submission takes the check away and gives it back: it shows what it read, read once", async () => {
  const user = userEvent.setup();
  const events = eventServer();
  const answer = deferred<void>();
  const server: ReturnType<typeof upgradeServer> = serverOf(rulesCheck(), {
    answers: {
      "GET /api/v0/events": events.answer,
      [`GET ${plans}/upgrade/check`]: async () => {
        server.asked.push("CHECK");
        await answer.promise;
        return json(server.check);
      },
    },
  });
  const section = await live(server, events);
  await user.click(within(section).getByRole("button", { name: "Check" }));
  await waitFor(() => expect(checks(server)).toBe(1));

  // Another admin's submission is checked, then sent back upgrading.
  server.withUpgrade(upgradeJSON({ state: "reviewing", submitted_by: { user_id: "bob", display_name: "Bob" } }));
  mode(events);
  await waitFor(() => expect(screen.queryByRole("heading", { level: 2, name: "Check" })).toBeNull());
  server.withUpgrade(upgradeJSON());
  mode(events);
  const back = await checkSection();
  expect(resultOf(back)).toBe("Checking…");
  answer.resolve();

  await waitFor(() => expect(resultOf(back)).toBe(passedAt));
  expect(cardsOf(back).map((each) => each[1])).toContain("Missing type");
  expect(checks(server)).toBe(1);
});

test("checked, the upgrade ended and another started while the page is open: the new one's check is not read until Check is pressed, then read anew", async () => {
  const user = userEvent.setup();
  const events = eventServer();
  const server = serverOf(checkJSON(), { events });
  const section = await live(server, events);
  await checked(user, section, passedAt);

  // Abandoned and rolled back, then started again by another admin.
  server.withUpgrade(undefined);
  mode(events);
  expect(await screen.findByText("This notebook has no upgrade under way.")).toBeTruthy();
  server.withUpgrade(upgradeJSON({ id: "0199a000-0000-7000-8000-00000000beef" }));
  server.check = checkJSON({ computed_at: "2026-10-06T11:15:00Z" });
  mode(events);
  const next = await checkSection();

  expect(resultOf(next)).toBe("");
  expect(within(next).getByRole("button", { name: "Check" })).toBeTruthy();
  expect(checks(server)).toBe(1);
  await checked(user, next, passedAt.replace("9:30", "11:15"));
  expect(checks(server)).toBe(2);
});

test("a check refused as busy, its section taken away and given back, is not read again: Try again is still offered", async () => {
  const user = userEvent.setup();
  const events = eventServer();
  const server = serverOf(checkJSON(), { answers: { "GET /api/v0/events": events.answer } });
  server.checkBusy = true;
  const section = await live(server, events);
  await user.click(within(section).getByRole("button", { name: "Check" }));
  await within(section).findByText("The server is busy. Try again in a moment.");

  server.withUpgrade(upgradeJSON({ state: "reviewing", submitted_by: { user_id: "bob", display_name: "Bob" } }));
  mode(events);
  await waitFor(() => expect(screen.queryByRole("heading", { level: 2, name: "Check" })).toBeNull());
  server.checkBusy = false;
  server.withUpgrade(upgradeJSON());
  mode(events);

  const back = await checkSection();
  expect(await within(back).findByText("The server is busy. Try again in a moment.")).toBeTruthy();
  expect(within(back).getByRole("button", { name: "Try again" })).toBeTruthy();
  expect(checks(server)).toBe(1);
});

const resultBehind = (section: HTMLElement) =>
  within(section).getByRole("status", { name: "The check's result", hidden: true }).textContent;

test.each([
  ["after its check is given back", false],
  ["while its check is away", true],
])(
  "a check under way as the admin's own submission is refused as busy, the check answered %s: the check given back shows what it read, read once",
  async (_, away) => {
    const user = userEvent.setup();
    const events = eventServer();
    const answer = deferred<void>();
    const second = deferred<void>();
    const server: ReturnType<typeof upgradeServer> = serverOf(rulesCheck(), {
      events,
      answers: {
        [`GET ${plans}/upgrade/check`]: async () => {
          server.asked.push("CHECK");
          await answer.promise;
          return json(server.check);
        },
      },
    });
    server.submission = { ...server.submission, outcome: "busy", checked: second.promise };
    const section = await live(server, events);
    await user.click(within(section).getByRole("button", { name: "Check" }));
    await waitFor(() => expect(checks(server)).toBe(1));
    await user.click(screen.getByRole("button", { name: "Submit for review…" }));
    const dialog = await screen.findByRole("alertdialog", { name: "Submit for review?" });

    // The submission's first step takes the check away; its second, the
    // server too busy, gives it back.
    await user.click(within(dialog).getByRole("button", { name: "Submit for review" }));
    await waitFor(() => expect(screen.queryByRole("heading", { level: 2, name: "Check", hidden: true })).toBeNull());
    if (away) {
      answer.resolve();
      await act(() => answer.promise);
    }
    second.resolve();
    expect(await within(dialog).findByText("The server is busy. Try again in a moment.")).toBeTruthy();
    const back = (await screen.findByRole("heading", { level: 2, name: "Check", hidden: true })).closest(
      "section"
    ) as HTMLElement;
    if (!away) {
      expect(resultBehind(back)).toBe("Checking…");
      answer.resolve();
    }

    await waitFor(() => expect(resultBehind(back)).toBe(passedAt));
    expect(checks(server)).toBe(1);
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    expect(cardsOf(back).map((each) => each[1])).toContain("Missing type");
  }
);
