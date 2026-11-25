import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { arrived } from "../../app/arrival";
import type { Notebook } from "../../services/notebook.service";
import type { Workspace } from "../../services/workspace.service";
import { json, notebookJSON, ownerlessJSON, problem, signedInApp, workspaceJSON, type Answer } from "../../test/fakes";
import { renderApp } from "../../test/render";

// A workspace's home: its notebooks, as cards (M3/P4 design 3.3).

const atlas: Notebook = {
  ...notebookJSON,
  id: "0199a2b4-0000-7000-8000-0000000000c2",
  name: "Atlas",
  workspace_access: "editor",
  role: "editor",
};

function labApp(list: Notebook[], role: Workspace["role"] = "admin", routes: Record<string, Answer> = {}) {
  return signedInApp({
    "GET /api/v0/workspaces": () => json({ data: [{ ...workspaceJSON, role }] }),
    "GET /api/v0/workspaces/lab/notebooks": () => json({ data: list }),
    ...routes,
  });
}

const main = async () =>
  (await screen.findByRole("heading", { level: 1, name: "Lab" })).closest("section") as HTMLElement;

test("the home shows each group's notebooks as cards leading to them", async () => {
  const user = userEvent.setup();
  const { router } = renderApp("/lab", labApp([atlas, notebookJSON]));
  const home = await main();

  const mine = await within(home).findByRole("list", { name: "My notebooks" });
  const team = within(home).getByRole("list", { name: "Team notebooks" });
  expect(
    within(mine)
      .getAllByRole("link")
      .map((link) => link.textContent)
  ).toEqual(["PlansPrivate · 1 member"]);
  expect(
    within(team)
      .getAllByRole("link")
      .map((link) => link.textContent)
  ).toEqual(["AtlasWorkspace can edit · 1 member"]);
  await user.click(within(team).getByRole("link"));

  expect(await screen.findByRole("heading", { level: 1, name: "Atlas" })).toBeTruthy();
  expect(router.state.location.pathname).toBe(`/lab/notebooks/${atlas.id}`);
});

// M12/P2/S1: the card leaves with the home; the notebook's home is arrived at (M5-polish 12.3).
test("a notebook's card leads to its home arrived at: its title takes the focus", async () => {
  const user = userEvent.setup();
  renderApp("/lab", labApp([atlas, notebookJSON]));
  const home = await main();

  const mine = await within(home).findByRole("list", { name: "My notebooks" });
  await user.click(within(mine).getByRole("link"));

  const heading = await screen.findByRole("heading", { level: 1, name: "Plans" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
});

test.each([
  ["admin", ["New notebook"]],
  ["member", ["New notebook"]],
  ["guest", []],
] as const)("with no notebook, the home says so, offering a workspace's %s %j", async (role, offered) => {
  renderApp("/lab", labApp([], role));
  const home = await main();

  expect(await within(home).findByText("No notebooks yet.")).toBeTruthy();
  expect(
    within(home)
      .queryAllByRole("button")
      .map((button) => button.textContent)
  ).toEqual(offered);
});

// M13/P5/S1 (decisions 918, 942): after the two groups, in no list, a dashed New notebook for who creates them.
test("with notebooks, the home offers New notebook after the groups, in no list: the left column's dialog, Escape's focus back on it", async () => {
  const user = userEvent.setup();
  renderApp("/lab", labApp([atlas, notebookJSON]));
  const home = await main();

  await within(home).findByRole("list", { name: "My notebooks" });
  const create = within(home).getByRole("button", { name: "New notebook" });
  expect(
    within(home)
      .queryAllByRole("button")
      .map((button) => button.textContent)
  ).toEqual(["New notebook"]);
  expect(create.closest("ul")).toBeNull();
  // A control's focus, the ring, as a Button's: transparent outline in forced colours (v0.1 design 13.2, item 31).
  expect(create.className.split(" ")).toEqual(
    expect.arrayContaining([
      "border-dashed",
      "focus-visible:ring-2",
      "focus-visible:ring-ring",
      "focus-visible:outline-hidden",
    ])
  );
  const team = within(home).getByRole("list", { name: "Team notebooks" });
  expect(team.compareDocumentPosition(create)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  await user.click(create);
  const dialog = await screen.findByRole("dialog", { name: "New notebook" });
  expect(within(dialog).getByLabelText("Name")).toBeTruthy();
  await user.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  await waitFor(() => expect(document.activeElement).toBe(create));
});

test("a guest's home with notebooks offers no New notebook", async () => {
  renderApp("/lab", labApp([atlas, notebookJSON], "guest"));
  const home = await main();

  await within(home).findByRole("list", { name: "My notebooks" });
  expect(within(home).queryAllByRole("button")).toEqual([]);
});

const wiki: Notebook = {
  ...notebookJSON,
  id: "0199a2b4-0000-7000-8000-0000000000c3",
  name: "Research notes for the spring",
  mode: "llm_wiki",
  workspace_access: "viewer",
  member_count: 2,
};
const big: Notebook = {
  ...notebookJSON,
  id: "0199a2b4-0000-7000-8000-0000000000c4",
  name: "Company",
  workspace_access: "editor",
  member_count: 1200,
};

/** The text of each card of the team's list, in its order. */
const teamCards = async (home: HTMLElement) =>
  within(await within(home).findByRole("list", { name: "Team notebooks" }))
    .getAllByRole("link")
    .map((link) => link.textContent);

test("a card: its icon read by no one, its name titled, an LLM Wiki's badge, its access and its members, counted as the locale writes numbers (decision 917)", async () => {
  renderApp("/lab", labApp([wiki, big]));
  const home = await main();

  expect(await teamCards(home)).toEqual([
    "Research notes for the springLLM WikiWorkspace can read · 2 members",
    "CompanyWorkspace can edit · 1,200 members",
  ]);
  const card = within(home).getByRole("link", { name: /^Research notes for the spring/ });
  const icon = card.querySelector("svg")?.parentElement;
  expect(icon?.getAttribute("aria-hidden")).toBe("true");
  expect(within(card).getByText("Research notes for the spring").getAttribute("title")).toBe(
    "Research notes for the spring"
  );
  expect(within(card).getByText("LLM Wiki")).toBeTruthy();
  expect(within(home).getByRole("link", { name: /^Company/ }).textContent).not.toContain("LLM Wiki");
});

test("in Chinese, a card's members: one, and more as the locale writes numbers", async () => {
  const app = labApp([notebookJSON, { ...big, member_count: 2 }]);
  app.preferences.setLocale("zh-CN");
  renderApp("/lab", app);
  const home = (await screen.findByRole("heading", { level: 1, name: "Lab" })).closest("section") as HTMLElement;

  const mine = await within(home).findByRole("list", { name: "我的笔记本" });
  expect(within(mine).getByRole("link").textContent).toBe("Plans私密 · 1 位成员");
  const team = within(home).getByRole("list", { name: "团队笔记本" });
  expect(within(team).getByRole("link").textContent).toBe("Company工作区成员可编辑 · 2 位成员");
});

// The home's New notebook goes with the empty state the creation ends: the
// creation still arrives at the new notebook.
test("a notebook created from the empty home opens on its home, arrived at", async () => {
  const user = userEvent.setup();
  const app = labApp([], "member", {
    "POST /api/v0/workspaces/lab/notebooks": () => json(notebookJSON, 201),
  });
  const { router } = renderApp("/lab", app);
  const home = await main();

  await user.click(await within(home).findByRole("button", { name: "New notebook" }));
  const dialog = await screen.findByRole("dialog", { name: "New notebook" });
  await user.type(within(dialog).getByLabelText("Name"), "Plans");
  await user.click(within(dialog).getByRole("button", { name: "Create" }));

  const heading = await screen.findByRole("heading", { level: 1, name: "Plans" });
  expect(router.state.location.pathname).toBe(`/lab/notebooks/${notebookJSON.id}`);
  await waitFor(() => expect(document.activeElement).toBe(heading));
});

// The home's creation ends its empty state, and the dialog with it, yet
// still arrives; once the home was left, it goes nowhere (v0.1 design
// 13.2, item 16; M3 Codex review R3).
test("a notebook created from the empty home answered after leaving it goes nowhere", async () => {
  const user = userEvent.setup();
  let release: (() => void) | undefined;
  const app = labApp([], "member", {
    "POST /api/v0/workspaces/lab/notebooks": async () => {
      await new Promise<void>((resolve) => (release = resolve));
      return json(notebookJSON, 201);
    },
  });
  const { router } = renderApp("/lab", app);

  await user.click(await within(await main()).findByRole("button", { name: "New notebook" }));
  const dialog = await screen.findByRole("dialog", { name: "New notebook" });
  await user.type(within(dialog).getByLabelText("Name"), "Plans");
  await user.click(within(dialog).getByRole("button", { name: "Create" }));
  await waitFor(() => expect(release).toBeDefined());
  await act(() => router.navigate("/settings/profile"));
  await screen.findByRole("heading", { level: 1, name: "Profile" });
  release?.();
  await act(() => new Promise((resolve) => setTimeout(resolve, 20)));

  expect(router.state.location.pathname).toBe("/settings/profile");
});

// The admins read how many notebooks have no admin (M3/P5 design 3.3).
test("an admin with ownerless notebooks reads how many, with a way to them", async () => {
  const user = userEvent.setup();
  const app = labApp([notebookJSON], "admin", {
    "GET /api/v0/workspaces/lab/ownerless-notebooks": () =>
      json({ data: [ownerlessJSON, { ...ownerlessJSON, id: "0199a2b4-0000-7000-8000-0000000000e2" }] }),
  });
  const { router } = renderApp("/lab", app);
  const home = await main();

  expect((await within(home).findByText(/^Notebooks without an admin/)).textContent).toBe(
    "Notebooks without an admin: 2. Review them"
  );
  await user.click(within(home).getByRole("link", { name: "Review them" }));

  expect(await screen.findByRole("heading", { level: 1, name: "Ownerless notebooks" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/lab/settings/ownerless");
});

// M13/P5/S1 (decision 924): Review them leaves the workspace's shell for the settings', arrived at.
test("the reminder's Review them arrives at the ownerless notebooks: their page's title takes the focus", async () => {
  const user = userEvent.setup();
  const app = labApp([notebookJSON], "admin", {
    "GET /api/v0/workspaces/lab/ownerless-notebooks": () => json({ data: [ownerlessJSON] }),
  });
  const { router } = renderApp("/lab", app);
  const home = await main();

  await user.click(await within(home).findByRole("link", { name: "Review them" }));

  const title = await screen.findByRole("heading", { level: 1, name: "Ownerless notebooks" });
  expect(router.state.location.state).toEqual(arrived);
  await waitFor(() => expect(document.activeElement).toBe(title));
});

test.each([
  ["an admin without ownerless notebooks", "admin" as const],
  ["a member", "member" as const],
  ["a guest", "guest" as const],
])("%s reads no reminder; only an admin asks", async (_, role) => {
  const asked: string[] = [];
  const app = labApp([notebookJSON], role, {
    "GET /api/v0/workspaces/lab/ownerless-notebooks": () => {
      asked.push("ownerless");
      return json({ data: [] });
    },
  });
  renderApp("/lab", app);
  const home = await main();
  await within(home).findByRole("list", { name: "My notebooks" });

  await waitFor(() => expect(asked).toEqual(role === "admin" ? ["ownerless"] : []));
  expect(within(home).queryByText(/^Notebooks without an admin/)).toBeNull();
});

test("made a member elsewhere, an admin's reminder is refused as forbidden, and reads the workspaces again", async () => {
  let role: Workspace["role"] = "admin";
  const asked: string[] = [];
  const app = signedInApp({
    "GET /api/v0/workspaces": () => {
      asked.push("workspaces");
      return json({ data: [{ ...workspaceJSON, role }] });
    },
    "GET /api/v0/workspaces/lab/notebooks": () => json({ data: [notebookJSON] }),
    "GET /api/v0/workspaces/lab/ownerless-notebooks": () => {
      asked.push("ownerless");
      return role === "admin" ? json({ data: [ownerlessJSON] }) : problem(403, "forbidden");
    },
  });
  const { router } = renderApp("/lab/settings/general", app);
  await screen.findByRole("navigation", { name: "Workspace settings" });

  role = "member";
  await act(() => router.navigate("/lab"));

  await waitFor(() => expect(asked).toEqual(["workspaces", "ownerless", "workspaces"]));
  expect(within(await main()).queryByText(/^Notebooks without an admin/)).toBeNull();
});

afterEach(() => vi.useRealTimers());

/** The reminder's text, with its link; "none" without it. */
const reminder = () => screen.queryByText(/^Notebooks without an admin/)?.textContent ?? "none";

test("the reminder read again shows what changed elsewhere; once none is left, it goes", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  let ownerless = [ownerlessJSON, { ...ownerlessJSON, id: "0199a2b4-0000-7000-8000-0000000000e2" }];
  const app = labApp([notebookJSON], "admin", {
    "GET /api/v0/workspaces/lab/ownerless-notebooks": () => json({ data: ownerless }),
  });
  const { router } = renderApp("/lab", app);
  /** comeBack goes to another page and back to the home after the reads' 2s: it reads the reminder again. */
  const comeBack = async () => {
    await act(() => router.navigate("/lab/settings/general"));
    await act(() => vi.advanceTimersByTimeAsync(2_000));
    await act(() => router.navigate("/lab"));
  };
  await waitFor(() => expect(reminder()).toBe("Notebooks without an admin: 2. Review them"));

  // Elsewhere, another admin takes one over, then the other.
  ownerless = ownerless.slice(1);
  await comeBack();
  await waitFor(() => expect(reminder()).toBe("Notebooks without an admin: 1. Review them"));
  ownerless = [];
  await comeBack();
  await waitFor(() => expect(reminder()).toBe("none"));
  expect(await within(await main()).findByRole("list", { name: "My notebooks" })).toBeTruthy();
});
