import { expect, test } from "vitest";

import type { TransferJob, TransferJobPage, TransferService } from "../services/transfer.service";
import { jobJSON, underWay } from "../test/jobs-server";
import { bob } from "../test/page-server";
import { TransferStore } from "./transfer.store";

const job = (n: number, more: Partial<TransferJob> = {}) => jobJSON(n, more);
const page = (jobs: TransferJob[], next: string | null = null): TransferJobPage => ({ data: jobs, next_cursor: next });

type Service = Pick<
  TransferService,
  "list" | "startExport" | "startImport" | "startTrashRestore" | "startRevert" | "cancel" | "get"
>;

/** A request of the store's service held until the test answers it. */
type Held<A, T> = { ask: A; answer: (answered: T) => void; fail: (error: unknown) => void };

/**
 * A store of n1's jobs over a service whose every list, start, import,
 * restore, revert and cancel waits for the test to answer it: lists,
 * starts, imports, restores, reverts and cancels have them in order, with
 * their cursor, root, place and file's name, node, changeset and message,
 * and id; warned, what the store asked the browser's warning.
 */
function storeOf(more: Partial<Service> = {}) {
  const lists: Held<string | undefined, TransferJobPage>[] = [];
  const starts: Held<string | null, TransferJob>[] = [];
  const imports: Held<string, TransferJob>[] = [];
  const restores: Held<string, TransferJob>[] = [];
  const reverts: Held<string, TransferJob>[] = [];
  const cancels: Held<string, TransferJob>[] = [];
  const warned: boolean[] = [];
  const store = new TransferStore(
    {
      list: (_notebook, cursor) =>
        new Promise((resolve, reject) => lists.push({ ask: cursor, answer: resolve, fail: reject })),
      startExport: (_notebook, rootId) =>
        new Promise((resolve, reject) => starts.push({ ask: rootId, answer: resolve, fail: reject })),
      startImport: (_notebook, parent, file) =>
        new Promise((resolve, reject) =>
          imports.push({ ask: `${parent ?? "root"} ${file.name}`, answer: resolve, fail: reject })
        ),
      startTrashRestore: (_notebook, node) =>
        new Promise((resolve, reject) => restores.push({ ask: node, answer: resolve, fail: reject })),
      startRevert: (_notebook, changeset, message) =>
        new Promise((resolve, reject) =>
          reverts.push({ ask: `${changeset} ${message ?? "-"}`, answer: resolve, fail: reject })
        ),
      cancel: (id) => new Promise((resolve, reject) => cancels.push({ ask: id, answer: resolve, fail: reject })),
      get: () => Promise.reject(new Error("not read")),
      ...more,
    },
    "n1",
    (on) => warned.push(on)
  );
  return { store, lists, starts, imports, restores, reverts, cancels, warned };
}

/** answer answers the held request i of asks with answered, and waits for what came of it. */
async function answer<A, T>(asks: Held<A, T>[], i: number, answered: T, done?: Promise<unknown>): Promise<void> {
  const held = asks[i];
  if (held === undefined) {
    throw new Error(`no request ${i.toString()} yet`);
  }
  held.answer(answered);
  await (done ?? Promise.resolve());
}

/** settled waits for the store to act on what was answered. */
const settled = () => new Promise((resolve) => setTimeout(resolve, 0));

const ids = (store: TransferStore) => store.jobs?.map((each) => Number(each.id.slice(-2)));

/** A store holding [4, 3] from its first page and [2] from the next, whose page after is c3. */
async function holding() {
  const held = storeOf();
  const { store, lists } = held;
  await answer(lists, 0, page([job(4), job(3)], "c2"), store.load());
  await answer(lists, 1, page([job(2)], "c3"), store.more());
  return held;
}

test("a read again reads back as many pages as are held, each job as it is now", async () => {
  const { store, lists } = await holding();

  const load = store.load();
  await answer(lists, 2, page([job(4), job(3)], "c2"));
  await settled();
  await answer(lists, 3, page([job(2, { state: "expired", download: null })], "c3"), load);

  expect(lists.map((list) => list.ask)).toEqual([undefined, "c2", undefined, "c2"]);
  expect([ids(store), store.nextCursor, store.jobs?.at(-1)?.state]).toEqual([[4, 3, 2], "c3", "expired"]);
});

// The server's pages do not overlap (its cursor is a key of time and id); a job twice is kept once all the same.
test("a read again whose pages moved, a job started elsewhere, reads them from where they are now; a job twice, once", async () => {
  const { store, lists } = await holding();

  const load = store.load();
  await answer(lists, 2, page([job(5), job(4)], "c4"));
  await settled();
  await answer(lists, 3, page([job(4), job(3), job(2)], "c1"), load);

  expect(lists.map((list) => list.ask)).toEqual([undefined, "c2", undefined, "c4"]);
  expect([ids(store), store.nextCursor]).toEqual([[5, 4, 3, 2], "c1"]);
});

test("a read again stops at the last page, fewer than are held", async () => {
  const { store, lists } = await holding();

  await answer(lists, 2, page([job(4)]), store.load());

  expect(lists.map((list) => list.ask)).toEqual([undefined, "c2", undefined]);
  expect([ids(store), store.nextCursor]).toEqual([[4], null]);
});

test("a page that more adds while a read again is out is read on to", async () => {
  const { store, lists } = storeOf();
  await answer(lists, 0, page([job(4), job(3)], "c2"), store.load());
  const load = store.load();
  const more = store.more();

  await answer(lists, 2, page([job(2)]), more);
  await answer(lists, 1, page([job(4), job(3)], "c2"));
  await settled();
  await answer(lists, 3, page([job(2)]), load);

  expect(lists.map((list) => list.ask)).toEqual([undefined, undefined, "c2", "c2"]);
  expect([ids(store), store.nextCursor]).toEqual([[4, 3, 2], null]);
});

test("a page answered once a read again moved the cursor is not added", async () => {
  const { store, lists } = storeOf();
  await answer(lists, 0, page([job(4), job(3)], "c2"), store.load());
  const more = store.more();
  await answer(lists, 2, page([job(5), job(4)], "c4"), store.load());

  await answer(lists, 1, page([job(3)]), more);

  await expect(more).resolves.toEqual([]);
  expect([ids(store), store.nextCursor]).toEqual([[5, 4], "c4"]);
});

test.each([
  ["queued", underWay(job(1), 0, 0), true],
  ["running", underWay(job(1), 1, 4), true],
  ["ended", job(1, { state: "failed", download: null }), false],
])("active tells whether a job is under way: %s", async (_, held, active) => {
  const { store, lists } = storeOf();
  expect(store.active).toBe(false);

  await answer(lists, 0, page([job(2), held]), store.load());

  expect(store.active).toBe(active);
});

test("more adds the next page's jobs, a job twice once, answers them, and nothing on the last page", async () => {
  const { store, lists } = storeOf();
  await answer(lists, 0, page([job(4), job(3)], "c1"), store.load());

  const added = store.more();
  expect(store.more()).toBe(added);
  await answer(lists, 1, page([job(3), job(2)], "c2"), added);
  const last = store.more();
  await answer(lists, 2, page([job(1)]), last);

  expect((await added).map((each) => each.id)).toEqual([job(2).id]);
  expect([ids(store), store.nextCursor]).toEqual([[4, 3, 2, 1], null]);
  await expect(store.more()).resolves.toEqual([]);
  expect(lists.map((list) => list.ask)).toEqual([undefined, "c1", "c2"]);
});

test("an export started goes first, once though a read had it; the read on its way, which may not have it, is dropped", async () => {
  const { store, lists, starts } = storeOf();
  await answer(lists, 0, page([job(1)]), store.load());
  const begun = store.start("p1");
  await settled();
  // A read out after the export began has it already.
  await answer(lists, 1, page([underWay(job(9), 0, 0), job(1)]), store.load());
  const load = store.load();

  await answer(starts, 0, underWay(job(9, { root_id: "p1" }), 0, 0), begun);
  await answer(lists, 2, page([job(1)]), load);

  expect(starts.map((start) => start.ask)).toEqual(["p1"]);
  expect((await begun).root_id).toBe("p1");
  expect(ids(store)).toEqual([9, 1]);
  expect(store.active).toBe(true);
});

test("an export started before the list is read: the read out meanwhile is read again, the list then held", async () => {
  const { store, lists, starts } = storeOf();
  const load = store.load();
  const begun = store.start(null);
  await settled();
  await answer(starts, 0, underWay(job(9), 0, 0), begun);
  expect([ids(store), store.loaded]).toEqual([[9], false]);

  await answer(lists, 0, page([job(1)]));
  await settled();
  await answer(lists, 1, page([underWay(job(9), 0, 0), job(1)]), load);

  expect([ids(store), store.loaded]).toEqual([[9, 1], true]);
});

test("an export started before the list is read: the read again that fails fails, the list still not held", async () => {
  const { store, lists, starts } = storeOf();
  const load = store.load();
  const begun = store.start(null);
  await settled();
  await answer(starts, 0, underWay(job(9), 0, 0), begun);

  await answer(lists, 0, page([job(1)]));
  await settled();
  lists[1]?.fail(new TypeError("offline"));

  await expect(load).rejects.toThrow("offline");
  expect([ids(store), store.loaded]).toEqual([[9], false]);
});

test("an import goes beside an export's start, first once answered, the read on its way dropped; the browser asks before the page is left as it uploads", async () => {
  const { store, lists, starts, imports, warned } = storeOf();
  await answer(lists, 0, page([job(1)]), store.load());
  const exported = store.start(null);
  const imported = store.startImport("p1", new File(["zip"], "Vault.zip"));
  await settled();
  expect([starts.map((start) => start.ask), imports.map((each) => each.ask)]).toEqual([[null], ["p1 Vault.zip"]]);
  expect([store.uploading, warned]).toEqual([1, [true]]);
  const load = store.load();

  const vault = underWay(job(9, { kind: "import", name: "Vault.zip", root_id: "p1" }), 0, 0);
  await answer(imports, 0, vault, imported);
  await answer(lists, 1, page([job(1)]), load);
  expect(ids(store)).toEqual([9, 1]);
  expect([store.uploading, warned, store.writing]).toEqual([0, [true, false], true]);
  await answer(starts, 0, underWay(job(8), 0, 0), exported);
  expect(ids(store)).toEqual([8, 9, 1]);
});

test.each([
  ["refused", new Error("transfer.busy")],
  ["stopped", new DOMException("the upload was stopped", "AbortError")],
])("an import %s leaves the jobs held and no longer asks before the page is left", async (_, error) => {
  const { store, lists, imports, warned } = storeOf();
  await answer(lists, 0, page([job(1)]), store.load());
  const imported = store.startImport(null, new File(["zip"], "Vault.zip"));
  await settled();
  imports[0]?.fail(error);

  await expect(imported).rejects.toBe(error);
  expect([ids(store), store.uploading, warned, imports.map((each) => each.ask)]).toEqual([
    [1],
    0,
    [true, false],
    ["root Vault.zip"],
  ]);
});

test("of two imports at once, the page asks before it is left until both have ended", async () => {
  const { store, imports, warned } = storeOf();
  const first = store.startImport(null, new File(["zip"], "A.zip"));
  const second = store.startImport(null, new File(["zip"], "B.zip"));
  await settled();

  imports[0]?.fail(new Error("transfer.busy"));
  await expect(first).rejects.toThrow("transfer.busy");
  expect([store.uploading, warned.at(-1)]).toEqual([1, true]);
  imports[1]?.fail(new Error("transfer.busy"));
  await expect(second).rejects.toThrow("transfer.busy");
  expect([store.uploading, warned.at(-1)]).toEqual([0, false]);
});

test("writing tells whether a job held that writes the tree, an import, a restore or a revert, is queued or running: not an export, not one ended", () => {
  const { store } = storeOf();
  const imported = job(2, { kind: "import" });
  const restore = job(3, { kind: "trash_restore", root_id: "n3" });
  const revert = job(4, { kind: "revert", target_changeset_id: "c4" });
  const cases: [TransferJob[], boolean][] = [
    [[], false],
    [[underWay(job(1), 0, 0), imported, restore, revert], false],
    [[job(1), underWay(imported, 0, 0)], true],
    [[underWay(imported, 1, 4)], true],
    [[underWay(restore, 0, 0)], true],
    [[underWay(restore, 1, 103)], true],
    [[underWay(revert, 0, 0)], true],
    [[underWay(revert, 1, 1501)], true],
    [
      [
        { ...imported, state: "failed" },
        { ...restore, state: "cancelled" },
        { ...revert, state: "failed" },
      ],
      false,
    ],
  ];
  for (const [jobs, writing] of cases) {
    store.jobs = jobs;
    expect(store.writing).toBe(writing);
  }
  // An export running is under way, the list read again for it, but writes no tree.
  store.jobs = [underWay(job(1), 1, 4)];
  expect([store.active, store.writing]).toEqual([true, false]);
});

test("a restore and a revert started in the background go first, once though a read had them, the read on its way dropped; they start one at a time with the exports, the message sent", async () => {
  const { store, lists, starts, restores, reverts } = storeOf();
  await answer(lists, 0, page([job(1)]), store.load());
  const load = store.load();

  const exported = store.start(null);
  const restored = store.startTrashRestore("n7");
  const reverted = store.startRevert("c8", "why");
  await settled();
  expect([starts.length, restores.length, reverts.length]).toEqual([1, 0, 0]);
  await answer(starts, 0, underWay(job(6), 0, 0), exported);
  await settled();
  expect([restores.map((each) => each.ask), reverts.length]).toEqual([["n7"], 0]);
  const restore = underWay(job(7, { kind: "trash_restore", root_id: "n7" }), 0, 0);
  await answer(restores, 0, restore, restored);
  await settled();
  expect(reverts.map((each) => each.ask)).toEqual(["c8 why"]);
  await answer(lists, 1, page([restore, job(1)]), load);
  const revert = underWay(job(8, { kind: "revert", target_changeset_id: "c8" }), 0, 0);
  await answer(reverts, 0, revert, reverted);

  await expect(restored).resolves.toBe(restore);
  await expect(reverted).resolves.toBe(revert);
  expect([ids(store), store.writing]).toEqual([[8, 7, 6, 1], true]);
  void store.startRevert("c9");
  await settled();
  expect(reverts.map((each) => each.ask)).toEqual(["c8 why", "c9 -"]);
});

test("a start refused leaves the jobs held, and the next start goes", async () => {
  const { store, lists, restores } = storeOf();
  await answer(lists, 0, page([job(1)]), store.load());
  const refused = store.startTrashRestore("n7");
  const next = store.startTrashRestore("n8");
  await settled();

  restores[0]?.fail(new Error("transfer.busy"));
  await expect(refused).rejects.toThrow("transfer.busy");
  await settled();
  await answer(restores, 1, underWay(job(8, { kind: "trash_restore", root_id: "n8" }), 0, 0), next);

  expect([restores.map((each) => each.ask), ids(store)]).toEqual([
    ["n7", "n8"],
    [8, 1],
  ]);
});

// M8/P4 design 4.2, review B-M6: a start whose answer was lost finds its job by its kind, its target, who started it
// and when.
test("press holds the browser's time and the jobs held as it is pressed", async () => {
  const { store, lists } = storeOf();
  expect([...store.press().known]).toEqual([]);
  await answer(lists, 0, page([job(2), job(1)]), store.load());
  const before = Date.now();

  const press = store.press();

  expect(press.at).toBeGreaterThanOrEqual(before);
  expect(press.at).toBeLessThanOrEqual(Date.now());
  expect([...press.known]).toEqual([job(2).id, job(1).id]);
});

test("startedJob reads the list again and finds the account's job of the kind on the target that is the press's: one under way, whatever time the server's clock gave it; one ended that the store did not hold at the press, made no earlier than a minute before it; not another kind's, target's or account's", async () => {
  const { store, lists } = storeOf();
  const press = { at: Date.parse("2026-10-06T08:00:00Z"), known: new Set([job(8).id]) };
  const mine = (n: number, more: Partial<TransferJob>) => job(n, { created_at: "2026-10-06T08:00:01Z", ...more });
  const listed = [
    mine(1, { kind: "trash_restore", root_id: "c3" }),
    mine(2, { kind: "revert", target_changeset_id: "c4" }),
    mine(3, { kind: "revert", target_changeset_id: "c3", created_by: bob }),
    // Ended: one the store held at the press; one made more than a minute before it.
    mine(8, { kind: "revert", target_changeset_id: "c3", created_at: "2026-10-06T07:59:50Z" }),
    mine(4, { kind: "revert", target_changeset_id: "c3", created_at: "2026-10-06T07:58:59Z" }),
    // Ended, made by a server's clock 30 seconds behind the browser's.
    mine(5, { kind: "revert", target_changeset_id: "c3", created_at: "2026-10-06T07:59:30Z" }),
    mine(6, { kind: "import", root_id: "n6" }),
    // Under way, made by a server's clock ten minutes behind.
    underWay(mine(7, { kind: "trash_restore", root_id: "n6", created_at: "2026-10-06T07:50:00Z" }), 0, 0),
  ];

  const revert = store.startedJob("revert", "c3", jobJSON(1).created_by.user_id, press);
  await answer(lists, 0, page(listed), revert);
  const restore = store.startedJob("trash_restore", "n6", jobJSON(1).created_by.user_id, press);
  await answer(lists, 1, page(listed), restore);
  const none = store.startedJob("trash_restore", "n9", jobJSON(1).created_by.user_id, press);
  await answer(lists, 2, page(listed), none);

  await expect(revert).resolves.toBe(listed[5]);
  await expect(restore).resolves.toBe(listed[7]);
  await expect(none).resolves.toBeUndefined();
  expect(lists.map((list) => list.ask)).toEqual([undefined, undefined, undefined]);
});

test("startedJob rejects when the list cannot be read: nothing is known", async () => {
  const { store, lists } = storeOf();
  const found = store.startedJob("revert", "c3", jobJSON(1).created_by.user_id, store.press());
  await settled();
  lists[0]?.fail(new TypeError("offline"));

  await expect(found).rejects.toThrow("offline");
});

test("a running job's cancel answered replaces it at once: its cancel asked", async () => {
  const { store, lists, cancels } = storeOf();
  await answer(lists, 0, page([underWay(job(2), 1, 4)]), store.load());

  const cancel = store.cancel(job(2).id);
  await settled();
  await answer(cancels, 0, { ...underWay(job(2), 1, 4), cancel_requested_at: "2026-10-05T09:11:00Z" }, cancel);

  expect(store.jobs?.[0]?.cancel_requested_at).toBe("2026-10-05T09:11:00Z");
});

test("a job cancelled is replaced by the job answered, the read on its way dropped", async () => {
  const { store, lists, cancels } = storeOf();
  await answer(lists, 0, page([underWay(job(2), 0, 0), job(1)]), store.load());
  const load = store.load();

  const cancel = store.cancel(job(2).id);
  await settled();
  await answer(cancels, 0, { ...job(2), state: "cancelled", download: null }, cancel);
  await answer(lists, 1, page([underWay(job(2), 0, 0), job(1)]), load);

  expect(store.jobs?.map((each) => each.state)).toEqual(["cancelled", "succeeded"]);
  expect(store.active).toBe(false);
});

test("exports start one at a time; a job's cancels go one at a time, another job's beside them", async () => {
  const { store, starts, cancels } = storeOf();

  const first = store.start(null);
  const second = store.start("p1");
  const cancels2 = [store.cancel(job(2).id), store.cancel(job(2).id)];
  const cancel3 = store.cancel(job(3).id);
  await settled();
  expect([starts.map((start) => start.ask), cancels.map((cancel) => cancel.ask)]).toEqual([
    [null],
    [job(2).id, job(3).id],
  ]);

  await answer(starts, 0, underWay(job(8), 0, 0), first);
  await answer(cancels, 0, job(2), cancels2[0]);
  await settled();

  expect([starts.map((start) => start.ask), cancels.map((cancel) => cancel.ask)]).toEqual([
    [null, "p1"],
    [job(2).id, job(3).id, job(2).id],
  ]);
  await answer(starts, 1, underWay(job(9), 0, 0), second);
  await answer(cancels, 1, job(3), cancel3);
  await answer(cancels, 2, job(2), cancels2[1]);
});

test("a read that a later one replaced fails quietly: the later one tells", async () => {
  const { store, lists } = storeOf();
  const older = store.load();
  const newer = store.load();

  await answer(lists, 1, page([job(2)]), newer);
  lists[0]?.fail(new TypeError("offline"));

  await expect(older).resolves.toEqual(store.jobs);
  expect(ids(store)).toEqual([2]);
  // The later one's failure is the read's.
  const failing = store.load();
  lists[2]?.fail(new TypeError("offline"));
  await expect(failing).rejects.toThrow("offline");
});

test("a cancel answered once a read found the job ended does not bring it back under way", async () => {
  const { store, lists, cancels } = storeOf();
  await answer(lists, 0, page([underWay(job(2), 1, 4)]), store.load());
  const cancel = store.cancel(job(2).id);
  await settled();

  await answer(lists, 1, page([job(2, { state: "cancelled", download: null })]), store.load());
  await answer(cancels, 0, { ...underWay(job(2), 2, 4), cancel_requested_at: "2026-10-05T09:11:00Z" }, cancel);

  expect([store.jobs?.[0]?.state, store.active]).toEqual(["cancelled", false]);
});

test("of two reads that overlap, the later one asked wins, even answered first", async () => {
  const { store, lists } = storeOf();
  const older = store.load();
  const newer = store.load();

  await answer(lists, 1, page([job(2)]), newer);
  await answer(lists, 0, page([job(1)]), older);

  expect(ids(store)).toEqual([2]);
});

test("detail reads the job from the service, which the store does not hold", async () => {
  const detail = { ...job(1), problems: [], problems_truncated: true, conflicts: [], conflict_count: 0 };
  const { store } = storeOf({ get: async () => detail });

  await expect(store.detail(job(1).id)).resolves.toBe(detail);
  expect(store.jobs).toBeUndefined();
});
