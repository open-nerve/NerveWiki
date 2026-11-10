import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { json, notebookJSON, problem } from "../../test/fakes";
import { ada, bob, notebookMember, notebookServer } from "../../test/notebook-server";
import { ada as adaPerson, assetNode, bob as bobPerson, guide, install, notes, pageNode } from "../../test/page-server";
import { renderApp } from "../../test/render";
import { deferred } from "../../test/health-server";
import { bobsUpgrade, upgradeJSON, upgradeServer } from "../../test/upgrade-server";

/** A page raw at the root, with a page under it, a page under that, and an attachment. */
const raw = pageNode(80, "raw");
const rawPage = pageNode(81, "Clipping", raw);
const rawDeep = pageNode(82, "Quote", rawPage);
const rawFile = assetNode(83, "scan.pdf", raw);

// A notebook's general settings (M3/P4 design 3.4).

const general = `/lab/notebooks/${notebookJSON.id}/settings/general`;
const plans = `/api/v0/notebooks/${notebookJSON.id}`;
const nameField = () => screen.findByLabelText<HTMLInputElement>("Name");
const nav = () => screen.findByRole("navigation", { name: "Lab" });

test("the settings of a notebook open on its general page, from its home", async () => {
  const user = userEvent.setup();
  const { router } = renderApp(`/lab/notebooks/${notebookJSON.id}`, notebookServer().app);

  await user.click(await screen.findByRole("link", { name: "Notebook settings" }));

  expect(await screen.findByRole("heading", { level: 2, name: "General" })).toBeTruthy();
  expect(router.state.location.pathname).toBe(general);
  expect(screen.getByRole("heading", { level: 1, name: "Plans settings" })).toBeTruthy();
  const settings = screen.getByRole("navigation", { name: "Notebook settings" });
  expect(within(settings).getByRole("link", { name: "General" }).getAttribute("aria-current")).toBe("page");
  expect(
    within(await nav())
      .getByRole("link", { name: "Plans" })
      .getAttribute("aria-current")
  ).toBe("page");
});

test("an admin renames the notebook; the left column shows the new name, in its place", async () => {
  const user = userEvent.setup();
  const server = notebookServer();
  renderApp(general, server.app);

  await user.clear(await nameField());
  await user.type(await nameField(), "  Roadmap ");
  await user.click(screen.getAllByRole("button", { name: "Save" })[0] as HTMLElement);

  expect(await screen.findByText("Saved.")).toBeTruthy();
  expect(server.sent).toEqual(['PATCH {"name":"Roadmap"}']);
  expect((await nameField()).value).toBe("Roadmap");
  expect(within(await nav()).getByRole("link", { name: "Roadmap" })).toBeTruthy();

  // The name as saved goes out no more.
  await user.click(screen.getAllByRole("button", { name: "Save" })[0] as HTMLElement);
  expect(await screen.findByText("Saved.")).toBeTruthy();
  expect(server.sent).toEqual(['PATCH {"name":"Roadmap"}']);
});

test("a name the local check or the server refuses shows under the field, by a title's rules", async () => {
  const user = userEvent.setup();
  const server = notebookServer({
    answers: {
      [`PATCH /api/v0/notebooks/${notebookJSON.id}`]: () =>
        problem(422, "validation_failed", { errors: [{ field: "name", code: "not_allowed", message: "…" }] }),
    },
  });
  renderApp(general, server.app);
  const save = async () => user.click(screen.getAllByRole("button", { name: "Save" })[0] as HTMLElement);

  await user.clear(await nameField());
  await save();
  expect(await screen.findByText("Required.")).toBeTruthy();
  await user.type(await nameField(), "nul.txt");
  await save();

  expect(
    await screen.findByText("Windows reserves this name (such as CON, NUL or COM1); choose another.")
  ).toBeTruthy();
  await waitFor(async () => expect(document.activeElement).toBe(await nameField()));
  expect(server.sent).toEqual(['PATCH {"name":"nul.txt"}']);
});

test("an admin opens the notebook to the workspace on Save; the left column moves it to the team's", async () => {
  const user = userEvent.setup();
  const server = notebookServer({ members: [notebookMember(ada, "admin")] });
  renderApp(general, server.app);
  await within(await nav()).findByRole("list", { name: "My notebooks" });

  // Moving through the options chooses them; nothing goes out until Save.
  await user.click(await screen.findByRole("radio", { name: "Workspace can read" }));
  await user.keyboard("{ArrowDown}");
  expect(screen.getByRole("radio", { name: "Workspace can edit" })).toHaveProperty("checked", true);
  expect(server.sent).toEqual([]);
  await user.click(screen.getAllByRole("button", { name: "Save" })[1] as HTMLElement);

  expect(await within(await nav()).findByRole("list", { name: "Team notebooks" })).toBeTruthy();
  expect(server.sent).toEqual(['PATCH {"workspace_access":"editor"}']);
  expect(within(await nav()).queryByRole("list", { name: "My notebooks" })).toBeNull();
});

test.each([
  ["deleted", undefined],
  ["already gone", () => problem(404, "notebook.not_found")],
])(
  "the admin deletes the notebook once its name is typed, and lands on the workspace's home, its heading focused: %s",
  async (_, gone) => {
    const user = userEvent.setup();
    const server = notebookServer(gone === undefined ? {} : { answers: { [`DELETE ${plans}`]: gone } });
    const { router } = renderApp(general, server.app);

    await user.click(await screen.findByRole("button", { name: "Delete notebook" }));
    const dialog = await screen.findByRole("alertdialog", { name: "Delete Plans?" });
    const confirm = within(dialog).getByRole("button", { name: "Delete" });
    expect(confirm).toHaveProperty("disabled", true);
    await user.type(within(dialog).getByLabelText("Type Plans to confirm"), "Plans{Enter}");

    const home = await screen.findByRole("heading", { level: 1, name: "Lab" });
    expect(router.state.location.pathname).toBe("/lab");
    expect(server.sent).toEqual(["DELETE"]);
    await waitFor(() => expect(document.activeElement).toBe(home));
    expect(screen.queryByRole("heading", { name: "Page not found" })).toBeNull();
    expect(within(await nav()).queryByRole("link", { name: "Plans" })).toBeNull();
  }
);

test("a deletion refused stays in the dialog with why, and the notebook stays", async () => {
  const user = userEvent.setup();
  const server = notebookServer({
    answers: { [`DELETE /api/v0/notebooks/${notebookJSON.id}`]: () => problem(403, "forbidden") },
  });
  renderApp(general, server.app);

  await user.click(await screen.findByRole("button", { name: "Delete notebook" }));
  const dialog = await screen.findByRole("alertdialog");
  await user.type(within(dialog).getByLabelText("Type Plans to confirm"), "Plans{Enter}");

  expect((await within(dialog).findByRole("alert")).textContent).toBe("You do not have permission to do this.");
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  expect(within(await nav()).getByRole("link", { name: "Plans" })).toBeTruthy();
  expect(screen.getByRole("heading", { level: 2, name: "General" })).toBeTruthy();
});

test.each(["editor", "reader"] as const)(
  "its %s sees the name and the access, and cannot change them",
  async (role) => {
    const members = [notebookMember(ada, role), notebookMember(bob, "admin")];
    renderApp(general, notebookServer({ access: "viewer", members }).app);

    expect(await screen.findByText("Only the notebook's admins can change its name.")).toBeTruthy();
    expect(screen.getByText("Workspace can read")).toBeTruthy();
    expect(screen.getByText("Only the notebook's admins can change its workspace access.")).toBeTruthy();
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.queryByRole("radio")).toBeNull();
    expect(screen.queryByRole("button", { name: "Save" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Delete notebook" })).toBeNull();
  }
);

/** The general page's two Save buttons: the name's, then the access's. */
const saves = () => screen.getAllByRole("button", { name: "Save" }) as [HTMLElement, HTMLElement];

test("a name or an access chosen while its save is out keeps the choice, not marked saved", async () => {
  const user = userEvent.setup();
  let release: (() => void) | undefined;
  const server = notebookServer({
    members: [notebookMember(ada, "admin")],
    answers: {
      [`PATCH ${plans}`]: async (request) => {
        const body = (await request.clone().json()) as object;
        await new Promise<void>((resolve) => (release = resolve));
        return json({ ...notebookJSON, ...body });
      },
    },
  });
  renderApp(general, server.app);

  await user.clear(await nameField());
  await user.type(await nameField(), "Roadmap");
  await user.click(saves()[0]);
  await waitFor(() => expect(server.sent).toHaveLength(1));
  await user.type(await nameField(), " 2027");
  release?.();
  await waitFor(() => expect(saves()[0]).toHaveProperty("disabled", false));
  expect((await nameField()).value).toBe("Roadmap 2027");
  expect(screen.queryByText("Saved.")).toBeNull();

  await user.click(screen.getByRole("radio", { name: "Workspace can read" }));
  await user.click(saves()[1]);
  await waitFor(() => expect(server.sent).toHaveLength(2));
  await user.click(screen.getByRole("radio", { name: "Workspace can edit" }));
  release?.();
  await waitFor(() => expect(saves()[1]).toHaveProperty("disabled", false));
  expect(screen.getByRole("radio", { name: "Workspace can edit" })).toHaveProperty("checked", true);
  expect(screen.queryByText("Saved.")).toBeNull();
});

afterEach(() => vi.useRealTimers());

// SWR reads the notebooks again as the tab regains the focus: untouched
// forms follow a change made elsewhere, and Save sends nothing back over it.
test("untouched forms follow another admin's changes; Save sends nothing back over them", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const server = notebookServer({ members: [notebookMember(ada, "admin"), notebookMember(bob, "admin")] });
  renderApp(general, server.app);
  expect((await nameField()).value).toBe("Plans");

  server.plans = { ...server.plans, name: "Roadmap", workspace_access: "editor" };
  await act(() => vi.advanceTimersByTimeAsync(6_000));
  act(() => void window.dispatchEvent(new Event("focus")));

  await waitFor(async () => expect((await nameField()).value).toBe("Roadmap"));
  expect(screen.getByRole("radio", { name: "Workspace can edit" })).toHaveProperty("checked", true);
  await user.click(saves()[0]);
  await user.click(saves()[1]);
  expect(await screen.findAllByText("Saved.")).toHaveLength(2);
  expect(server.sent).toEqual([]);
});

test("an admin downgrades a wiki once and focuses the Mode heading", async () => {
  const user = userEvent.setup();
  const server = notebookServer({ mode: "llm_wiki" });
  renderApp(general, server.app);

  const heading = await screen.findByRole("heading", { level: 2, name: "Mode" });
  await user.click(screen.getByRole("button", { name: "Downgrade to a standard notebook…" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Downgrade “Plans” to a standard notebook?" });
  expect(within(dialog).queryByRole("textbox")).toBeNull();
  await user.dblClick(within(dialog).getByRole("button", { name: "Downgrade" }));

  expect(await screen.findByText("Standard notebook")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Downgrade to a standard notebook…" })).toBeNull();
  expect(screen.queryByRole("alertdialog")).toBeNull();
  expect(server.sent).toEqual(["downgrade"]);
  expect([server.plans.mode, server.plans.source_root_id]).toEqual(["standard", null]);
  await waitFor(() => expect(document.activeElement).toBe(heading));
});

test.each([
  ["llm_wiki", "admin", true],
  ["llm_wiki", "editor", false],
  ["llm_wiki", "reader", false],
  ["standard", "admin", false],
] as const)("a %s notebook's %s sees its mode, with downgrade available: %s", async (mode, role, downgrades) => {
  const server = notebookServer({ mode, members: [notebookMember(ada, role), notebookMember(bob, "admin")] });
  renderApp(general, server.app);

  const heading = await screen.findByRole("heading", { level: 2, name: "Mode" });
  const section = within(heading.parentElement as HTMLElement);
  expect(section.getByText(mode === "llm_wiki" ? "LLM Wiki" : "Standard notebook")).toBeTruthy();
  expect(
    section.getByText(
      mode === "llm_wiki"
        ? "Agents keep its pages from the sources in its source area, by the rules on its page AGENTS."
        : "Its members write and organize its pages themselves."
    )
  ).toBeTruthy();
  expect(section.queryByRole("button", { name: "Downgrade to a standard notebook…" }) !== null).toBe(downgrades);
  expect(server.sent).toEqual([]);
});

test("a downgrade forbidden after losing admin keeps its dialog, then focuses Mode when cancelled", async () => {
  const user = userEvent.setup();
  const server = notebookServer({
    mode: "llm_wiki",
    answers: {
      [`POST ${plans}/downgrade`]: () => {
        server.members = [notebookMember(ada, "editor"), notebookMember(bob, "admin")];
        return problem(403, "forbidden");
      },
    },
  });
  renderApp(general, server.app);
  const trigger = await screen.findByRole("button", { name: "Downgrade to a standard notebook…" });
  const heading = screen.getByRole("heading", { level: 2, name: "Mode" });
  const reads = server.notebookReads;

  await user.click(trigger);
  const dialog = await screen.findByRole("alertdialog");
  await user.click(within(dialog).getByRole("button", { name: "Downgrade" }));

  expect((await within(dialog).findByRole("alert")).textContent).toBe("Only the notebook's admins can downgrade it.");
  expect(trigger.isConnected).toBe(false);
  expect(dialog.isConnected).toBe(true);
  expect(screen.getByRole("alertdialog")).toBe(dialog);
  expect(server.notebookReads).toBe(reads + 1);
  expect(server.plans.mode).toBe("llm_wiki");
  expect(server.sent).toEqual(["downgrade"]);
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));

  await waitFor(() => expect(document.activeElement).toBe(heading));
  expect(screen.queryByRole("alertdialog")).toBeNull();
});

test("a downgrade busy with writing keeps its dialog and does not reread the notebooks", async () => {
  const user = userEvent.setup();
  const server = notebookServer({
    mode: "llm_wiki",
    answers: { [`POST ${plans}/downgrade`]: () => problem(409, "transfer.busy") },
  });
  renderApp(general, server.app);
  const trigger = await screen.findByRole("button", { name: "Downgrade to a standard notebook…" });
  const reads = server.notebookReads;

  await user.click(trigger);
  const dialog = await screen.findByRole("alertdialog");
  await user.click(within(dialog).getByRole("button", { name: "Downgrade" }));

  expect((await within(dialog).findByRole("alert")).textContent).toBe(
    "An import, a restore or an undo is under way in this notebook: wait for it to end, then try again."
  );
  expect(server.notebookReads).toBe(reads);
  expect(server.plans.mode).toBe("llm_wiki");
  expect(server.sent).toEqual(["downgrade"]);
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(document.activeElement).toBe(trigger));
});

test("a downgrade already done elsewhere closes its dialog and focuses Mode", async () => {
  const user = userEvent.setup();
  const server = notebookServer({
    mode: "llm_wiki",
    answers: {
      [`POST ${plans}/downgrade`]: () => {
        server.plans = { ...server.plans, mode: "standard", source_root_id: null };
        return problem(409, "notebook.not_llm_wiki");
      },
    },
  });
  renderApp(general, server.app);
  await user.click(await screen.findByRole("button", { name: "Downgrade to a standard notebook…" }));
  const dialog = await screen.findByRole("alertdialog");
  await user.click(within(dialog).getByRole("button", { name: "Downgrade" }));

  expect(await screen.findByText("Standard notebook")).toBeTruthy();
  expect(screen.queryByRole("alertdialog")).toBeNull();
  expect(screen.queryByRole("button", { name: "Downgrade to a standard notebook…" })).toBeNull();
  expect(server.sent).toEqual(["downgrade"]);
  const heading = screen.getByRole("heading", { level: 2, name: "Mode" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
});

test("cancelling a downgrade returns to its trigger without sending it", async () => {
  const user = userEvent.setup();
  const server = notebookServer({ mode: "llm_wiki" });
  renderApp(general, server.app);
  const trigger = await screen.findByRole("button", { name: "Downgrade to a standard notebook…" });

  await user.click(trigger);
  const dialog = await screen.findByRole("alertdialog");
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));

  await waitFor(() => expect(document.activeElement).toBe(trigger));
  expect(server.sent).toEqual([]);
  expect(server.plans.mode).toBe("llm_wiki");
});

// An upgrade's start, from the notebook's mode (M11/P1 design 4.4).

const startLabel = "Upgrade to an LLM Wiki…";
const startTitle = "Upgrade “Plans” to an LLM Wiki?";
const modeOf = async (name = "Mode") =>
  within((await screen.findByRole("heading", { level: 2, name })).parentElement as HTMLElement);

test.each([
  ["standard", "admin", null, true],
  ["llm_wiki", "admin", null, false],
  ["standard", "editor", null, false],
  ["standard", "reader", null, false],
  ["standard", "admin", "upgrading", false],
] as const)("a %s notebook's %s, its upgrade %s, is offered an upgrade: %s", async (mode, role, state, offered) => {
  const server = upgradeServer({
    mode,
    role,
    sourceRootId: mode === "llm_wiki" ? guide.id : null,
    upgrade: state === null ? undefined : bobsUpgrade({ state }),
  });
  renderApp(general, server.app);

  const section = await modeOf();
  await within(await screen.findByRole("navigation", { name: "Pages of Plans" })).findByRole("link", {
    name: "Notes",
  });
  expect(section.queryByRole("button", { name: startLabel }) !== null).toBe(offered);
  expect(server.asked).toEqual([]);
});

test.each([
  ["admin", "upgrading", true],
  ["admin", "rolling_back", true],
  ["editor", "upgrading", false],
  ["reader", "reviewing", false],
] as const)(
  "a notebook being upgraded says where its upgrade is to its %s, %s; its page offered: %s",
  async (role, state, linked) => {
    const server = upgradeServer({ role, upgrade: bobsUpgrade({ state }) });
    renderApp(general, server.app);

    const section = await modeOf();
    const texts = {
      upgrading: "This notebook is being upgraded to an LLM Wiki: it is read-only for now.",
      rolling_back: "This notebook's upgrade is being undone: it is read-only for now.",
      reviewing: "This notebook's upgrade is submitted and waits for an admin's review: it is read-only for now.",
    };
    expect(section.getByText(texts[state], { exact: false })).toBeTruthy();
    const link = section.queryByRole("link", { name: "View the upgrade" });
    expect(link?.getAttribute("href") ?? null).toBe(linked ? `/lab/notebooks/${notebookJSON.id}/upgrade` : null);
  }
);

test.each([
  ["a page raw at the root with nodes under it, at any depth", [raw, rawPage, rawDeep, rawFile], 3],
  ["a page raw at the root with none under it", [raw], 0],
  // The server finds raw by its title's key (start_upgrade.go), as the root's titles are told apart.
  ["a page Raw at the root with nodes under it", [{ ...raw, name: "Raw" }, rawPage, rawDeep], 2],
  [
    "a page raw elsewhere",
    [
      { ...raw, parent_id: guide.id },
      { ...rawPage, parent_id: raw.id },
    ],
    0,
  ],
  ["no page raw", [], 0],
] as const)("the start's dialog says what becomes of %s", async (_, more, count) => {
  const user = userEvent.setup();
  const server = upgradeServer({ nodes: [guide, install, ...more] });
  renderApp(general, server.app);

  await user.click(await (await modeOf()).findByRole("button", { name: startLabel }));
  const dialog = await screen.findByRole("alertdialog", { name: startTitle });

  expect(
    within(dialog).getByText(
      "Until the upgrade ends, approved or abandoned, the notebook is read-only on the web: only your agent writes it, over MCP. The upgrade writes the pages AGENTS and raw at the root where the notebook lacks them; the contents of its pages are not changed."
    )
  ).toBeTruthy();
  const said = within(dialog).queryByText(/^The page raw at the root becomes the source area/u);
  expect(said?.textContent ?? null).toBe(
    count === 0
      ? null
      : `The page raw at the root becomes the source area (${count} nodes under it now): once approved, everything under it is a read-only source. Your agent can move out what is not a source.`
  );
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  expect(server.asked).toEqual([]);
});

test("an admin starts the upgrade, in the page's language, and lands on its page, its title focused", async () => {
  const user = userEvent.setup();
  const server = upgradeServer();
  server.app.preferences.setLocale("zh-CN");
  const { router } = renderApp(general, server.app);

  await user.click(await (await modeOf("模式")).findByRole("button", { name: "升级为 LLM Wiki…" }));
  const dialog = await screen.findByRole("alertdialog", { name: "把「Plans」升级为 LLM Wiki？" });
  await user.dblClick(within(dialog).getByRole("button", { name: "开始升级" }));

  const heading = await screen.findByRole("heading", { level: 1, name: "升级为 LLM Wiki" });
  expect(router.state.location.pathname).toBe(`/lab/notebooks/${notebookJSON.id}/upgrade`);
  await waitFor(() => expect(document.activeElement).toBe(heading));
  expect(server.asked.filter((line) => line.startsWith("START"))).toEqual(["START zh-CN"]);
  // What the start answered shows at once; the page reads it anew a frame later (SWR, with data cached).
  await screen.findByText("Ada 于", { exact: false });
  await waitFor(() => expect(server.asked).toContain("GET upgrade"));
  expect(screen.queryByRole("alertdialog")).toBeNull();
});

test("a start refused as pages being edited names them and who edits each, the account itself too", async () => {
  const user = userEvent.setup();
  const server = upgradeServer();
  server.hold(install.id, bobPerson);
  server.hold(notes.id, adaPerson);
  renderApp(general, server.app);

  await user.click(await (await modeOf()).findByRole("button", { name: startLabel }));
  const dialog = await screen.findByRole("alertdialog", { name: startTitle });
  await user.click(within(dialog).getByRole("button", { name: "Start the upgrade" }));

  const alert = await within(dialog).findByRole("alert");
  expect([...alert.querySelectorAll("p, li")].map((each) => each.textContent)).toEqual([
    "These pages are being edited, so the upgrade cannot start. Try again once their editors leave; a notebook admin can also release a lock.",
    "Bob is editing “Install”.",
    "You are editing “Notes”.",
  ]);
  expect(server.upgrade).toBeUndefined();
});

test("a start refused as busy with writing says so in its dialog and reads the notebooks no more", async () => {
  const user = userEvent.setup();
  let reads = 0;
  const server: ReturnType<typeof upgradeServer> = upgradeServer({
    answers: {
      "GET /api/v0/workspaces/lab/notebooks": () => {
        reads++;
        return json({ data: [server.notebook] });
      },
    },
  });
  server.busy = true;
  renderApp(general, server.app);
  await user.click(await (await modeOf()).findByRole("button", { name: startLabel }));
  const dialog = await screen.findByRole("alertdialog", { name: startTitle });
  const before = reads;

  await user.click(within(dialog).getByRole("button", { name: "Start the upgrade" }));

  expect((await within(dialog).findByRole("alert")).textContent).toBe(
    "An import, a restore or an undo is under way in this notebook: wait for it to end, then try again."
  );
  expect(reads).toBe(before);
});

test("the start is offered once the tree is read, which tells what it makes of raw (decision 250)", async () => {
  const tree = deferred<void>();
  const server: ReturnType<typeof upgradeServer> = upgradeServer({
    nodes: [guide, raw, rawPage],
    answers: {
      [`GET ${plans}/nodes`]: async () => {
        await tree.promise;
        return json({ data: server.nodes });
      },
    },
  });
  renderApp(general, server.app);
  const section = await modeOf();
  expect(section.getByText("Standard notebook")).toBeTruthy();
  expect(section.queryByRole("button", { name: startLabel })).toBeNull();

  tree.resolve();

  expect(await section.findByRole("button", { name: startLabel })).toBeTruthy();
});

test("a start answered once the settings were left takes no one anywhere: the notebook's home stays, the banner says the upgrade", async () => {
  const user = userEvent.setup();
  const started = deferred<void>();
  const server: ReturnType<typeof upgradeServer> = upgradeServer({
    answers: {
      [`POST ${plans}/upgrade`]: async () => {
        await started.promise;
        server.withUpgrade(upgradeJSON());
        return json(server.upgrade);
      },
    },
  });
  const { router } = renderApp(general, server.app);
  await user.click(await (await modeOf()).findByRole("button", { name: startLabel }));
  await user.click(
    within(await screen.findByRole("alertdialog", { name: startTitle })).getByRole("button", {
      name: "Start the upgrade",
    })
  );
  const home = `/lab/notebooks/${notebookJSON.id}`;
  await act(() => router.navigate(home));
  await screen.findByRole("heading", { level: 1, name: "Plans" });

  started.resolve();

  expect(
    await screen.findByText("This notebook is being upgraded to an LLM Wiki: it is read-only for now.")
  ).toBeTruthy();
  expect(router.state.location.pathname).toBe(home);
});

test("an admin told no upgrade is under way who starts one at once lands on what the start answered, not on what was read, while it is read anew", async () => {
  const user = userEvent.setup();
  const read = deferred<void>();
  const server: ReturnType<typeof upgradeServer> = upgradeServer({
    answers: {
      [`GET ${plans}/upgrade`]: async () => {
        server.asked.push("GET upgrade");
        if (server.upgrade === undefined) {
          return problem(404, "wiki.upgrade_not_found");
        }
        await read.promise;
        return json(server.upgrade);
      },
    },
  });
  const { router } = renderApp(`/lab/notebooks/${notebookJSON.id}/upgrade`, server.app);
  expect(await screen.findByText("This notebook has no upgrade under way.")).toBeTruthy();
  await act(() => router.navigate(general));

  await user.click(await (await modeOf()).findByRole("button", { name: startLabel }));
  await user.click(
    within(await screen.findByRole("alertdialog", { name: startTitle })).getByRole("button", {
      name: "Start the upgrade",
    })
  );

  const state = (await screen.findByRole("heading", { level: 2, name: "Where it is" })).parentElement as HTMLElement;
  expect(within(state).getByText("Started by Ada on Oct 5, 2026, 8:00 AM.")).toBeTruthy();
  expect(screen.queryByText("This notebook has no upgrade under way.")).toBeNull();
  await waitFor(() => expect(server.asked).toEqual(["GET upgrade", "START en", "GET upgrade"]));
  read.resolve();
  await act(() => read.promise);
  expect(screen.getByRole("heading", { level: 2, name: "Where it is" })).toBeTruthy();
  expect(server.asked).toEqual(["GET upgrade", "START en", "GET upgrade"]);
});

test("cancelling the start returns to its trigger; another admin's start first keeps the dialog, then focuses Mode", async () => {
  const user = userEvent.setup();
  const server: ReturnType<typeof upgradeServer> = upgradeServer({
    answers: {
      [`POST ${plans}/upgrade`]: () => {
        server.withUpgrade(bobsUpgrade());
        return problem(409, "notebook.upgrading");
      },
    },
  });
  renderApp(general, server.app);
  const trigger = await (await modeOf()).findByRole("button", { name: startLabel });

  await user.click(trigger);
  await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(document.activeElement).toBe(trigger));

  await user.click(trigger);
  const dialog = await screen.findByRole("alertdialog", { name: startTitle });
  await user.click(within(dialog).getByRole("button", { name: "Start the upgrade" }));
  expect((await within(dialog).findByRole("alert")).textContent).toBe(
    "This notebook is being upgraded to an LLM Wiki: it is read-only until the upgrade ends."
  );
  await waitFor(() => expect(trigger.isConnected).toBe(false));
  expect(screen.getByRole("alertdialog")).toBe(dialog);
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));

  const heading = screen.getByRole("heading", { level: 2, name: "Mode" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
});
