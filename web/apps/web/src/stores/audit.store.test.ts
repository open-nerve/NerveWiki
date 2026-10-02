import { expect, test } from "vitest";

import type { NotebookAuditEvent, NotebookAuditEventPage } from "../services/ownerless.service";
import { auditEventJSON } from "../test/fakes";
import { AuditStore } from "./audit.store";

const event = (id: string): NotebookAuditEvent => ({ ...auditEventJSON, id });
const page = (ids: string[], next: string | null = null): NotebookAuditEventPage => ({
  data: ids.map(event),
  next_cursor: next,
});

/** A store of lab's audit events over a service that answers each page by its cursor in pages ("" the first). */
function storeOf(pages: Record<string, NotebookAuditEventPage | Promise<NotebookAuditEventPage>>) {
  const asked: string[] = [];
  const store = new AuditStore(
    {
      auditEvents: async (_slug, cursor) => {
        asked.push(cursor ?? "");
        const answer = pages[cursor ?? ""];
        if (answer === undefined) {
          throw new Error(`no page ${cursor}`);
        }
        return answer;
      },
    },
    "lab"
  );
  return { store, asked };
}

const ids = (store: AuditStore) => store.events?.map((e) => e.id);

test("more adds the next page's events, each once, until the last page", async () => {
  const { store, asked } = storeOf({ "": page(["e5", "e4"], "c1"), c1: page(["e4", "e3"], "c2"), c2: page(["e2"]) });
  await store.load();

  await store.more();
  await store.more();
  await store.more();

  expect([ids(store), store.nextCursor]).toEqual([["e5", "e4", "e3", "e2"], null]);
  expect(asked).toEqual(["", "c1", "c2"]);
});

test("more asked twice while its page is out reads it once", async () => {
  const { store, asked } = storeOf({ "": page(["e2"], "c1"), c1: page(["e1"]) });
  await store.load();

  await Promise.all([store.more(), store.more()]);

  expect([ids(store), asked]).toEqual([
    ["e2", "e1"],
    ["", "c1"],
  ]);
});

/** A store whose service's every read waits for the test to answer it: asks has them in order, by cursor. */
function heldStore() {
  const asks: { cursor: string | undefined; answer: (page: NotebookAuditEventPage) => void }[] = [];
  const store = new AuditStore(
    {
      auditEvents: (_slug, cursor) => new Promise((resolve) => asks.push({ cursor, answer: resolve })),
    },
    "lab"
  );
  /** answer answers the read i with page, and waits for what came of it. */
  const answer = async (i: number, answered: NotebookAuditEventPage, done: Promise<unknown>) => {
    asks[i]?.answer(answered);
    await done;
  };
  return { store, asks, answer };
}

/** A store holding [e4, e3], whose next page's cursor is c3. */
async function holding() {
  const held = heldStore();
  await held.answer(0, page(["e4", "e3"], "c3"), held.store.load());
  return held;
}

test("a read again whose first page reaches the events held keeps the ones past it, and their cursor", async () => {
  const { store, answer } = await holding();
  await answer(1, page(["e2"], "c2"), store.more());

  // An event came since: the first page now ends at e4, which is held.
  await answer(2, page(["e5", "e4"], "c4"), store.load());

  expect([ids(store), store.nextCursor]).toEqual([["e5", "e4", "e3", "e2"], "c2"]);
});

test.each([
  ["does not reach the events held", page(["e9", "e8"], "c8"), [["e9", "e8"], "c8"]],
  ["is the last page", page(["e5", "e4", "e3"]), [["e5", "e4", "e3"], null]],
])("a read again whose first page %s replaces them", async (_, first, want) => {
  const { store, answer } = await holding();
  await answer(1, page(["e2"], "c2"), store.more());

  await answer(2, first, store.load());

  expect([ids(store), store.nextCursor]).toEqual(want);
});

// A page asked for while the first is read again (M3/P5 review M2): it is
// added when it still follows the events held, whichever answers first.
test.each([
  [
    "the read again answers first, reaching the events held",
    "load",
    page(["e5", "e4"], "c4"),
    [["e5", "e4", "e3", "e2"], null],
  ],
  ["the read again answers first, past the events held", "load", page(["e9", "e8"], "c8"), [["e9", "e8"], "c8"]],
  [
    "the page answers first, the read again reaching it",
    "more",
    page(["e5", "e4"], "c4"),
    [["e5", "e4", "e3", "e2"], null],
  ],
  ["the page answers first, the read again past it", "more", page(["e9", "e8"], "c8"), [["e9", "e8"], "c8"]],
] as const)("more while the first page is read again: %s", async (_, firstAnswered, first, want) => {
  const { store, asks, answer } = await holding();
  const load = store.load();
  const more = store.more();
  expect(asks.map((ask) => ask.cursor)).toEqual([undefined, undefined, "c3"]);

  const answerLoad = () => answer(1, first, load);
  const answerMore = () => answer(2, page(["e2"]), more);
  if (firstAnswered === "load") {
    await answerLoad();
    await answerMore();
  } else {
    await answerMore();
    await answerLoad();
  }

  expect([ids(store), store.nextCursor]).toEqual(want);
});

test("once a read again replaced the events, more reads the page after them, not the one still out for the older", async () => {
  const { store, asks, answer } = await holding();
  const older = store.more();
  await answer(2, page(["e9", "e8"], "c8"), store.load());

  const more = store.more();
  expect(asks.map((ask) => ask.cursor)).toEqual([undefined, "c3", undefined, "c8"]);
  await answer(1, page(["e2"]), older);
  await answer(3, page(["e7"]), more);

  expect([ids(store), store.nextCursor]).toEqual([["e9", "e8", "e7"], null]);
});

test("of two first reads that overlap, the later one asked wins, even answered first", async () => {
  const { store, answer } = await holding();
  const older = store.load();
  const newer = store.load();

  await answer(2, page(["e9", "e8"], "c8"), newer);
  await answer(1, page(["e4", "e3"], "c3"), older);

  expect([ids(store), store.nextCursor]).toEqual([["e9", "e8"], "c8"]);
});
