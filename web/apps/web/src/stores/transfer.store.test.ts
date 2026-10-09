import { expect, test } from "vitest";

import type { TransferJob, TransferJobPage, TransferService } from "../services/transfer.service";
import { jobJSON, underWay } from "../test/jobs-server";
import { TransferStore } from "./transfer.store";

const job = (n: number, more: Partial<TransferJob> = {}) => jobJSON(n, more);
const page = (jobs: TransferJob[], next: string | null = null): TransferJobPage => ({ data: jobs, next_cursor: next });

type Service = Pick<TransferService, "list" | "startExport" | "cancel" | "get">;

/**
 * A store of n1's jobs over a service whose every list waits for the test
 * to answer it (lists, in order, with its cursor); a start or a cancel
 * answers what the test gives.
 */
function storeOf(more: Partial<Service> = {}) {
  const lists: { cursor: string | undefined; answer: (page: TransferJobPage) => void }[] = [];
  const started: (string | null)[] = [];
  const store = new TransferStore(
    {
      list: (_notebook, cursor) => new Promise((resolve) => lists.push({ cursor, answer: resolve })),
      startExport: async (_notebook, rootId) => {
        started.push(rootId);
        return underWay(job(9, { root_id: rootId }), 0, 0);
      },
      cancel: async (id) => ({ ...job(0), id, state: "cancelled" }),
      get: async () => {
        throw new Error("not read");
      },
      ...more,
    },
    "n1"
  );
  /** answer answers the list i with answered, and waits for what came of it. */
  const answer = async (i: number, answered: TransferJobPage, done: Promise<unknown>) => {
    lists[i]?.answer(answered);
    await done;
  };
  return { store, lists, started, answer };
}

const ids = (store: TransferStore) => store.jobs?.map((each) => Number(each.id.slice(-2)));

test("the first page read again replaces the jobs it has, as they changed, and keeps the pages added after it", async () => {
  const { store, answer } = storeOf();
  await answer(0, page([underWay(job(3), 1, 4), job(2)], "c2"), store.load());
  await answer(1, page([job(1)]), store.more());

  await answer(2, page([job(3), job(2)], "c2"), store.load());

  expect([ids(store), store.nextCursor]).toEqual([[3, 2, 1], null]);
  expect(store.jobs?.[0]?.state).toBe("succeeded");
});

test.each([
  ["queued", underWay(job(1), 0, 0), true],
  ["running", underWay(job(1), 1, 4), true],
  ["ended", job(1, { state: "failed" }), false],
])("active tells whether a job is under way: %s", async (_, held, active) => {
  const { store, answer } = storeOf();
  expect(store.active).toBe(false);

  await answer(0, page([job(2), held]), store.load());

  expect(store.active).toBe(active);
});

test("more adds the next page's jobs, each once, answers them, and nothing on the last page", async () => {
  const { store, lists, answer } = storeOf();
  await answer(0, page([job(4), job(3)], "c1"), store.load());

  const added = store.more();
  expect(store.more()).toBe(added);
  await answer(1, page([job(3), job(2)], "c2"), added);
  const last = store.more();
  await answer(2, page([job(1)]), last);

  expect((await added).map((each) => each.id)).toEqual([job(2).id]);
  expect([ids(store), store.nextCursor]).toEqual([[4, 3, 2, 1], null]);
  await expect(store.more()).resolves.toEqual([]);
  expect(lists.map((list) => list.cursor)).toEqual([undefined, "c1", "c2"]);
});

test("an export started goes first, the first page on its way, which may not have it, dropped", async () => {
  const { store, started, answer } = storeOf();
  await answer(0, page([job(1)]), store.load());
  const load = store.load();

  const begun = await store.start("p1");
  await answer(1, page([job(1)]), load);

  expect(started).toEqual(["p1"]);
  expect(begun.root_id).toBe("p1");
  expect(ids(store)).toEqual([9, 1]);
  expect(store.active).toBe(true);
});

test("a job cancelled is replaced by the job answered, the first page on its way dropped", async () => {
  const { store, answer } = storeOf();
  await answer(0, page([underWay(job(2), 0, 0), job(1)]), store.load());
  const load = store.load();

  await store.cancel(job(2).id);
  await answer(1, page([underWay(job(2), 0, 0), job(1)]), load);

  expect(store.jobs?.map((each) => each.state)).toEqual(["cancelled", "succeeded"]);
  expect(store.active).toBe(false);
});

test("of two first reads that overlap, the later one asked wins, even answered first", async () => {
  const { store, answer } = storeOf();
  const older = store.load();
  const newer = store.load();

  await answer(1, page([job(2)]), newer);
  await answer(0, page([job(1)]), older);

  expect(ids(store)).toEqual([2]);
});

test("detail reads the job from the service, which the store does not hold", async () => {
  const detail = { ...job(1), problems: [], problems_truncated: true };
  const { store } = storeOf({ get: async () => detail });

  await expect(store.detail(job(1).id)).resolves.toBe(detail);
  expect(store.jobs).toBeUndefined();
});
