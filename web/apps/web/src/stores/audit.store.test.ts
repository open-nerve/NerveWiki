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

test("a read again goes back to the first page, and drops a page of the older series still out", async () => {
  let answerMore: ((p: NotebookAuditEventPage) => void) | undefined;
  let first = 0;
  const { store } = storeOf({
    get ""() {
      first += 1;
      return first === 1 ? page(["e3", "e2"], "c1") : page(["e4", "e3"], "c9");
    },
    c1: new Promise<NotebookAuditEventPage>((resolve) => (answerMore = resolve)),
  });
  await store.load();

  const more = store.more();
  await store.load();
  answerMore?.(page(["e1"]));
  await more;

  expect([ids(store), store.nextCursor]).toEqual([["e4", "e3"], "c9"]);
});
