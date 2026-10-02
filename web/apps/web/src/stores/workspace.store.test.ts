import { expect, test } from "vitest";

import { ApiError } from "../services/api";
import type { Workspace } from "../services/workspace.service";
import { WorkspaceStore } from "./workspace.store";

const workspace = (slug: string, name: string, id = slug): Workspace => ({
  id,
  slug,
  name,
  role: "admin",
  created_at: "2026-10-01T08:00:00Z",
  updated_at: "2026-10-01T08:00:00Z",
});

type Service = ConstructorParameters<typeof WorkspaceStore>[0];

/** A store over a service whose list answers list, whose writes answer what they are given, and as overrides says. */
function storeOf(list: Workspace[], overrides: Partial<Service> = {}) {
  return new WorkspaceStore({
    list: async () => list,
    create: async (body) => workspace(body.slug, body.name),
    rename: async (slug, name) => ({ ...workspace(slug, name), updated_at: "2026-10-01T09:00:00Z" }),
    remove: async () => {},
    leave: async () => {},
    accept: async ({ id }) => workspace(id, id),
    checkSlug: async () => ({ available: true }),
    ...overrides,
  });
}

const slugs = (store: WorkspaceStore) => store.list?.map((w) => w.slug);

// SWR reads the list again on focus, and a write may go out at the same
// moment: a read answered after the write's answer may hold the list from
// before, and must not undo the write. Each write is held to it.
test.each([
  ["a creation", (store: WorkspaceStore) => store.create({ name: "Acme", slug: "acme" }), ["Acme", "Beta"]],
  ["an acceptance", (store: WorkspaceStore) => store.accept({ id: "acme", token: "nwk_inv_acme" }), ["acme", "Beta"]],
  ["a renaming", (store: WorkspaceStore) => store.rename("beta", "Zeta"), ["Zeta"]],
  ["a deletion", (store: WorkspaceStore) => store.remove("beta"), []],
  ["a leaving", (store: WorkspaceStore) => store.leave("beta"), []],
])("a read answered after %s keeps it", async (_, write: (store: WorkspaceStore) => Promise<unknown>, names) => {
  let answerRead: ((list: Workspace[]) => void) | undefined;
  let reads = 0;
  const store = storeOf([], {
    list: () =>
      ++reads === 1
        ? Promise.resolve([workspace("beta", "Beta")])
        : new Promise<Workspace[]>((resolve) => (answerRead = resolve)),
  });
  await store.load();

  const read = store.load();
  await write(store);
  answerRead?.([workspace("beta", "Beta")]);

  expect((await read).map((w) => w.name)).toEqual(names);
  expect(store.list?.map((w) => w.name)).toEqual(names);
});

test("a created or renamed workspace takes its place by name; a deleted one leaves", async () => {
  const store = storeOf([workspace("acme", "Acme"), workspace("zeta", "zeta")]);
  await store.load();

  await store.create({ name: "beta", slug: "beta" });
  expect(slugs(store)).toEqual(["acme", "beta", "zeta"]);

  const renamed = await store.rename("acme", "Zulu");
  expect(slugs(store)).toEqual(["beta", "zeta", "acme"]);
  expect(store.bySlug("acme")).toEqual(renamed);

  await store.remove("beta");
  expect(slugs(store)).toEqual(["zeta", "acme"]);
  expect(store.bySlug("beta")).toBeUndefined();
});

test("names that differ in case only are ordered as the server does: lower(name), name, id", async () => {
  const store = storeOf([]);
  await store.load();

  await Promise.all(
    [
      ["b2", "b"],
      ["a2", "B"],
      ["a1", "a"],
    ].map(([slug = "", name = ""]) => store.create({ name, slug }))
  );

  expect(slugs(store)).toEqual(["a1", "a2", "b2"]);
});

test("a write before the first read leaves the list to that read", async () => {
  const store = storeOf([workspace("beta", "Beta")]);

  await store.create({ name: "Acme", slug: "acme" });
  expect(store.list).toBeUndefined();

  await store.load();
  expect(slugs(store)).toEqual(["beta"]);
});

// A read that goes out after the creation was committed, and comes back
// before the creation's answer, holds the new workspace already.
test("a read that holds a workspace being created keeps it once", async () => {
  const acme = workspace("acme", "Acme");
  let answerCreate: ((w: Workspace) => void) | undefined;
  let reads = 0;
  const store = storeOf([], {
    list: async () => (++reads === 1 ? [workspace("beta", "Beta")] : [acme, workspace("beta", "Beta")]),
    create: () => new Promise<Workspace>((resolve) => (answerCreate = resolve)),
  });
  await store.load();

  const creation = store.create({ name: "Acme", slug: "acme" });
  await store.load();
  answerCreate?.(acme);
  await creation;

  expect(slugs(store)).toEqual(["acme", "beta"]);
});

test("the same name is ordered by id, as the server orders it", async () => {
  const store = storeOf([]);
  await store.load();

  await store.create({ name: "Acme", slug: "b" });
  await store.create({ name: "Acme", slug: "a" });

  expect(slugs(store)).toEqual(["a", "b"]);
});

test.each(["remove", "leave"] as const)(
  "%s: the workspace is gone, and one the account no longer has already is gone as well",
  async (write) => {
    const notFound = new ApiError(404, { status: 404, code: "workspace.not_found", title: "" });
    const forbidden = new ApiError(403, { status: 403, code: "forbidden", title: "" });
    let refusal: ApiError | undefined;
    const send = () => (refusal === undefined ? Promise.resolve() : Promise.reject(refusal));
    const store = storeOf([workspace("acme", "Acme"), workspace("beta", "Beta"), workspace("zeta", "Zeta")], {
      [write]: send,
    });
    await store.load();

    await store[write]("acme");
    refusal = notFound;
    await store[write]("beta");
    expect([slugs(store), store.wasRemoved("acme"), store.wasRemoved("beta")]).toEqual([["zeta"], true, true]);

    refusal = forbidden;
    await expect(store[write]("zeta")).rejects.toBe(forbidden);
    expect([slugs(store), store.wasRemoved("zeta")]).toEqual([["zeta"], false]);
  }
);

test("an invitation accepted takes its workspace's place by name, once", async () => {
  let answerAccept: ((joined: Workspace) => void) | undefined;
  let reads = 0;
  const store = storeOf([], {
    list: async () =>
      ++reads === 1
        ? [workspace("acme", "Acme"), workspace("zeta", "Zeta")]
        : [workspace("acme", "Acme"), workspace("lab", "Lab"), workspace("zeta", "Zeta")],
    accept: () => new Promise<Workspace>((resolve) => (answerAccept = resolve)),
  });
  await store.load();

  // A read that went out after the acceptance, answered before it.
  const accepting = store.accept({ id: "i1", token: "nwk_inv_x" });
  await store.load();
  answerAccept?.(workspace("lab", "Lab"));

  expect(await accepting).toEqual(workspace("lab", "Lab"));
  expect(slugs(store)).toEqual(["acme", "lab", "zeta"]);
});

test("an invitation accepted adds its workspace to the list read", async () => {
  const store = storeOf([workspace("acme", "Acme"), workspace("zeta", "Zeta")], {
    accept: async () => ({ ...workspace("lab", "Lab"), role: "guest" }),
  });
  await store.load();

  await store.accept({ id: "i1", token: "nwk_inv_x" });

  expect(store.list?.map((w) => `${w.slug}:${w.role}`)).toEqual(["acme:admin", "lab:guest", "zeta:admin"]);
});

test("an invitation accepted before the first read leaves the list to it", async () => {
  const store = storeOf([workspace("lab", "Lab")]);

  await store.accept({ id: "lab", token: "nwk_inv_x" });
  expect(store.list).toBeUndefined();
  await store.load();

  expect(slugs(store)).toEqual(["lab"]);
});

// A workspace's renames go out one at a time (v0.1 design 13.2, item 1):
// the store outlives the general page, whose form, mounted anew, may send
// while the rename before is out; the server's answers may come back in
// any order (M3 Codex review R1).
test("a workspace's renames go out one at a time, the last made last", async () => {
  const answers: ((w: Workspace) => void)[] = [];
  const sent: string[] = [];
  const store = storeOf([workspace("acme", "Acme")], {
    rename: (slug, name) => {
      sent.push(name);
      return new Promise<Workspace>((resolve) => answers.push(resolve));
    },
  });
  await store.load();

  const first = store.rename("acme", "Acme Works");
  const second = store.rename("acme", "Acme Labs");
  await Promise.resolve();
  expect(sent).toEqual(["Acme Works"]);
  answers[0]?.(workspace("acme", "Acme Works"));
  await first;
  await Promise.resolve();
  expect(sent).toEqual(["Acme Works", "Acme Labs"]);
  answers[1]?.(workspace("acme", "Acme Labs"));
  await second;

  expect(store.bySlug("acme")?.name).toBe("Acme Labs");
});

// A workspace's deletion and leaving wait for its rename still out, in the
// same queue (M3 Codex review R1).
test("a workspace's deletion waits for its rename still out", async () => {
  let answer: ((w: Workspace) => void) | undefined;
  const sent: string[] = [];
  const store = storeOf([workspace("acme", "Acme")], {
    rename: (_slug, name) => {
      sent.push(`rename ${name}`);
      return new Promise<Workspace>((resolve) => (answer = resolve));
    },
    remove: async (slug) => {
      sent.push(`remove ${slug}`);
    },
  });
  await store.load();

  const renaming = store.rename("acme", "Acme Works");
  const removing = store.remove("acme");
  await Promise.resolve();
  expect(sent).toEqual(["rename Acme Works"]);
  answer?.(workspace("acme", "Acme Works"));
  await Promise.all([renaming, removing]);

  expect([sent, slugs(store), store.wasRemoved("acme")]).toEqual([["rename Acme Works", "remove acme"], [], true]);
});
