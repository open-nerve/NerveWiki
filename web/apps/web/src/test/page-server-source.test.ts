import { describe, expect, test } from "vitest";

import { PageService } from "../services/page.service";
import { notebookJSON } from "./fakes";
import { ada, assetNode, pageNode, pageServer, versionJSON } from "./page-server";

const root = pageNode(30, "raw");
const folder = pageNode(31, "folder", root);
const source = pageNode(32, "source", folder);
const asset = assetNode(33, "evidence.pdf", folder);
const outside = pageNode(34, "outside");
const agents = pageNode(35, "AGENTS");
const nodes = [root, folder, source, asset, outside, agents];

function fixture(role: "admin" | "editor" = "editor", mode: "llm_wiki" | "standard" = "llm_wiki") {
  const options = { role, mode, sourceRootId: root.id, nodes: [...nodes] };
  const server = pageServer(options);
  return { server, page: new PageService(server.app.session.public) };
}

const refused = (code: string) => ({ code, status: code === "page.source_admin_only" ? 403 : 409 });

describe("source management in the page fake", () => {
  test.each([source, asset])(
    "writer cannot rename, move, reorder or delete $name through its full parent chain",
    async (node) => {
      const { server, page } = fixture();
      const before = [...server.nodes];
      await expect(page.renameNode(node.id, node.kind === "asset" ? "renamed.pdf" : "renamed")).rejects.toMatchObject(
        refused("page.source_admin_only")
      );
      await expect(page.moveNode(node.id, { parent_id: null })).rejects.toMatchObject(
        refused("page.source_admin_only")
      );
      await expect(
        page.moveNode(node.id, { parent_id: folder.id, after_id: node.id === source.id ? asset.id : null })
      ).rejects.toMatchObject(refused("page.source_admin_only"));
      await expect(page.deleteNode(node.id)).rejects.toMatchObject(refused("page.source_admin_only"));
      expect(server.nodes).toEqual(before);
      expect(server.trash.size).toBe(0);
    }
  );

  test.each([source, asset])("admin manages $name but cannot delete the source root", async (node) => {
    const { server, page } = fixture("admin");
    const name = node.kind === "asset" ? "renamed.pdf" : "renamed";
    await expect(page.renameNode(node.id, name)).resolves.toMatchObject({ name });
    await expect(page.moveNode(node.id, { parent_id: folder.id, after_id: null })).resolves.toMatchObject({
      parent_id: folder.id,
    });
    await expect(page.deleteNode(node.id)).resolves.toBeUndefined();
    expect(server.trash.size).toBe(1);
    await expect(page.deleteNode(root.id)).rejects.toMatchObject(refused("page.protected"));
    expect(server.nodes.some((each) => each.id === root.id)).toBe(true);
  });

  test("writer creates and moves ordinary nodes in, then loses source management", async () => {
    const { page } = fixture();
    await expect(page.createPage(notebookJSON.id, folder.id, "new")).resolves.toMatchObject({ parent_id: folder.id });
    await expect(page.moveNode(outside.id, { parent_id: folder.id })).resolves.toMatchObject({ parent_id: folder.id });
    await expect(page.renameNode(outside.id, "now a source")).rejects.toMatchObject(refused("page.source_admin_only"));
  });

  test("writer's unchanged source name and position remain no-ops", async () => {
    const { server, page } = fixture();
    const before = [...server.nodes];
    await expect(page.renameNode(source.id, source.name)).resolves.toMatchObject({ id: source.id });
    await expect(page.moveNode(source.id, { parent_id: folder.id, after_id: null })).resolves.toMatchObject({
      id: source.id,
    });
    expect(server.nodes).toEqual(before);
  });

  test("standard mode ignores its nominated source root", async () => {
    const { page } = fixture("editor", "standard");
    await expect(page.renameNode(source.id, "ordinary")).resolves.toMatchObject({ name: "ordinary" });
    await expect(page.deleteNode(root.id)).resolves.toBeUndefined();
  });
});

describe("source content in the page fake", () => {
  test.each(["admin", "editor"] as const)(
    "%s cannot change nonempty content; same bytes keep the stale-base no-op",
    async (role) => {
      const { server, page } = fixture(role);
      server.contents.set(source.id, { content: "evidence", revision: 3 });
      await expect(page.putPageContent(source.id, { content: "different", base_revision: 3 })).rejects.toMatchObject(
        refused("page.source_immutable")
      );
      await expect(page.putPageContent(source.id, { content: "different", base_revision: 1 })).rejects.toMatchObject(
        refused("page.revision_mismatch")
      );
      await expect(page.putPageContent(source.id, { content: "evidence", base_revision: 1 })).resolves.toMatchObject({
        revision: 3,
      });
      expect(server.contents.get(source.id)).toEqual({ content: "evidence", revision: 3 });
      expect(server.versions.size).toBe(0);
    }
  );

  test("empty source permits one changed one-shot write, root and AGENTS remain editable", async () => {
    const { server, page } = fixture();
    server.contents.set(source.id, { content: "", revision: 1 });
    await expect(page.putPageContent(source.id, { content: "first", base_revision: 1 })).resolves.toMatchObject({
      revision: 2,
    });
    await expect(page.putPageContent(source.id, { content: "second", base_revision: 2 })).rejects.toMatchObject(
      refused("page.source_immutable")
    );
    await Promise.all(
      [root, agents, outside].map(async (node) => {
        await expect(page.putPageContent(node.id, { content: "edited", base_revision: 1 })).resolves.toMatchObject({
          revision: 2,
        });
        await expect(page.openEditSession(node.id)).resolves.toMatchObject({ page_id: node.id });
      })
    );
  });

  test("session veto precedes locks and leaves takeover tombstones unchanged, even for an empty source", async () => {
    const { server, page } = fixture();
    server.contents.set(source.id, { content: "", revision: 1 });
    const session = server.hold(source.id, ada);
    const before = structuredClone([...server.sessions]);
    await expect(page.openEditSession(source.id, true)).rejects.toMatchObject(refused("page.source_immutable"));
    expect([...server.sessions]).toEqual(before);
    await expect(
      page.putPageContent(source.id, { content: "", base_revision: 0, edit_session_id: session })
    ).rejects.toMatchObject(refused("page.source_immutable"));
    await expect(
      page.putPageContent(source.id, { content: "first", base_revision: 1, edit_session_id: session })
    ).rejects.toMatchObject(refused("page.source_immutable"));
    await expect(
      page.putPageContent(source.id, { content: "", base_revision: 0, edit_session_id: "missing" })
    ).rejects.toMatchObject(refused("page.edit_session_ended"));
    server.takeOver(source.id);
    await expect(
      page.putPageContent(source.id, { content: "", base_revision: 0, edit_session_id: session })
    ).rejects.toMatchObject(refused("page.edit_session_taken_over"));
    expect(server.contents.get(source.id)).toEqual({ content: "", revision: 1 });
  });

  test("version restore refuses same bytes and changing bytes, after a changed-base mismatch but before locks", async () => {
    const { server, page } = fixture();
    server.withVersions(source.id, [versionJSON(1, null, "old"), versionJSON(2, 1, "current")]);
    server.hold(source.id);
    const versions = [...(server.versions.get(source.id) ?? [])];
    await expect(page.restoreVersion(source.id, 2, 0)).rejects.toMatchObject(refused("page.source_immutable"));
    await expect(page.restoreVersion(source.id, 1, 0)).rejects.toMatchObject(refused("page.revision_mismatch"));
    await expect(page.restoreVersion(source.id, 1, 2)).rejects.toMatchObject(refused("page.source_immutable"));
    expect(server.versions.get(source.id)).toEqual(versions);
    expect(server.contents.get(source.id)).toEqual({ content: "current", revision: 2 });
  });

  test("tasks keep base and offset errors and no-ops, then immutable before the edit lock", async () => {
    const { server, page } = fixture();
    server.withTasks(source.id, "- [ ] evidence\n", 4);
    server.hold(source.id);
    const before = server.contents.get(source.id);
    await expect(page.toggleTask(source.id, { base_revision: 1, offset: 3, checked: true })).rejects.toMatchObject(
      refused("page.revision_mismatch")
    );
    await expect(page.toggleTask(source.id, { base_revision: 4, offset: 2, checked: true })).rejects.toMatchObject({
      code: "validation_failed",
      status: 422,
    });
    await expect(page.toggleTask(source.id, { base_revision: 4, offset: 3, checked: false })).resolves.toMatchObject({
      revision: 4,
    });
    await expect(page.toggleTask(source.id, { base_revision: 4, offset: 3, checked: true })).rejects.toMatchObject(
      refused("page.source_immutable")
    );
    expect(server.contents.get(source.id)).toEqual(before);
  });
});

// The whole notebook is not the move's size: count the carried root and every
// descendant kind, including empty pages. Removing the source-entry-only
// condition, counting pages alone, or using >= instead of > breaks these calls.
test.each([
  { shape: "entry at the bound", size: 10000, mode: "llm_wiki", from: "outside", to: "inside", refused: false },
  { shape: "entry past the bound", size: 10001, mode: "llm_wiki", from: "outside", to: "inside", refused: true },
  { shape: "standard large move", size: 10001, mode: "standard", from: "outside", to: "inside", refused: false },
  { shape: "large move outside", size: 10001, mode: "llm_wiki", from: "outside", to: "outside", refused: false },
  { shape: "large move out", size: 10001, mode: "llm_wiki", from: "inside", to: "outside", refused: false },
  { shape: "large move within", size: 10001, mode: "llm_wiki", from: "inside", to: "inside", refused: false },
] as const)("source entry limit: $shape", async ({ size, mode, from, to, refused: expectRefusal }) => {
  const raw = pageNode(60, "raw");
  const inside = pageNode(61, "Inside", raw);
  const outsideParent = pageNode(62, "Outside");
  const carriedFolder = pageNode(63, "Folder", from === "inside" ? raw : undefined);
  const children = Array.from({ length: size - 1 }, (_, i) => ({
    ...(i % 2 === 0 ? pageNode(64, `Empty ${i}`, carriedFolder) : assetNode(64, `File ${i}.txt`, carriedFolder)),
    id: `0199a2b4-0000-7000-8000-${(1000 + i).toString(16).padStart(12, "0")}`,
  }));
  const server = pageServer({
    role: "admin",
    mode,
    sourceRootId: raw.id,
    nodes: [raw, inside, outsideParent, carriedFolder, ...children],
  });
  for (const node of children) {
    if (node.kind === "page") server.contents.set(node.id, { content: "", revision: 1 });
  }
  const before = structuredClone({ nodes: server.nodes, contents: [...server.contents] });
  const destination = to === "inside" ? inside.id : outsideParent.id;
  const request = new PageService(server.app.session.public).moveNode(carriedFolder.id, { parent_id: destination });
  if (expectRefusal) {
    await expect(request).rejects.toMatchObject({ code: "page.source_too_large", status: 422 });
    expect({ nodes: server.nodes, contents: [...server.contents] }).toEqual(before);
  } else {
    await expect(request).resolves.toMatchObject({ id: carriedFolder.id, parent_id: destination });
    expect(server.nodes.filter((node) => node.parent_id === carriedFolder.id)).toHaveLength(size - 1);
    expect([...server.contents]).toEqual(before.contents);
  }
  expect(server.changesets).toEqual([]);
  expect(server.trash.size).toBe(0);
});

test.each(["root", "ancestor"] as const)("moving the source $shape keeps a large area's members", async (shape) => {
  const outer = pageNode(70, "Outer");
  const raw = pageNode(71, "raw", shape === "ancestor" ? outer : undefined);
  const destination = pageNode(72, "Destination");
  const children = Array.from({ length: 10001 }, (_, i) => ({
    ...assetNode(73, `File ${i}.txt`, raw),
    id: `0199a2b4-0000-7000-8000-${(20000 + i).toString(16).padStart(12, "0")}`,
  }));
  const server = pageServer({
    role: "admin",
    mode: "llm_wiki",
    sourceRootId: raw.id,
    nodes: [outer, raw, destination, ...children],
  });
  const moved = shape === "root" ? raw : outer;
  await expect(
    new PageService(server.app.session.public).moveNode(moved.id, { parent_id: destination.id })
  ).resolves.toMatchObject({
    id: moved.id,
    parent_id: destination.id,
  });
  expect(server.nodes.filter((node) => node.parent_id === raw.id)).toHaveLength(10001);
});

// C1: known position/title errors precede source-entry size. Each invalid
// position also has a title collision, so it must not fall through to that 409.
test.each(["title", "after another parent's child", "after itself"] as const)(
  "oversized source entry checks %s before size",
  async (invalid) => {
    const raw = pageNode(100, "raw");
    const carriedFolder = pageNode(101, "Folder");
    const holder = pageNode(102, "fOlDeR", raw);
    const children = Array.from({ length: 10000 }, (_, i) => ({
      ...assetNode(103, `File ${i}.txt`, carriedFolder),
      id: `0199a2b4-0000-7000-8000-${(80000 + i).toString(16).padStart(12, "0")}`,
    }));
    const server = pageServer({
      role: "admin",
      mode: "llm_wiki",
      sourceRootId: raw.id,
      nodes: [raw, carriedFolder, holder, ...children],
    });
    const before = structuredClone({
      nodes: server.nodes,
      contents: [...server.contents],
      trash: [...server.trash],
      history: server.changesets,
    });
    const after = invalid === "title" ? undefined : invalid === "after itself" ? carriedFolder.id : children[0]?.id;
    const request = new PageService(server.app.session.public).moveNode(carriedFolder.id, {
      parent_id: raw.id,
      after_id: after,
    });
    if (invalid === "title") {
      await expect(request).rejects.toMatchObject({ status: 409, code: "page.title_taken" });
    } else {
      await expect(request).rejects.toMatchObject({
        status: 422,
        code: "validation_failed",
        problem: { errors: [{ field: "after_id", code: "not_allowed" }] },
      });
    }
    expect({
      nodes: server.nodes,
      contents: [...server.contents],
      trash: [...server.trash],
      history: server.changesets,
    }).toEqual(before);
  }
);
