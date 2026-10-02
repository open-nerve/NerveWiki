import { expect, test } from "vitest";

import { ApiError } from "../services/api";
import type { NotebookMember } from "../services/notebook-member.service";
import { NotebookMemberStore } from "./notebook-member.store";

const member = (id: string, joined: string, role: NotebookMember["role"] = "editor"): NotebookMember => ({
  id,
  user_id: `user-${id}`,
  display_name: id,
  email: `${id}@example.com`,
  role,
  created_at: joined,
});

type Service = ConstructorParameters<typeof NotebookMemberStore>[0];

function storeOf(list: NotebookMember[], overrides: Partial<Service> = {}) {
  return new NotebookMemberStore(
    {
      list: async () => list,
      add: async (_notebook, userId, role) => member(userId, "2026-10-02T12:00:00Z", role),
      update: async (id, role) => member(id, "2026-10-02T08:00:00Z", role),
      remove: async () => {},
      ...overrides,
    },
    "n1"
  );
}

const ids = (store: NotebookMemberStore) => store.list?.map((m) => m.id);

test.each([
  ["an addition", (store: NotebookMemberStore) => store.add("carol", "reader"), ["bob", "carol"]],
  ["a role changed", (store: NotebookMemberStore) => store.changeRole("bob", "admin"), ["bob"]],
  ["a removal", (store: NotebookMemberStore) => store.remove("bob"), []],
])(
  "a read answered after %s keeps it",
  async (_, write: (store: NotebookMemberStore) => Promise<unknown>, expected) => {
    let answerRead: ((list: NotebookMember[]) => void) | undefined;
    let reads = 0;
    const store = storeOf([], {
      list: () =>
        ++reads === 1
          ? Promise.resolve([member("bob", "2026-10-02T08:00:00Z")])
          : new Promise<NotebookMember[]>((resolve) => (answerRead = resolve)),
    });
    await store.load();

    const read = store.load();
    await write(store);
    answerRead?.([member("bob", "2026-10-02T08:00:00Z")]);

    expect((await read).map((m) => m.id)).toEqual(expected);
    expect(ids(store)).toEqual(expected);
  }
);

// A membership given back keeps when it was first created (M3/P2 design
// 3.3): the server lists it there, not last.
test("an addition takes its place by when the membership was created, once", async () => {
  const store = storeOf([member("ann", "2026-10-02T08:00:00Z"), member("cid", "2026-10-02T10:00:00.5Z")], {
    add: async (_notebook, userId) =>
      userId === "bob" ? member("bob", "2026-10-02T09:00:00Z") : member(userId, "2026-10-02T10:00:00Z"),
  });
  await store.load();

  await store.add("bob", "reader");
  await store.add("bob", "reader");
  await store.add("dan", "editor");

  expect(ids(store)).toEqual(["ann", "bob", "dan", "cid"]);
});

test("a removal: the membership is gone, and one ended already is gone as well; a refusal keeps it", async () => {
  let refusal: ApiError | undefined;
  const store = storeOf(
    [member("a", "2026-10-02T08:00:00Z"), member("b", "2026-10-02T09:00:00Z"), member("c", "2026-10-02T10:00:00Z")],
    { remove: () => (refusal === undefined ? Promise.resolve() : Promise.reject(refusal)) }
  );
  await store.load();

  await store.remove("a");
  refusal = new ApiError(404, { status: 404, code: "notebook.member_not_found", title: "" });
  await store.remove("b");
  expect(ids(store)).toEqual(["c"]);

  const forbidden = new ApiError(403, { status: 403, code: "forbidden", title: "" });
  refusal = forbidden;
  await expect(store.remove("c")).rejects.toBe(forbidden);
  expect(ids(store)).toEqual(["c"]);
});

test("a role changed takes the answer's place", async () => {
  const store = storeOf([member("a", "2026-10-02T08:00:00Z", "reader")]);
  await store.load();

  await store.changeRole("a", "admin");

  expect(store.list?.map((m) => m.role)).toEqual(["admin"]);
});

// A member's changes go out one at a time (v0.1 design 13.2, item 1): the
// store outlives the members page, whose role menu, mounted anew, may send
// while the change before is out; the server's answers may come back in
// any order (M3 Codex review R1). Another member's go out side by side.
test("a member's changes go out one at a time, the last made last; another member's alongside", async () => {
  const answers: ((m: NotebookMember) => void)[] = [];
  const sent: string[] = [];
  const joined = "2026-10-02T08:00:00Z";
  const store = storeOf([member("ada", joined, "admin"), member("bob", joined)], {
    update: (id, role) => {
      sent.push(`${id}:${role}`);
      return new Promise<NotebookMember>((resolve) => answers.push(resolve));
    },
  });
  await store.load();

  const toReader = store.changeRole("bob", "reader");
  const toAdmin = store.changeRole("bob", "admin");
  const ada = store.changeRole("ada", "editor");
  await Promise.resolve();
  expect(sent).toEqual(["bob:reader", "ada:editor"]);
  answers[0]?.(member("bob", joined, "reader"));
  await toReader;
  await Promise.resolve();
  expect(sent).toEqual(["bob:reader", "ada:editor", "bob:admin"]);
  answers[2]?.(member("bob", joined, "admin"));
  answers[1]?.(member("ada", joined, "editor"));
  await Promise.all([toAdmin, ada]);

  expect(store.list?.map((m) => `${m.id}:${m.role}`)).toEqual(["ada:editor", "bob:admin"]);
});
