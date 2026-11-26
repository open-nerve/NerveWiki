import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { json, notebookJSON, signedInApp, workspaceJSON } from "../test/fakes";
import { notebookServer } from "../test/notebook-server";
import { pageServer } from "../test/page-server";
import { renderApp } from "../test/render";
import { arrived } from "./arrival";
import { sectionAt, workspaceSections } from "./settings-directory";

// The settings' directory in the left column (M13/P3/S1, decisions 810, 813, 816–818, 837, 843): Back, the title and a
// navigation for each group, the account's, the workspace's and the notebook's.

afterEach(() => vi.useRealTimers());

const home = `/lab/notebooks/${notebookJSON.id}`;
const acme = { ...workspaceJSON, id: "0199a2b4-0000-7000-8000-0000000000a1", slug: "acme", name: "Acme" };

/** The left column's root, [data-shell]. */
function column(): HTMLElement {
  const root = document.querySelector<HTMLElement>("[data-shell]");
  if (root === null) {
    throw new Error("no left column");
  }
  return root;
}

/** The names of the column's navigations, in their order. */
const groups = () =>
  within(column())
    .getAllByRole("navigation")
    .map((nav) => nav.getAttribute("aria-label"));

/** The group named name: its navigation. */
const group = (name: string) => within(column()).getByRole("navigation", { name });

/** A group's title: its first child, a line of text, no heading. */
const titleOf = (nav: HTMLElement) => nav.firstElementChild as HTMLElement;

/** The links of the group named name, each its text and whether it is the page shown. */
const linksOf = (name: string) =>
  within(group(name))
    .getAllByRole("link")
    .map((link) => [link.textContent, link.getAttribute("aria-current")]);

/** The directory's Back: a link of the left column, "Back" or "Back to …". */
const back = (name = /^Back(?: to .+)?$/) => within(column()).getByRole("link", { name });

/** A window narrower than md, its drawer the column (M13/P2/S3). */
function phone() {
  vi.stubGlobal("matchMedia", (query: string) => ({
    matches: query === "(width < 48rem)",
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
  }));
}

/** The menu of a phone's width that opens the drawer: the shown layout's. */
const navigation = () => screen.getByRole("button", { name: "Navigation" });

/** What the strip of a phone's width says beside its menu. */
const strip = (menu = "Navigation") => screen.getByRole("button", { name: menu }).parentElement?.textContent;

test("a workspace's settings: the column is their directory, the account's group then the workspace's, named with it in a bdi; no search, no switcher", async () => {
  renderApp("/lab/settings/general", notebookServer().app);
  await screen.findByRole("heading", { level: 1, name: "General" });

  expect(groups()).toEqual(["Settings", "Workspace settings"]);
  const title = titleOf(group("Workspace settings"));
  expect(title.tagName).toBe("P");
  // A space between the group's title and the name, outside the bdi (review A's N1).
  expect(title.textContent).toBe("Workspace Lab");
  expect(title.querySelector("bdi")?.textContent).toBe("Lab");
  expect(titleOf(group("Settings")).textContent).toBe("Personal");
  expect(linksOf("Settings")).toEqual([
    ["Profile", null],
    ["Appearance and language", null],
    ["Security", null],
    ["Access tokens", null],
  ]);
  expect(linksOf("Workspace settings")).toEqual([
    ["General", "page"],
    ["Members", null],
    ["Ownerless notebooks", null],
  ]);
  expect(screen.queryByRole("searchbox")).toBeNull();
  expect(screen.queryByRole("search")).toBeNull();
  expect(screen.queryByRole("button", { description: "Switch workspace" })).toBeNull();
  expect(screen.queryByRole("navigation", { name: "Lab" })).toBeNull();
  // No heading of the column's: the page's h1 is the only one.
  expect(within(column()).queryAllByRole("heading")).toEqual([]);
});

test("a member's directory has no ownerless notebooks", async () => {
  renderApp("/lab/settings/members", notebookServer({ workspaceRole: "member" }).app);
  await screen.findByRole("heading", { level: 1, name: "Members" });

  expect(linksOf("Workspace settings")).toEqual([
    ["General", null],
    ["Members", "page"],
  ]);
});

test("at a phone's width the strip says Settings", async () => {
  phone();
  renderApp("/lab/settings/general", notebookServer().app);
  await screen.findByRole("heading", { level: 1, name: "General" });

  expect(strip()).toBe("Settings");
});

test("a notebook's settings: three groups, the notebook's named as the list has it; the tree held out of sight", async () => {
  renderApp(`${home}/settings/general`, notebookServer().app);
  await screen.findByRole("heading", { level: 1, name: "General" });

  expect(groups()).toEqual(["Settings", "Workspace settings", "Notebook settings"]);
  expect(titleOf(group("Notebook settings")).textContent).toBe("Notebook Plans");
  // A name cut short has its whole as its title (M13 closeout, review A's M13CA-M1).
  expect(titleOf(group("Workspace settings")).querySelector("bdi")?.getAttribute("title")).toBe("Lab");
  expect(titleOf(group("Notebook settings")).querySelector("bdi")?.getAttribute("title")).toBe("Plans");
  expect(linksOf("Notebook settings")).toEqual([
    ["General", "page"],
    ["Members", null],
    ["Import, export and restore", null],
  ]);
  // Mounted and read as on its pages, in a container neither shown nor read out (decisions 814, 836).
  const tree = await screen.findByRole("navigation", { name: "Pages of Plans", hidden: true });
  expect(column().contains(tree)).toBe(true);
  expect(screen.queryByRole("navigation", { name: "Pages of Plans" })).toBeNull();
});

test("the settings of a notebook the list has not, its 404: the account's and the workspace's groups alone", async () => {
  renderApp("/lab/notebooks/0199a2b4-0000-7000-8000-0000000000cf/settings/general", notebookServer().app);
  await screen.findByRole("heading", { level: 1, name: "Page not found" });

  expect(groups()).toEqual(["Settings", "Workspace settings"]);
});

// The account's settings name the workspace they were entered from, else the one this device showed last, as the list
// has it (decisions 818, 837, 843).

test("the account's settings entered from Lab, another tab having shown Acme last: Lab's group, Back to Lab", async () => {
  const app = signedInApp({ "GET /api/v0/workspaces": () => json({ data: [workspaceJSON, acme] }) });
  const { router } = renderApp("/lab", app);
  await screen.findByRole("heading", { level: 1, name: "Lab" });
  app.preferences.setLastWorkspace("acme");

  await act(() => router.navigate("/settings/profile", { state: arrived }));

  await screen.findByRole("heading", { level: 1, name: "Profile" });
  await waitFor(() => expect(titleOf(group("Workspace settings")).textContent).toBe("Workspace Lab"));
  expect(back().textContent).toBe("Back to Lab");
});

test("the account's settings opened at their address: the workspace this device showed last, as the list has it", async () => {
  const app = signedInApp();
  app.preferences.setLastWorkspace("lab");
  renderApp("/settings/profile", app);
  await screen.findByRole("heading", { level: 1, name: "Profile" });

  await waitFor(() => expect(groups()).toEqual(["Settings", "Workspace settings"]));
  expect(titleOf(group("Workspace settings")).textContent).toBe("Workspace Lab");
  expect([back().textContent, back().getAttribute("href")]).toEqual(["Back to Lab", "/lab"]);
});

test("the account's settings opened at their address, the workspace shown last not in the list: no workspace's group, Back to /", async () => {
  let reads = 0;
  const app = signedInApp({
    "GET /api/v0/workspaces": () => {
      reads += 1;
      return json({ data: [workspaceJSON] });
    },
  });
  app.preferences.setLastWorkspace("zeta");
  renderApp("/settings/profile", app);
  await screen.findByRole("heading", { level: 1, name: "Profile" });

  await waitFor(() => expect(reads).toBe(1));
  await act(() => new Promise((resolve) => setTimeout(resolve, 20)));
  expect(groups()).toEqual(["Settings"]);
  expect([back().textContent, back().getAttribute("href")]).toEqual(["Back", "/"]);
});

test("the account's settings opened at their address, no workspace to look for: the list is not read", async () => {
  let reads = 0;
  const app = signedInApp({
    "GET /api/v0/workspaces": () => {
      reads += 1;
      return json({ data: [workspaceJSON] });
    },
  });
  renderApp("/settings/profile", app);
  await screen.findByRole("heading", { level: 1, name: "Profile" });
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));

  expect(reads).toBe(0);
  expect(groups()).toEqual(["Settings"]);
  expect([back().textContent, back().getAttribute("href")]).toEqual(["Back", "/"]);
});

test("the account's settings entered from a workspace, its list read: not read again, past SWR's dedupe", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  let reads = 0;
  const app = signedInApp({
    "GET /api/v0/workspaces": () => {
      reads += 1;
      return json({ data: [workspaceJSON] });
    },
  });
  const { router } = renderApp("/lab", app);
  await screen.findByRole("heading", { level: 1, name: "Lab" });
  expect(reads).toBe(1);
  await act(() => vi.advanceTimersByTimeAsync(2_000));

  await act(() => router.navigate("/settings/profile", { state: arrived }));
  await screen.findByRole("heading", { level: 1, name: "Profile" });
  await act(() => vi.advanceTimersByTimeAsync(50));

  expect(titleOf(group("Workspace settings")).textContent).toBe("Workspace Lab");
  expect(reads).toBe(1);
});

// Back (decisions 813, 834, 835): to the origin, named by its notebook or workspace as their lists have them.

test("entered from a notebook's page, Back goes there, named by the notebook as the list has it, renamed too", async () => {
  const user = userEvent.setup();
  renderApp(home, notebookServer().app);
  await screen.findByRole("heading", { level: 1, name: "Plans" });
  await user.click(await screen.findByRole("link", { name: "Settings of Plans" }));
  await screen.findByRole("heading", { level: 1, name: "General" });
  expect([back().textContent, back().getAttribute("href")]).toEqual(["Back to Plans", home]);
  // Cut short, the name it goes to has its whole as its title (M13 closeout, review A's M13CA-M1).
  expect(back().querySelector("[title]")?.getAttribute("title")).toBe("Plans");

  const name = await screen.findByLabelText<HTMLInputElement>("Name");
  await user.clear(name);
  await user.type(name, "Roadmap");
  await user.click(within(name.form as HTMLFormElement).getByRole("button", { name: "Save" }));

  await waitFor(() => expect([back().textContent, back().getAttribute("href")]).toEqual(["Back to Roadmap", home]));
  expect(titleOf(group("Notebook settings")).textContent).toBe("Notebook Roadmap");
  expect(back().querySelector("[title]")?.getAttribute("title")).toBe("Roadmap");
  expect(titleOf(group("Notebook settings")).querySelector("bdi")?.getAttribute("title")).toBe("Roadmap");
});

test("entered from a notebook's address the list read has not (gone), Back goes to its workspace's home", async () => {
  const gone = "/lab/notebooks/0199a2b4-0000-7000-8000-0000000000cf";
  const { router } = renderApp(gone, notebookServer().app);
  await screen.findByRole("heading", { level: 1, name: "Page not found" });

  await act(() => router.navigate("/settings/security", { state: arrived }));

  await screen.findByRole("heading", { level: 1, name: "Security" });
  await waitFor(() => expect([back().textContent, back().getAttribute("href")]).toEqual(["Back to Lab", "/lab"]));
});

test("entered from the page that creates a workspace, Back alone, to it", async () => {
  const { router } = renderApp("/create-workspace", signedInApp());
  await screen.findByRole("heading", { level: 1, name: "Create a workspace" });

  await act(() => router.navigate("/settings/tokens", { state: arrived }));

  await screen.findByRole("heading", { level: 1, name: "Access tokens" });
  await waitFor(() => expect([back().textContent, back().getAttribute("href")]).toEqual(["Back", "/create-workspace"]));
});

test("entered from a changeset's page by the notebook's cog, Back to it: its title takes the focus", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  const change = server.log("rework", { at: "2026-10-05T08:01:00Z", changes: [] });
  const changeset = `${home}/history/${change.id}`;
  const { router } = renderApp(changeset, server.app);
  await screen.findByRole("heading", { level: 1, name: "Edit · Ada" });

  await user.click(await screen.findByRole("link", { name: "Settings of Plans" }));
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "General" })));
  await user.click(back());

  expect(router.state.location.pathname).toBe(changeset);
  const title = await screen.findByRole("heading", { level: 1, name: "Edit · Ada" });
  await waitFor(() => expect(document.activeElement).toBe(title));
});

test("opened at their address, Back goes to the workspace's home, its title taking the focus", async () => {
  const user = userEvent.setup();
  const { router } = renderApp("/lab/settings/members", notebookServer().app);
  await screen.findByRole("heading", { level: 1, name: "Members" });

  await user.click(back());

  const lab = await screen.findByRole("heading", { level: 1, name: "Lab" });
  expect(router.state.location.pathname).toBe("/lab");
  await waitFor(() => expect(document.activeElement).toBe(lab));
});

// The focus (decision 816): a link of the same shell leaves it on itself, one across to the other shell and Back are
// arrived at.

test("a link of the same shell keeps the focus; one to the account's settings and Back are arrived at, their titles focused", async () => {
  const user = userEvent.setup();
  const { router } = renderApp(home, notebookServer().app);
  await screen.findByRole("heading", { level: 1, name: "Plans" });
  await user.click(await screen.findByRole("link", { name: "Settings of Plans" }));
  const general = await screen.findByRole("heading", { level: 1, name: "General" });
  await waitFor(() => expect(document.activeElement).toBe(general));

  const members = within(group("Notebook settings")).getByRole("link", { name: "Members" });
  await user.click(members);
  await waitFor(() => expect(router.state.location.pathname).toBe(`${home}/settings/members`));
  // The page's h1 mounts anew, the focus staying on the link.
  await waitFor(() => expect(screen.getByRole("heading", { level: 1, name: "Members" })).not.toBe(general));
  expect(document.activeElement).toBe(members);
  // The link itself, not the h1 of the page it opened, which has its name (M13/P3/S2, decision 815).
  const workspaceGeneral = within(group("Workspace settings")).getByRole("link", { name: "General" });
  await user.click(workspaceGeneral);
  await waitFor(() => expect(router.state.location.pathname).toBe("/lab/settings/general"));
  expect(document.activeElement).toBe(workspaceGeneral);

  await user.click(within(group("Settings")).getByRole("link", { name: "Profile" }));
  const profile = await screen.findByRole("heading", { level: 1, name: "Profile" });
  await waitFor(() => expect(document.activeElement).toBe(profile));
  const security = within(group("Settings")).getByRole("link", { name: "Security" });
  await user.click(security);
  await waitFor(() => expect(router.state.location.pathname).toBe("/settings/security"));
  expect(document.activeElement).toBe(security);

  await user.click(back());
  const plans = await screen.findByRole("heading", { level: 1, name: "Plans" });
  expect(router.state.location.pathname).toBe(home);
  await waitFor(() => expect(document.activeElement).toBe(plans));
  // The tree is back in the column, shown.
  expect(await screen.findByRole("navigation", { name: "Pages of Plans" })).toBeTruthy();
});

test("at a phone's width, a link of the directory's drawer closes it, the page's title taking the focus, arrived at after an arrival too", async () => {
  phone();
  const user = userEvent.setup();
  const { router } = renderApp(home, notebookServer().app);
  await screen.findByRole("heading", { level: 1, name: "Plans" });
  const menu = screen.getByRole("button", { name: "Navigation" });

  await user.click(menu);
  await user.click(await within(column()).findByRole("link", { name: "Settings of Plans" }));
  const general = await screen.findByRole("heading", { level: 1, name: "General" });
  await waitFor(() => expect(document.activeElement).toBe(general));
  expect(column().hasAttribute("inert")).toBe(true);

  await user.click(menu);
  await user.click(within(group("Notebook settings")).getByRole("link", { name: "Members" }));
  await waitFor(() => expect(router.state.location.pathname).toBe(`${home}/settings/members`));
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Members" })));
  expect(document.activeElement).not.toBe(general);
  expect(column().hasAttribute("inert")).toBe(true);

  await user.click(menu);
  await user.click(within(group("Notebook settings")).getByRole("link", { name: "Import, export and restore" }));
  await waitFor(() => expect(router.state.location.pathname).toBe(`${home}/settings/transfer`));
  await waitFor(() =>
    expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Import, export and restore" }))
  );
  expect(column().hasAttribute("inert")).toBe(true);
});

// Each settings layout mounts its page's h1 anew, by its key, for a page arrived at after an arrival in the same
// layout to take the focus (decision 815; review A's P3S2A-N1): the notebook's above, the workspace's and the
// account's here.
test("at a phone's width, the workspace's settings arrived at after an arrival: the next page's title, mounted anew, takes the focus", async () => {
  phone();
  const user = userEvent.setup();
  const { router } = renderApp("/lab", notebookServer().app);
  await screen.findByRole("heading", { level: 1, name: "Lab" });
  const menu = screen.getByRole("button", { name: "Navigation" });

  await user.click(menu);
  await user.click(await within(column()).findByRole("link", { name: "Workspace settings" }));
  const general = await screen.findByRole("heading", { level: 1, name: "General" });
  await waitFor(() => expect(document.activeElement).toBe(general));

  await user.click(menu);
  await user.click(within(group("Workspace settings")).getByRole("link", { name: "Members" }));
  await waitFor(() => expect(router.state.location.pathname).toBe("/lab/settings/members"));
  const members = await screen.findByRole("heading", { level: 1, name: "Members" });
  expect(members).not.toBe(general);
  await waitFor(() => expect(document.activeElement).toBe(members));
});

test("at a phone's width, the account's settings arrived at after an arrival: the next page's title, mounted anew, takes the focus", async () => {
  phone();
  const user = userEvent.setup();
  const { router } = renderApp("/lab", notebookServer().app);
  await screen.findByRole("heading", { level: 1, name: "Lab" });
  // The account's settings are another layout's: its menu another button, found each time.
  await user.click(navigation());
  await user.click(await within(column()).findByRole("link", { name: "Workspace settings" }));
  await screen.findByRole("heading", { level: 1, name: "General" });

  await user.click(navigation());
  await user.click(within(group("Settings")).getByRole("link", { name: "Profile" }));
  const profile = await screen.findByRole("heading", { level: 1, name: "Profile" });
  await waitFor(() => expect(document.activeElement).toBe(profile));

  await user.click(navigation());
  await user.click(within(group("Settings")).getByRole("link", { name: "Security" }));
  await waitFor(() => expect(router.state.location.pathname).toBe("/settings/security"));
  const security = await screen.findByRole("heading", { level: 1, name: "Security" });
  expect(security).not.toBe(profile);
  await waitFor(() => expect(document.activeElement).toBe(security));
});

test("in Chinese: the groups' titles, Back, and the strip", async () => {
  phone();
  const app = notebookServer().app;
  app.preferences.setLocale("zh-CN");
  const { unmount } = renderApp(`${home}/settings/general`, app);
  await screen.findByRole("heading", { level: 1, name: "常规" });

  expect(groups()).toEqual(["设置", "工作区设置", "笔记本设置"]);
  expect(titleOf(group("设置")).textContent).toBe("个人");
  expect(titleOf(group("工作区设置")).textContent).toBe("工作区 Lab");
  expect(titleOf(group("笔记本设置")).textContent).toBe("笔记本 Plans");
  expect(back(/^返回/).textContent).toBe("返回 Lab");
  expect(strip("导航")).toBe("设置");
  unmount();

  const alone = signedInApp();
  alone.preferences.setLocale("zh-CN");
  renderApp("/settings/appearance", alone);
  await screen.findByRole("heading", { level: 1, name: "外观与语言" });
  expect(back(/^返回/).textContent).toBe("返回");
});

// The settings' indexes pass the state they were given on to their first page (decision 816).

test.each([
  [arrived, true],
  [null, false],
])(
  "a workspace's settings opened at their index, with the state %j: the first page's title takes the focus, %s",
  async (state, focused) => {
    const { router } = renderApp([{ pathname: "/lab/settings", state }], notebookServer().app);

    const title = await screen.findByRole("heading", { level: 1, name: "General" });
    expect(router.state.location.pathname).toBe("/lab/settings/general");
    if (focused) {
      await waitFor(() => expect(document.activeElement).toBe(title));
    } else {
      await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
      expect(document.activeElement).not.toBe(title);
    }
  }
);

/** The label of the workspace's section that sectionAt finds at pathname, under Acme's settings. */
const at = (pathname: string) => sectionAt(workspaceSections, "/acme/settings", pathname)?.label;

test("a section is the page at its address as the router matches it: decoded, with trailing slashes or not, whatever the case; the settings' own address is none (decision 923)", () => {
  expect(at("/acme/settings/members")).toBe("workspaceSettings.members");
  expect(at("/acme/settings/members/")).toBe("workspaceSettings.members");
  expect(at("/acme/settings/Members")).toBe("workspaceSettings.members");
  expect(at("/acme/SETTINGS/members")).toBe("workspaceSettings.members");
  // Any number of trailing slashes, and the path decoded first, as the router has them (P5S1C-M1, P5S1A-N2).
  expect(at("/acme/settings/members//")).toBe("workspaceSettings.members");
  expect(at("/acme/settings/%6Dembers")).toBe("workspaceSettings.members");
  expect(at("/acme/settings/members%2F")).toBeUndefined();
  expect(at("/acme/settings/%E0%A4%A")).toBeUndefined();
  // The index's redirect, which has no page of its own.
  expect(at("/acme/settings")).toBeUndefined();
  expect(at("/acme/settings/")).toBeUndefined();
  expect(at("/acme/settings//")).toBeUndefined();
});
