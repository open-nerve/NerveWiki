import { expect, test } from "vitest";

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

/** A store over a service whose list answers list, and whose writes answer what they are given. */
function storeOf(list: Workspace[]) {
  return new WorkspaceStore({
    list: async () => list,
    create: async (body) => workspace(body.slug, body.name),
    rename: async (slug, name) => ({ ...workspace(slug, name), updated_at: "2026-10-01T09:00:00Z" }),
    remove: async () => {},
    checkSlug: async () => ({ available: true }),
  });
}

const slugs = (store: WorkspaceStore) => store.list?.map((w) => w.slug);

// SWR reads the list again on focus, and a creation may go out at the same
// moment: a read answered after it may hold the list from before, and must
// not take the new workspace away.
test("a read answered after a creation keeps the new workspace", async () => {
  let answerRead: ((list: Workspace[]) => void) | undefined;
  let reads = 0;
  const store = new WorkspaceStore({
    list: () =>
      ++reads === 1
        ? Promise.resolve([workspace("beta", "Beta")])
        : new Promise<Workspace[]>((resolve) => (answerRead = resolve)),
    create: async (body) => workspace(body.slug, body.name),
    rename: async () => workspace("x", "X"),
    remove: async () => {},
    checkSlug: async () => ({ available: true }),
  });
  await store.load();

  const read = store.load();
  await store.create({ name: "Acme", slug: "acme" });
  answerRead?.([workspace("beta", "Beta")]);

  expect((await read).map((w) => w.slug)).toEqual(["acme", "beta"]);
  expect(slugs(store)).toEqual(["acme", "beta"]);
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
