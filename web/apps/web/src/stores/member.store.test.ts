import { expect, test } from "vitest";

import { ApiError } from "../services/api";
import type { WorkspaceMember } from "../services/member.service";
import { MemberStore } from "./member.store";

const member = (id: string, role: WorkspaceMember["role"] = "member"): WorkspaceMember => ({
  id,
  user_id: `user-${id}`,
  role,
  display_name: id,
  email: `${id}@example.com`,
  created_at: "2026-10-01T08:00:00Z",
});

type Service = ConstructorParameters<typeof MemberStore>[0];

/** A store of the workspace acme over a service whose list answers list, and as overrides says. */
function storeOf(list: WorkspaceMember[], overrides: Partial<Service> = {}) {
  return new MemberStore(
    {
      list: async () => list,
      update: async (id, role) => member(id, role),
      remove: async () => {},
      ...overrides,
    },
    "acme"
  );
}

const roles = (store: MemberStore) => store.list?.map((m) => `${m.id}:${m.role}`);

test("the list is the workspace's", async () => {
  const asked: string[] = [];
  const store = storeOf([], {
    list: async (slug) => {
      asked.push(slug);
      return [member("ada", "admin"), member("bob")];
    },
  });

  await store.load();

  expect([asked, roles(store)]).toEqual([["acme"], ["ada:admin", "bob:member"]]);
});

test("a role changed takes its member's place", async () => {
  const store = storeOf([member("ada", "admin"), member("bob"), member("cy")]);
  await store.load();

  await store.changeRole("bob", "guest");

  expect(roles(store)).toEqual(["ada:admin", "bob:guest", "cy:member"]);
});

test("a role change refused leaves the role as it was", async () => {
  const forbidden = new ApiError(403, { status: 403, code: "forbidden", title: "" });
  const store = storeOf([member("ada", "admin"), member("bob")], { update: () => Promise.reject(forbidden) });
  await store.load();

  await expect(store.changeRole("bob", "guest")).rejects.toBe(forbidden);

  expect(roles(store)).toEqual(["ada:admin", "bob:member"]);
});

// SWR reads the list again on focus: a read that went out before a change
// was answered may hold the list from before it.
// A read answered after a write's answer may hold the list from before,
// and must not undo the write. Each write is held to it.
test.each([
  ["a role change", (store: MemberStore) => store.changeRole("bob", "guest"), ["ada:admin", "bob:guest"]],
  ["a removal", (store: MemberStore) => store.remove("bob"), ["ada:admin"]],
])("a read answered after %s keeps it", async (_, write: (store: MemberStore) => Promise<void>, want) => {
  let answerRead: ((list: WorkspaceMember[]) => void) | undefined;
  let reads = 0;
  const store = storeOf([], {
    list: () =>
      ++reads === 1
        ? Promise.resolve([member("ada", "admin"), member("bob")])
        : new Promise<WorkspaceMember[]>((resolve) => (answerRead = resolve)),
  });
  await store.load();

  const read = store.load();
  await write(store);
  answerRead?.([member("ada", "admin"), member("bob")]);
  await read;

  expect(roles(store)).toEqual(want);
});

test("a member removed leaves; one whose membership has ended already leaves as well", async () => {
  const gone = new ApiError(404, { status: 404, code: "workspace.member_not_found", title: "" });
  const forbidden = new ApiError(403, { status: 403, code: "forbidden", title: "" });
  let refusal: ApiError | undefined;
  const store = storeOf([member("ada", "admin"), member("bob"), member("cy"), member("dee")], {
    remove: () => (refusal === undefined ? Promise.resolve() : Promise.reject(refusal)),
  });
  await store.load();

  await store.remove("bob");
  refusal = gone;
  await store.remove("cy");
  expect(roles(store)).toEqual(["ada:admin", "dee:member"]);

  refusal = forbidden;
  await expect(store.remove("dee")).rejects.toBe(forbidden);
  expect(roles(store)).toEqual(["ada:admin", "dee:member"]);
});
