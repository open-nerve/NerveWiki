import { act, screen, waitFor, within } from "@testing-library/react";
import { useEffect } from "react";
import { Outlet } from "react-router";
import { expect, test } from "vitest";

import { routes } from "../../app/routes";
import type { Notebook, NotebookRole } from "../../services/notebook.service";
import { useStore } from "../../stores/context";
import type { RootStore } from "../../stores/root.store";
import { notebookJSON, upgradeOf, workspaceJSON } from "../../test/fakes";
import { changesetId, created } from "../../test/history-server";
import { bob, guide, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The banner of a notebook's upgrade under way, above each of its pages
// (M11/P1 design 4.4): what it says by the upgrade's state, and what more
// it gives the notebook's admins (decision 246): that any of them can
// abandon it while it is upgrading, reject it while it is in review
// (decision 291).

const said = {
  upgrading: "This notebook is being upgraded to an LLM Wiki: it is read-only for now.",
  reviewing: "This notebook's upgrade is submitted and waits for an admin's review: it is read-only for now.",
  rolling_back: "This notebook's upgrade is being undone: it is read-only for now.",
} as const;
const anyAdmin = "Any admin of the notebook can abandon this upgrade.";
const anyAdminRejects = "Any admin of the notebook can reject this upgrade.";
/** What the banner tells the notebook's admins any of them can do, by the upgrade's state: nothing once it rolls back. */
const anyAdminSays = { upgrading: anyAdmin, reviewing: anyAdminRejects, rolling_back: undefined } as const;
const upgradePath = `/lab/notebooks/${notebookJSON.id}/upgrade`;

type State = keyof typeof said;

/** bannerOf is the banner saying text, a status, or null. */
function bannerOf(text: string): HTMLElement | null {
  const banner = screen.queryByText(text)?.parentElement ?? null;
  return banner !== null && screen.queryAllByRole("status").includes(banner) ? banner : null;
}

test.each(
  (["upgrading", "reviewing", "rolling_back"] as const).flatMap((state) =>
    (["admin", "editor", "reader"] as const).map((role) => [state, role] as const)
  )
)("an upgrade %s: a notebook's %s reads it in a status above the page", async (state, role) => {
  const server = pageServer({ role });
  server.notebook = { ...server.notebook, upgrade: upgradeOf(state, bob.user_id) };
  renderApp(pagePath(guide.id), server.app);

  await screen.findByRole("article", { name: "Guide" });
  const banner = bannerOf(said[state]);
  expect(banner).not.toBeNull();
  const admin = role === "admin";
  const link = within(banner as HTMLElement).queryByRole("link", { name: "View the upgrade" });
  expect(link?.getAttribute("href") ?? null).toBe(admin ? upgradePath : null);
  // Who may abandon it, while it is upgrading, or reject it, in review: neither once it rolls back.
  const says = [anyAdmin, anyAdminRejects].filter(
    (text) => within(banner as HTMLElement).queryByText(text, { exact: false }) !== null
  );
  expect(says).toEqual(admin && anyAdminSays[state] !== undefined ? [anyAdminSays[state]] : []);
  for (const other of Object.keys(said) as State[]) {
    if (other !== state) {
      expect(screen.queryByText(said[other])).toBeNull();
    }
  }
});

test("a notebook without an upgrade has no banner", async () => {
  const server = pageServer();
  renderApp(pagePath(guide.id), server.app);

  await screen.findByRole("article", { name: "Guide" });
  for (const text of Object.values(said)) {
    expect(screen.queryByText(text)).toBeNull();
  }
  expect(screen.queryByRole("link", { name: "View the upgrade" })).toBeNull();
});

/** captured renders path for server's app, keeping the stores of the generation the page uses. */
function captured(path: string, server: ReturnType<typeof pageServer>) {
  let root: RootStore | undefined;
  function Capture() {
    const store = useStore();
    useEffect(() => {
      root = store;
    }, [store]);
    return <Outlet />;
  }
  renderApp(path, server.app, { routes: [{ Component: Capture, children: routes }] });
  return {
    /** receive has the list hold Plans with upgrade, as the list read again on the mode event would. */
    receive(upgrade: Notebook["upgrade"], role: NotebookRole = "admin"): void {
      server.notebook = { ...server.notebook, role, upgrade };
      act(() => root?.notebooksOf(workspaceJSON)?.receive(server.notebook));
    },
  };
}

test("the banner comes and goes with the upgrade, taking no focus, the page staying as it is", async () => {
  const server = pageServer();
  const page = captured(pagePath(guide.id), server);
  const article = await screen.findByRole("article", { name: "Guide" });
  const tree = screen.getByRole("navigation", { name: "Pages of Plans" });
  const link = within(tree).getByRole("link", { name: "Guide" });
  link.focus();

  page.receive(upgradeOf("upgrading"));

  expect(bannerOf(said.upgrading)).not.toBeNull();
  expect(document.activeElement).toBe(link);
  expect(screen.getByRole("article", { name: "Guide" })).toBe(article);

  page.receive(upgradeOf("rolling_back"));
  expect(bannerOf(said.rolling_back)).not.toBeNull();
  expect(screen.queryByText(said.upgrading)).toBeNull();
  expect(document.activeElement).toBe(link);

  page.receive(null);
  expect(screen.queryByText(said.rolling_back)).toBeNull();
  expect(document.activeElement).toBe(link);
  expect(screen.getByRole("article", { name: "Guide" })).toBe(article);
});

test.each(["upgrading", "reviewing", "rolling_back"] as const)(
  "on the upgrade's own page, an upgrade %s's banner leads nowhere: the page is there",
  async (state) => {
    const server = pageServer();
    server.notebook = { ...server.notebook, upgrade: upgradeOf(state, bob.user_id) };
    renderApp(upgradePath, server.app);

    await screen.findByRole("heading", { level: 1, name: "Upgrade to an LLM Wiki" });
    const banner = bannerOf(said[state]);
    expect(banner).not.toBeNull();
    expect(within(banner as HTMLElement).queryByRole("link")).toBeNull();
    const says = [anyAdmin, anyAdminRejects].filter(
      (text) => within(banner as HTMLElement).queryByText(text, { exact: false }) !== null
    );
    expect(says).toEqual(anyAdminSays[state] === undefined ? [] : [anyAdminSays[state]]);
  }
);

// M13/P4/S1 (decision 857), M13/P5 (decision 915): every branch of a notebook's page draws the banner at the top of its
// column. Each row is a route and one of its branches: how the server is set, the address, what the branch shows first
// (a level-1 heading by its name, the main's alert, or its Loading…). Rows are added by the Step that lays out their
// page (05-P5 4.3.6), red before it does.

/** How a row sets the server before the address opens. */
type Setup = (server: ReturnType<typeof pageServer>) => void;
/** What a branch shows first: its h1 by its name, the main's alert, or its Loading…. */
type First = { h1: string } | "alert" | "loading";

const home = `/lab/notebooks/${notebookJSON.id}`;
const none: Setup = () => undefined;
const held: Setup = (server) => {
  server.nodesHeld = true;
};
const down: Setup = (server) => {
  server.nodesDown = true;
};
/** A changeset of the history's, logged as the history's rows find it: Ada's creation of Guide. */
const logged = changesetId(9101);
const logOne: Setup = (server) => {
  server.log("one", { id: logged, changes: [created(guide)] });
};
const historyDown: Setup = (server) => {
  server.historyDown = true;
};
/** The changeset's read never answered. */
const changesetHeld: Setup = (server) => {
  logOne(server);
  server.historyWaits.changeset = new Promise(() => undefined);
};

const columned: [string, string, Setup, First][] = [
  ["a page, read", pagePath(guide.id), none, { h1: "Guide" }],
  ["a page, its tree on its way", pagePath(guide.id), held, "loading"],
  ["a page, its tree not read", pagePath(guide.id), down, "alert"],
  ["a page not found", pagePath("0199a2b4-0000-7000-8000-0000000001ff"), none, { h1: "Page not found" }],
  ["a notebook's settings, read", `${home}/settings/general`, none, { h1: "General" }],
  ["a notebook's home, read", home, none, { h1: "Plans" }],
  ["a notebook's home, its tree on its way", home, held, "loading"],
  ["a notebook's home, its tree not read", home, down, "alert"],
  // M13/P5/S2: the history and a changeset's page, each branch of the changeset's (changeset-page.tsx).
  ["the history, read", `${home}/history`, logOne, { h1: "History" }],
  ["the history, its log not read", `${home}/history`, historyDown, "alert"],
  ["a changeset's page, read", `${home}/history/${logged}`, logOne, { h1: "Edit · Ada" }],
  ["a changeset's page, its address no id", `${home}/history/not-an-id`, none, { h1: "Page not found" }],
  ["a changeset's page, not found", `${home}/history/${changesetId(9102)}`, logOne, { h1: "Page not found" }],
  ["a changeset's page, on its way", `${home}/history/${logged}`, changesetHeld, "loading"],
  ["a changeset's page, not read", `${home}/history/${logged}`, historyDown, "alert"],
];

/** The banners saying the upgrade: statuses, as the settings' general page says the mode in a row of its own. */
const banners = () => screen.queryAllByRole("status").filter((status) => status.textContent?.includes(said.upgrading));

test.each(columned)(
  "%s: one banner, first in the branch's column, before what it shows and before any title",
  async (_, path, setup, first) => {
    const server = pageServer();
    server.notebook = { ...server.notebook, upgrade: upgradeOf("upgrading", bob.user_id) };
    setup(server);
    renderApp(path, server.app);

    let shown: HTMLElement;
    if (first === "loading") {
      // The banner first: the notebook's layout says Loading… itself before the notebooks are read, then goes.
      await waitFor(() => expect(banners()).toHaveLength(1));
      shown = within(screen.getByRole("main")).getByText("Loading…", { selector: "output" });
    } else if (first === "alert") {
      shown = await within(screen.getByRole("main")).findByRole("alert");
    } else {
      shown = await screen.findByRole("heading", { level: 1, name: first.h1 });
    }

    expect(banners()).toHaveLength(1);
    const banner = banners()[0] as HTMLElement;
    const column = banner.parentElement as HTMLElement;
    // The column's root: the banner its first child, the column centred (the main's own first child otherwise).
    expect(column.firstElementChild).toBe(banner);
    expect(column.classList.contains("mx-auto")).toBe(true);
    expect(column.contains(shown)).toBe(true);
    expect(banner.compareDocumentPosition(shown)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    for (const title of within(screen.getByRole("main")).queryAllByRole("heading", { level: 1 })) {
      expect(banner.compareDocumentPosition(title)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    }
  }
);

test("a notebook's settings index has the banner in the settings' column, as its first page has it, from the render that goes there (decisions 892, 898)", async () => {
  const server = pageServer();
  server.notebook = { ...server.notebook, upgrade: upgradeOf("upgrading", bob.user_id) };
  // Where the banner is each time the page changes: in the settings' column or not, and whether a page's title is there.
  const placed: { column: boolean; title: boolean }[] = [];
  const observer = new MutationObserver(() => {
    // Not by its text: the general page says it in its own row too.
    const banner = Array.from(document.querySelectorAll("output")).find((each) =>
      each.textContent?.includes(said.upgrading)
    );
    if (banner !== undefined) {
      placed.push({ column: banner.closest(".max-w-178") !== null, title: document.querySelector("main h1") !== null });
    }
  });
  observer.observe(document.body, { subtree: true, childList: true });
  try {
    renderApp(`/lab/notebooks/${notebookJSON.id}/settings`, server.app);
    await within(screen.getByRole("main")).findByRole("heading", { level: 1 });
  } finally {
    observer.disconnect();
  }
  // Seen before the first page's title, as the index goes to it (decision 892), and never out of the column.
  expect(placed[0]).toEqual({ column: true, title: false });
  expect(placed.every(({ column }) => column)).toBe(true);
  // One banner: the general page says the mode in its own row too, not as a status.
  expect(screen.getAllByRole("status").filter((status) => status.textContent?.includes(said.upgrading))).toHaveLength(
    1
  );
});

test("a version's page has the banner where it had it, above the notebook's outlet: not in the page's column", async () => {
  const server = pageServer();
  server.notebook = { ...server.notebook, upgrade: upgradeOf("upgrading", bob.user_id) };
  renderApp(`${pagePath(guide.id)}/versions/1`, server.app);

  const heading = await screen.findByRole("heading", { level: 1, name: "Version 1" });
  const banner = bannerOf(said.upgrading);
  expect(banner).not.toBeNull();
  expect(banner?.contains(heading)).toBe(false);
  expect(banner?.parentElement?.contains(heading)).toBe(true);
  expect(banner?.parentElement).not.toBe(heading.parentElement);
  expect(banner?.compareDocumentPosition(heading)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
});
