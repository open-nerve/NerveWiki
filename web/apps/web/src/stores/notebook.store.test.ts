import { expect, test } from "vitest";

import { ApiError } from "../services/api";
import type { Notebook } from "../services/notebook.service";
import { groupNotebooks, NotebookStore } from "./notebook.store";

const notebook = (id: string, name: string, overrides: Partial<Notebook> = {}): Notebook => ({
  id,
  workspace_id: "w1",
  name,
  workspace_access: "none",
  role: "admin",
  member_count: 1,
  created_at: "2026-10-02T08:00:00Z",
  updated_at: "2026-10-02T08:00:00Z",
  ...overrides,
});

type Service = ConstructorParameters<typeof NotebookStore>[0];

/** A store of lab's notebooks over a service whose list answers list, whose writes answer what they are given, and as overrides says. */
function storeOf(list: Notebook[], overrides: Partial<Service> = {}) {
  return new NotebookStore(
    {
      list: async () => list,
      create: async (_slug, body) => notebook(body.name.toLowerCase(), body.name),
      update: async (id, body) => notebook(id, body.name ?? id, { workspace_access: body.workspace_access ?? "none" }),
      remove: async () => {},
      leave: async () => {},
      ...overrides,
    },
    "lab"
  );
}

const names = (store: NotebookStore) => store.list?.map((n) => n.name);
const notFound = new ApiError(404, { status: 404, code: "notebook.not_found", title: "" });
const forbidden = new ApiError(403, { status: 403, code: "forbidden", title: "" });

// SWR reads the list again on focus, and a write may go out at the same
// moment: a read answered after the write's answer may hold the list from
// before, and must not undo the write (v0.1 design 13.2, item 1).
test.each([
  ["a creation", (store: NotebookStore) => store.create({ name: "Acme" }), ["Acme", "Beta"]],
  ["a renaming", (store: NotebookStore) => store.update("beta", { name: "Zeta" }), ["Zeta"]],
  ["a deletion", (store: NotebookStore) => store.remove("beta"), []],
  ["a leaving, still seen", (store: NotebookStore) => store.leave("beta"), ["beta"]],
])("a read answered after %s keeps it", async (_, write: (store: NotebookStore) => Promise<unknown>, expected) => {
  let answerRead: ((list: Notebook[]) => void) | undefined;
  let reads = 0;
  // The first read, the read overlapping the write, then a leaving's read: the notebook still seen.
  const store = storeOf([], {
    list: () =>
      ++reads === 1
        ? Promise.resolve([notebook("beta", "Beta")])
        : reads === 2
          ? new Promise<Notebook[]>((resolve) => (answerRead = resolve))
          : Promise.resolve([notebook("beta", "beta", { role: "reader", workspace_access: "viewer" })]),
  });
  await store.load();

  const read = store.load();
  await write(store);
  answerRead?.([notebook("beta", "Beta")]);

  expect((await read).map((n) => n.name)).toEqual(expected);
  expect(names(store)).toEqual(expected);
});

test("a created or renamed notebook takes its place by name, as the server orders it; a read holding it keeps it once", async () => {
  let answerCreate: ((n: Notebook) => void) | undefined;
  let reads = 0;
  const store = storeOf([], {
    list: async () =>
      ++reads === 1
        ? [notebook("a", "alpha"), notebook("z", "Zeta")]
        : [notebook("a", "alpha"), notebook("m", "Mid"), notebook("z", "Zeta")],
    create: () => new Promise<Notebook>((resolve) => (answerCreate = resolve)),
  });
  await store.load();

  const creation = store.create({ name: "Mid" });
  await store.load();
  answerCreate?.(notebook("m", "Mid"));
  expect(await creation).toEqual(notebook("m", "Mid"));
  expect(names(store)).toEqual(["alpha", "Mid", "Zeta"]);

  const renamed = await store.update("a", { name: "Zulu" });
  expect(names(store)).toEqual(["Mid", "Zeta", "Zulu"]);
  expect(store.byId("a")).toEqual(renamed);
});

test("a write before the first read leaves the list to that read", async () => {
  const store = storeOf([notebook("beta", "Beta")]);

  await store.create({ name: "Acme" });
  expect(store.list).toBeUndefined();

  await store.load();
  expect(names(store)).toEqual(["Beta"]);
});

test("a deletion: the notebook is gone, and one gone already is gone as well; a refusal keeps it", async () => {
  let refusal: ApiError | undefined;
  const store = storeOf([notebook("a", "A"), notebook("b", "B"), notebook("c", "C")], {
    remove: () => (refusal === undefined ? Promise.resolve() : Promise.reject(refusal)),
  });
  await store.load();

  await store.remove("a");
  refusal = notFound;
  await store.remove("b");
  expect([names(store), store.wasRemoved("a"), store.wasRemoved("b")]).toEqual([["C"], true, true]);

  refusal = forbidden;
  await expect(store.remove("c")).rejects.toBe(forbidden);
  expect([names(store), store.wasRemoved("c")]).toEqual([["C"], false]);
});

// Leaving reads the notebooks again (M3/P4 design 3.2): one open to the
// workspace stays, with the default role; one no longer seen is gone.
const stillSeen = notebook("a", "a", { role: "reader", workspace_access: "viewer" });
test.each([
  ["still seen", undefined, [stillSeen, notebook("b", "b")], [["a", "reader", "viewer"]], false],
  ["no longer seen", undefined, [notebook("b", "b")], [], true],
  ["gone already", notFound, [stillSeen], [], true],
])(
  "a leaving, the notebook %s",
  async (_, leaveRefusal: ApiError | undefined, after: Notebook[], expected, removed) => {
    let reads = 0;
    const store = storeOf([], {
      list: async () => (++reads === 1 ? [notebook("a", "a")] : after),
      leave: () => (leaveRefusal === undefined ? Promise.resolve() : Promise.reject(leaveRefusal)),
    });
    await store.load();

    await store.leave("a");

    expect(store.list?.map((n) => [n.id, n.role, n.workspace_access])).toEqual(expected);
    expect(store.wasRemoved("a")).toBe(removed);
  }
);

test("a leaving refused keeps the notebook, and says why", async () => {
  const ended = new ApiError(404, { status: 404, code: "notebook.member_not_found", title: "" });
  const soleAdmin = new ApiError(409, { status: 409, code: "notebook.sole_admin", title: "" });
  let refusal = ended;
  const store = storeOf([notebook("a", "A")], { leave: () => Promise.reject(refusal) });
  await store.load();

  await expect(store.leave("a")).rejects.toBe(ended);
  refusal = soleAdmin;
  await expect(store.leave("a")).rejects.toBe(soleAdmin);

  expect([names(store), store.wasRemoved("a")]).toEqual([["A"], false]);
});

// A notebook private with one member is the account's own (M3 design 5).
test.each([
  ["private, one member", { workspace_access: "none", member_count: 1 }, "mine"],
  ["private, two members", { workspace_access: "none", member_count: 2 }, "team"],
  ["open, one member", { workspace_access: "viewer", member_count: 1 }, "team"],
  ["open to editing, no member", { workspace_access: "editor", member_count: 0 }, "team"],
] as const)("a notebook %s is the %s group's", (_, fields, group) => {
  const groups = groupNotebooks([notebook("x", "X", fields)]);
  expect([groups.mine.length, groups.team.length]).toEqual(group === "mine" ? [1, 0] : [0, 1]);
});

test("each group keeps the list's order", () => {
  const list = [
    notebook("a", "A"),
    notebook("b", "B", { member_count: 2 }),
    notebook("c", "C"),
    notebook("d", "D", { workspace_access: "viewer" }),
  ];
  const { mine, team } = groupNotebooks(list);
  expect([mine.map((n) => n.id), team.map((n) => n.id)]).toEqual([
    ["a", "c"],
    ["b", "d"],
  ]);
});

// A notebook's changes go out one at a time (v0.1 design 13.2, item 1): its
// general page's two forms may each send while the other's change is out,
// and the server's answers may come back in any order.
test("a notebook's changes go out one at a time, the last answered last", async () => {
  const answers: ((n: Notebook) => void)[] = [];
  const sent: string[] = [];
  const store = storeOf([notebook("a", "A")], {
    update: (id, body) => {
      sent.push(JSON.stringify(body));
      return new Promise<Notebook>((resolve) => answers.push(resolve));
    },
  });
  await store.load();

  const renaming = store.update("a", { name: "B" });
  const opening = store.update("a", { workspace_access: "viewer" });
  await Promise.resolve();
  expect(sent).toEqual(['{"name":"B"}']);
  answers[0]?.(notebook("a", "B"));
  await renaming;
  await Promise.resolve();
  expect(sent).toEqual(['{"name":"B"}', '{"workspace_access":"viewer"}']);
  answers[1]?.(notebook("a", "B", { workspace_access: "viewer" }));
  await opening;

  expect(store.list?.map((n) => [n.name, n.workspace_access])).toEqual([["B", "viewer"]]);
});

// New notebook shows before the first read answers: a creation answered
// while that read is out is read again, not lost to it.
test("a creation answered while the first read is out is in the list that read leaves", async () => {
  let answerFirst: ((list: Notebook[]) => void) | undefined;
  let reads = 0;
  const store = storeOf([], {
    list: () =>
      ++reads === 1
        ? new Promise<Notebook[]>((resolve) => (answerFirst = resolve))
        : Promise.resolve([notebook("acme", "Acme")]),
  });

  const read = store.load();
  await store.create({ name: "Acme" });
  answerFirst?.([]);

  expect((await read).map((n) => n.name)).toEqual(["Acme"]);
  expect([names(store), reads]).toEqual([["Acme"], 2]);
});

test("a leaving answered, whose read again fails, is done: the notebook stays until a read without it sends it home", async () => {
  let reads = 0;
  const store = storeOf([], {
    list: () => (++reads === 1 ? Promise.resolve([notebook("a", "A")]) : Promise.reject(new TypeError("offline"))),
  });
  await store.load();

  await store.leave("a");

  expect([names(store), store.wasRemoved("a")]).toEqual([["A"], true]);
});
