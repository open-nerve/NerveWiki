import { screen, waitFor, within } from "@testing-library/react";
import { expect, test } from "vitest";

import { json, notebookJSON, problem } from "../../test/fakes";
import { guide, install, notes } from "../../test/page-server";
import { renderApp } from "../../test/render";
import { bobsUpgrade, seededAgents, seededRaw, upgradeServer } from "../../test/upgrade-server";

// The page of a notebook's upgrade under way (M11/P1 design 4.4): its
// admins' alone (v0.1 design 13.2, item 18).

const upgradePath = `/lab/notebooks/${notebookJSON.id}/upgrade`;
const plans = `/api/v0/notebooks/${notebookJSON.id}`;
const adminsOnly = "Only the notebook's admins can see its upgrade.";
const pageOf = (id: string) => `/lab/notebooks/${notebookJSON.id}/pages/${id}`;

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
