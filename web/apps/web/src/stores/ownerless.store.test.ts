import { expect, test } from "vitest";

import { ApiError } from "../services/api";
import type { OwnerlessNotebook } from "../services/ownerless.service";
import { notebookJSON, ownerlessJSON } from "../test/fakes";
import { OwnerlessStore } from "./ownerless.store";

const ownerless = (id: string): OwnerlessNotebook => ({ ...ownerlessJSON, id, name: id });

type Service = ConstructorParameters<typeof OwnerlessStore>[0];

/** A store of lab's ownerless notebooks over a service whose list answers list, and as overrides says. */
function storeOf(list: OwnerlessNotebook[], overrides: Partial<Service> = {}) {
  return new OwnerlessStore(
    {
      list: async () => list,
      takeOver: async (id) => ({ ...notebookJSON, id }),
      remove: async () => {},
      ...overrides,
    },
    "lab"
  );
}

const ids = (store: OwnerlessStore) => store.list?.map((n) => n.id);
const notFound = new ApiError(404, { status: 404, code: "notebook.not_found", title: "" });
const forbidden = new ApiError(403, { status: 403, code: "forbidden", title: "" });

test.each([
  ["taken over", (store: OwnerlessStore) => store.takeOver("a")],
  ["deleted", (store: OwnerlessStore) => store.remove("a")],
])("a notebook %s is off the list, and a read answered after keeps it off", async (_, write) => {
  let answerRead: ((list: OwnerlessNotebook[]) => void) | undefined;
  let reads = 0;
  const store = storeOf([], {
    list: () =>
      ++reads === 1
        ? Promise.resolve([ownerless("a"), ownerless("b")])
        : new Promise<OwnerlessNotebook[]>((resolve) => (answerRead = resolve)),
  });
  await store.load();

  const read = store.load();
  await write(store);
  answerRead?.([ownerless("a"), ownerless("b")]);

  expect((await read).map((n) => n.id)).toEqual(["b"]);
  expect(ids(store)).toEqual(["b"]);
});

test("taking over answers the notebook as the account now sees it", async () => {
  const store = storeOf([ownerless("a")]);
  await store.load();

  expect(await store.takeOver("a")).toMatchObject({ id: "a", role: "admin" });
});

// Ownerless no more (taken over, deleted or returned since): it is off the
// list, and what was asked did not happen, which the page says.
test.each([
  ["taking over", (store: OwnerlessStore) => store.takeOver("a")],
  ["deleting", (store: OwnerlessStore) => store.remove("a")],
])("%s a notebook ownerless no more takes it off, and throws", async (_, write) => {
  const store = storeOf([ownerless("a"), ownerless("b")], {
    takeOver: () => Promise.reject(notFound),
    remove: () => Promise.reject(notFound),
  });
  await store.load();

  await expect(write(store)).rejects.toBe(notFound);
  expect(ids(store)).toEqual(["b"]);
});

test("a refusal of another kind keeps the notebook, and throws", async () => {
  const store = storeOf([ownerless("a")], { takeOver: () => Promise.reject(forbidden) });
  await store.load();

  await expect(store.takeOver("a")).rejects.toBe(forbidden);
  expect(ids(store)).toEqual(["a"]);
});

test("a notebook's changes go out one at a time", async () => {
  const sent: string[] = [];
  let answer: (() => void) | undefined;
  const store = storeOf([ownerless("a")], {
    takeOver: (id) => {
      sent.push("take over");
      return new Promise((resolve) => (answer = () => resolve({ ...notebookJSON, id })));
    },
    remove: async () => void sent.push("delete"),
  });
  await store.load();

  const taking = store.takeOver("a");
  const deleting = store.remove("a");
  await Promise.resolve();
  expect(sent).toEqual(["take over"]);
  answer?.();
  await Promise.all([taking, deleting]);
  expect(sent).toEqual(["take over", "delete"]);
});
