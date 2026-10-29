import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect } from "react";
import { Outlet } from "react-router";
import { expect, test } from "vitest";

import { routes } from "../../app/routes";
import { useStore } from "../../stores/context";
import type { RootStore } from "../../stores/root.store";
import { attachments, picker } from "../../test/attachments";
import { json, notebookJSON, problem, workspaceJSON } from "../../test/fakes";
import { assetNode, guide, install, linux, notes, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

const sourcePhoto = assetNode(70, "photo.png", linux);
const ordinaryPhoto = assetNode(70, "photo.png", notes);
const base = [guide, install, linux, notes];
const managementWrites = (server: ReturnType<typeof pageServer>) =>
  server.sent.filter((line) => /^(PATCH|MOVE|DELETE) /.test(line));

/** The real route and stores, with a way to read changes made by another tab. */
function renderAttachments(server: ReturnType<typeof pageServer>, parent = notes.id) {
  let store: RootStore;
  function CaptureStore() {
    const current = useStore();
    useEffect(() => {
      store = current;
    }, [current]);
    return <Outlet />;
  }
  return {
    ...renderApp(pagePath(parent), server.app, { routes: [{ Component: CaptureStore, children: routes }] }),
    refreshNotebook: () =>
      act(async () => {
        await store.notebooksOf(workspaceJSON)?.load();
      }),
    refreshTree: () =>
      act(async () => {
        await store.pagesOf(server.notebook)?.load();
      }),
    renameNode: (id: string, name: string) => store.pagesOf(server.notebook)!.rename(id, name),
  };
}

async function chooseAction(user: ReturnType<typeof userEvent.setup>, action: string) {
  const section = await attachments();
  await user.click(await within(section).findByRole("button", { name: "Actions for photo.png" }));
  await user.click(await screen.findByRole("menuitem", { name: action }));
}

test.each(["editor", "admin"] as const)("a source attachment's menu follows the current %s role", async (role) => {
  const user = userEvent.setup();
  const server = pageServer({ role, mode: "llm_wiki", sourceRootId: guide.id, nodes: [...base, sourcePhoto] });
  renderAttachments(server, linux.id);
  const section = await attachments();
  const link = await within(section).findByRole("link", { name: "photo.png (source) (opens in a new tab)" });
  expect(link.getAttribute("href")).toBe(`/api/v0/assets/${sourcePhoto.id}/content?sig=1`);
  await user.click(await within(section).findByRole("button", { name: "Actions for photo.png" }));

  expect(screen.getAllByRole("menuitem").map((item) => item.textContent)).toEqual([
    "Open (opens in a new tab)",
    "Download",
    "Copy embed",
    ...(role === "admin" ? ["Rename", "Move to…", "Delete"] : []),
  ]);
  expect(managementWrites(server)).toEqual([]);
  const download = screen.getByRole("menuitem", { name: "Download" });
  expect(download.getAttribute("href")).toBe(`/api/v0/assets/${sourcePhoto.id}/content?sig=1&download=1`);
});

test("a writer can copy, drag the embed and upload beside a source attachment", async () => {
  const user = userEvent.setup();
  const server = pageServer({
    role: "editor",
    mode: "llm_wiki",
    sourceRootId: guide.id,
    nodes: [...base, sourcePhoto],
  });
  renderAttachments(server, linux.id);
  const section = await attachments();
  const row = (await within(section).findByRole("button", { name: "Actions for photo.png" })).closest("li")!;
  const transfer = {
    data: new Map<string, string>(),
    effectAllowed: "move",
    setData(type: string, value: string) {
      this.data.set(type, value);
    },
  };
  fireEvent.dragStart(row, { dataTransfer: transfer });
  expect(transfer.data.get("text/plain")).toBe("![[photo.png]]");
  expect(transfer.effectAllowed).toBe("copy");
  expect(row.getAttribute("draggable")).toBe("true");
  await chooseAction(user, "Copy embed");
  await expect(navigator.clipboard.readText()).resolves.toBe("![[photo.png]]");
  fireEvent.change(picker(section), { target: { files: [new File(["new bytes"], "new.png")] } });
  await waitFor(() => expect(server.sent).toContain("UPLOAD new.png under Linux"));
  await waitFor(() =>
    expect(server.nodes.some((node) => node.name === "new.png" && node.parent_id === linux.id)).toBe(true)
  );
  expect(managementWrites(server)).toEqual([]);
});

test.each(
  (
    [
      ["admin", "llm_wiki", sourcePhoto, "Rename"],
      ["admin", "llm_wiki", sourcePhoto, "Move to…"],
      ["admin", "llm_wiki", sourcePhoto, "Delete"],
      ["editor", "llm_wiki", ordinaryPhoto, "Rename"],
      ["editor", "llm_wiki", ordinaryPhoto, "Move to…"],
      ["editor", "llm_wiki", ordinaryPhoto, "Delete"],
      ["editor", "standard", sourcePhoto, "Rename"],
      ["editor", "standard", sourcePhoto, "Move to…"],
      ["editor", "standard", sourcePhoto, "Delete"],
    ] as const
  ).map(([role, mode, photo, action]) => ({ role, mode, photo, action }))
)("$role can use $action on a $mode attachment under $photo.parent_id", async ({ role, mode, photo, action }) => {
  const user = userEvent.setup();
  const server = pageServer({
    role,
    mode,
    sourceRootId: mode === "standard" ? null : guide.id,
    nodes: [...base, photo],
  });
  renderAttachments(server, photo.parent_id!);
  await chooseAction(user, action);
  if (action === "Rename") {
    const dialog = await screen.findByRole("dialog", { name: "Rename photo.png" });
    const field = within(dialog).getByRole("textbox", { name: "Name" });
    await user.clear(field);
    await user.type(field, "renamed{Enter}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(managementWrites(server)).toEqual(["PATCH photo.png to renamed.png"]);
    expect(server.nodes.find((node) => node.id === photo.id)?.name).toBe("renamed.png");
  } else if (action === "Move to…") {
    const dialog = await screen.findByRole("dialog", { name: "Move photo.png" });
    await user.selectOptions(within(dialog).getByRole("combobox", { name: "Parent page" }), "The notebook's top level");
    await user.click(within(dialog).getByRole("button", { name: "Move" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(managementWrites(server)).toEqual(["MOVE photo.png under root, after last"]);
    expect(server.nodes.find((node) => node.id === photo.id)?.parent_id).toBeNull();
    expect(screen.queryByRole("alertdialog")).toBeNull();
  } else {
    const dialog = await screen.findByRole("alertdialog", { name: "Delete photo.png?" });
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    expect(managementWrites(server)).toEqual(["DELETE photo.png"]);
    expect(server.nodes.some((node) => node.id === photo.id)).toBe(false);
  }
});

test.each([
  ["role", "Rename"],
  ["role", "Delete"],
  ["mode", "Rename"],
  ["mode", "Delete"],
  ["tree", "Rename"],
  ["tree", "Delete"],
] as const)(
  "an already-open %s change blocks attachment %s without a request or false success",
  async (change, action) => {
    const user = userEvent.setup();
    const photo = change === "tree" ? ordinaryPhoto : sourcePhoto;
    const server = pageServer({
      role: change === "role" ? "admin" : "editor",
      mode: change === "mode" ? "standard" : "llm_wiki",
      sourceRootId: change === "mode" ? null : guide.id,
      nodes: [...base, photo],
    });
    const view = renderAttachments(server, photo.parent_id!);
    await chooseAction(user, action);
    const dialog = await screen.findByRole(action === "Delete" ? "alertdialog" : "dialog", {
      name: action === "Delete" ? "Delete photo.png?" : "Rename photo.png",
    });
    if (action === "Rename") {
      const field = within(dialog).getByRole("textbox", { name: "Name" });
      await user.clear(field);
      await user.type(field, "my draft");
    }
    if (change === "tree") {
      server.nodes = [...base, sourcePhoto];
      await view.refreshTree();
    } else {
      server.notebook = { ...server.notebook, role: "editor", mode: "llm_wiki", source_root_id: guide.id };
      await view.refreshNotebook();
    }
    await user.click(within(dialog).getByRole("button", { name: action === "Delete" ? "Delete" : "Save" }));
    expect(await within(dialog).findByRole("alert")).toHaveProperty(
      "textContent",
      "You do not have permission to do this."
    );
    expect(managementWrites(server)).toEqual([]);
    expect(document.contains(dialog)).toBe(true);
    if (action === "Rename") {
      expect((within(dialog).getByRole("textbox", { name: "Name" }) as HTMLInputElement).value).toBe("my draft");
      expect(within(dialog).queryByText("Saved.")).toBeNull();
    }
  }
);

test("an unchanged attachment rename cannot report success after the writer loses source permission", async () => {
  const user = userEvent.setup();
  const server = pageServer({ role: "admin", mode: "llm_wiki", sourceRootId: guide.id, nodes: [...base, sourcePhoto] });
  const view = renderAttachments(server, linux.id);
  await chooseAction(user, "Rename");
  const dialog = await screen.findByRole("dialog", { name: "Rename photo.png" });
  server.notebook = { ...server.notebook, role: "editor" };
  await view.refreshNotebook();
  await user.click(within(dialog).getByRole("button", { name: "Save" }));
  expect(await within(dialog).findByRole("alert")).toHaveProperty(
    "textContent",
    "You do not have permission to do this."
  );
  expect(document.contains(dialog)).toBe(true);
  expect(within(dialog).queryByText("Saved.")).toBeNull();
  expect(managementWrites(server)).toEqual([]);
});

test("a writer attachment move into sources asks before sending, keeps Cancel and sends once on confirmation", async () => {
  const user = userEvent.setup();
  const server = pageServer({
    role: "editor",
    mode: "llm_wiki",
    sourceRootId: guide.id,
    nodes: [...base, ordinaryPhoto],
  });
  renderAttachments(server);
  await chooseAction(user, "Move to…");
  const dialog = await screen.findByRole("dialog", { name: "Move photo.png" });
  const parent = within(dialog).getByRole("combobox", { name: "Parent page" });
  await user.selectOptions(parent, linux.id);
  const submit = within(dialog).getByRole("button", { name: "Move" });
  await user.click(submit);
  let confirmation = await screen.findByRole("alertdialog", { name: "Move into sources?" });
  expect(confirmation.textContent).toContain(
    "After moving, you cannot edit the content or manage these sources. An admin can move them out."
  );
  expect(managementWrites(server)).toEqual([]);
  await user.click(within(confirmation).getByRole("button", { name: "Cancel" }));
  expect(managementWrites(server)).toEqual([]);
  expect((parent as HTMLSelectElement).value).toBe(linux.id);
  expect(document.contains(dialog)).toBe(true);
  await user.click(submit);
  confirmation = await screen.findByRole("alertdialog", { name: "Move into sources?" });
  fireEvent.submit(submit.closest("form")!);
  expect(screen.getAllByRole("alertdialog")).toHaveLength(1);
  expect(managementWrites(server)).toEqual([]);
  await user.click(within(confirmation).getByRole("button", { name: "Move" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(managementWrites(server)).toEqual(["MOVE photo.png under Linux, after last"]);
  expect(server.nodes.find((node) => node.id === ordinaryPhoto.id)?.parent_id).toBe(linux.id);
});

test("an attachment staying under its own parent needs no source confirmation and sends nothing", async () => {
  const user = userEvent.setup();
  const server = pageServer({
    role: "editor",
    mode: "llm_wiki",
    sourceRootId: guide.id,
    nodes: [...base, ordinaryPhoto],
  });
  renderAttachments(server);
  await chooseAction(user, "Move to…");
  const dialog = await screen.findByRole("dialog", { name: "Move photo.png" });
  await user.click(within(dialog).getByRole("button", { name: "Move" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(screen.queryByRole("alertdialog")).toBeNull();
  expect(managementWrites(server)).toEqual([]);
});

test("an attachment move to its own parent cannot report success after source permission is lost", async () => {
  const user = userEvent.setup();
  const server = pageServer({ role: "admin", mode: "llm_wiki", sourceRootId: guide.id, nodes: [...base, sourcePhoto] });
  const view = renderAttachments(server, linux.id);
  await chooseAction(user, "Move to…");
  const dialog = await screen.findByRole("dialog", { name: "Move photo.png" });
  server.notebook = { ...server.notebook, role: "editor" };
  await view.refreshNotebook();
  await user.click(within(dialog).getByRole("button", { name: "Move" }));
  expect(await within(dialog).findByRole("alert")).toHaveProperty(
    "textContent",
    "You do not have permission to do this."
  );
  expect(document.contains(dialog)).toBe(true);
  expect(managementWrites(server)).toEqual([]);
  expect(screen.queryByRole("alertdialog")).toBeNull();
});

test("an admin moves an ordinary attachment into sources without confirmation", async () => {
  const user = userEvent.setup();
  const server = pageServer({
    role: "admin",
    mode: "llm_wiki",
    sourceRootId: guide.id,
    nodes: [...base, ordinaryPhoto],
  });
  renderAttachments(server);
  await chooseAction(user, "Move to…");
  const dialog = await screen.findByRole("dialog", { name: "Move photo.png" });
  await user.selectOptions(within(dialog).getByRole("combobox", { name: "Parent page" }), guide.id);
  await user.click(within(dialog).getByRole("button", { name: "Move" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(screen.queryByRole("alertdialog")).toBeNull();
  expect(managementWrites(server)).toEqual(["MOVE photo.png under Guide, after last"]);
});

test("a refused source attachment move returns the reason to the original form with its chosen parent", async () => {
  const user = userEvent.setup();
  const requests: { id: string; body: unknown }[] = [];
  const server = pageServer({
    role: "editor",
    mode: "llm_wiki",
    sourceRootId: guide.id,
    nodes: [...base, ordinaryPhoto],
    answers: {
      "POST /api/v0/nodes/*/move": async (request) => {
        requests.push({ id: new URL(request.url).pathname.split("/")[4]!, body: await request.json() });
        return problem(409, "page.title_taken");
      },
    },
  });
  renderAttachments(server);
  await chooseAction(user, "Move to…");
  const dialog = await screen.findByRole("dialog", { name: "Move photo.png" });
  const parent = within(dialog).getByRole("combobox", { name: "Parent page" });
  await user.selectOptions(parent, guide.id);
  await user.click(within(dialog).getByRole("button", { name: "Move" }));
  const confirmation = await screen.findByRole("alertdialog", { name: "Move into sources?" });
  expect(requests).toEqual([]);
  await user.click(within(confirmation).getByRole("button", { name: "Move" }));
  expect(await within(dialog).findByRole("alert")).toHaveProperty(
    "textContent",
    "A page under the same parent already has this title (titles differ in more than case)."
  );
  expect(screen.queryByRole("alertdialog")).toBeNull();
  expect((parent as HTMLSelectElement).value).toBe(guide.id);
  expect(requests).toEqual([{ id: ordinaryPhoto.id, body: { parent_id: guide.id } }]);
  expect(server.nodes.find((node) => node.id === ordinaryPhoto.id)?.parent_id).toBe(notes.id);
});

test.each(["Rename", "Delete", "Move to…"] as const)(
  "a queued attachment %s rechecks permission before its request and keeps the form when invalidated",
  async (entry) => {
    const user = userEvent.setup();
    let holding = false;
    let waiting = 0;
    let release!: () => void;
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    const photo = entry === "Move to…" ? ordinaryPhoto : sourcePhoto;
    const server = pageServer({
      role: entry === "Move to…" ? "editor" : "admin",
      mode: "llm_wiki",
      sourceRootId: guide.id,
      nodes: [...base, photo],
      answers: {
        [`GET /api/v0/notebooks/${notebookJSON.id}/nodes`]: async () => {
          server.sent.push("GET nodes");
          if (holding) {
            waiting += 1;
            await held;
          }
          return json({ data: server.nodes });
        },
      },
    });
    const view = renderAttachments(server, photo.parent_id!);
    await within(await attachments()).findByRole("button", { name: "Actions for photo.png" });
    holding = true;
    const blocker = view.renameNode(notes.id, "Notes");
    // This legitimate no-op write holds the queue through its required tree reread.
    await waitFor(() => expect(waiting).toBe(1));
    expect(managementWrites(server)).toEqual(["PATCH Notes to Notes"]);

    await chooseAction(user, entry);
    const dialog = await screen.findByRole(entry === "Delete" ? "alertdialog" : "dialog", {
      name: entry === "Delete" ? "Delete photo.png?" : entry === "Rename" ? "Rename photo.png" : "Move photo.png",
    });
    if (entry === "Rename") {
      const field = within(dialog).getByRole("textbox", { name: "Name" });
      await user.clear(field);
      await user.type(field, "queued draft");
    } else if (entry === "Move to…") {
      await user.selectOptions(within(dialog).getByRole("combobox", { name: "Parent page" }), guide.id);
    }
    const submit = within(dialog).getByRole("button", {
      name: entry === "Rename" ? "Save" : entry === "Delete" ? "Delete" : "Move",
    });
    await user.click(submit);
    if (entry === "Move to…") {
      const confirmation = await screen.findByRole("alertdialog", { name: "Move into sources?" });
      await user.click(within(confirmation).getByRole("button", { name: "Move" }));
      await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    }
    expect(submit.hasAttribute("disabled")).toBe(true);
    expect(managementWrites(server)).toEqual(["PATCH Notes to Notes"]);
    server.notebook =
      entry === "Move to…"
        ? { ...server.notebook, mode: "standard", source_root_id: null }
        : { ...server.notebook, role: "editor" };
    await view.refreshNotebook();
    await act(async () => {
      holding = false;
      release();
      await blocker;
    });
    await waitFor(() => expect(document.contains(dialog) && submit.hasAttribute("disabled")).toBe(false));

    expect(managementWrites(server)).toEqual(["PATCH Notes to Notes"]);
    expect(document.contains(dialog)).toBe(true);
    expect(server.nodes.find((node) => node.id === photo.id)).toEqual(photo);
    if (entry === "Move to…") {
      expect((within(dialog).getByRole("combobox", { name: "Parent page" }) as HTMLSelectElement).value).toBe(guide.id);
    } else {
      expect(within(dialog).getByRole("alert").textContent).toBe("You do not have permission to do this.");
      if (entry === "Rename") {
        expect((within(dialog).getByRole("textbox", { name: "Name" }) as HTMLInputElement).value).toBe("queued draft");
      }
    }
  }
);
