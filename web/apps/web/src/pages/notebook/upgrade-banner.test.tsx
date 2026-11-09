import { act, screen, within } from "@testing-library/react";
import { useEffect } from "react";
import { Outlet } from "react-router";
import { expect, test } from "vitest";

import { routes } from "../../app/routes";
import type { Notebook, NotebookRole } from "../../services/notebook.service";
import { useStore } from "../../stores/context";
import type { RootStore } from "../../stores/root.store";
import { notebookJSON, upgradeOf, workspaceJSON } from "../../test/fakes";
import { bob, guide, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The banner of a notebook's upgrade under way, above each of its pages
// (M11/P1 design 4.4): what it says by the upgrade's state, and what more
// it gives the notebook's admins (decision 246).

const said = {
  upgrading: "This notebook is being upgraded to an LLM Wiki: it is read-only for now.",
  reviewing: "This notebook's upgrade is submitted and waits for an admin's review: it is read-only for now.",
  rolling_back: "This notebook's upgrade is being undone: it is read-only for now.",
} as const;
const anyAdmin = "Any admin of the notebook can abandon this upgrade.";
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
  // Who may abandon it, while it may be abandoned: not once it rolls back.
  expect(within(banner as HTMLElement).queryByText(anyAdmin, { exact: false }) !== null).toBe(
    admin && state !== "rolling_back"
  );
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
