import { expect, test } from "vitest";

import type { TransferJob, TransferJobPage, TransferService } from "../services/transfer.service";
import { jobJSON, underWay } from "../test/jobs-server";
import { TransferStore } from "./transfer.store";

const job = (n: number, more: Partial<TransferJob> = {}) => jobJSON(n, more);
const page = (jobs: TransferJob[], next: string | null = null): TransferJobPage => ({ data: jobs, next_cursor: next });

type Service = Pick<TransferService, "list" | "startExport" | "cancel" | "get">;

/** A request of the store's service held until the test answers it. */
type Held<A, T> = { ask: A; answer: (answered: T) => void; fail: (error: unknown) => void };

/**
 * A store of n1's jobs over a service whose every list, start and cancel
 * waits for the test to answer it: lists, starts and cancels have them in
 * order, with their cursor, root and id.
 */
function storeOf(more: Partial<Service> = {}) {
  const lists: Held<string | undefined, TransferJobPage>[] = [];
  const starts: Held<string | null, TransferJob>[] = [];
  const cancels: Held<string, TransferJob>[] = [];
  const store = new TransferStore(
    {
      list: (_notebook, cursor) =>
        new Promise((resolve, reject) => lists.push({ ask: cursor, answer: resolve, fail: reject })),
      startExport: (_notebook, rootId) =>
        new Promise((resolve, reject) => starts.push({ ask: rootId, answer: resolve, fail: reject })),
      cancel: (id) => new Promise((resolve, reject) => cancels.push({ ask: id, answer: resolve, fail: reject })),
      get: () => Promise.reject(new Error("not read")),
      ...more,
    },
    "n1"
  );
  return { store, lists, starts, cancels };
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
  const detail = { ...job(1), problems: [], problems_truncated: true };
  const { store } = storeOf({ get: async () => detail });

  await expect(store.detail(job(1).id)).resolves.toBe(detail);
  expect(store.jobs).toBeUndefined();
});
