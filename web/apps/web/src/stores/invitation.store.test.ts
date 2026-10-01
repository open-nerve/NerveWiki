import { expect, test } from "vitest";

import { ApiError } from "../services/api";
import type { WorkspaceInvitation } from "../services/invitation.service";
import { InvitationStore } from "./invitation.store";

const invitation = (id: string): WorkspaceInvitation => ({
  id,
  email: `${id}@example.com`,
  role: "member",
  token: `nwk_inv_${id}`,
  created_at: "2026-10-01T08:00:00Z",
});

type Service = ConstructorParameters<typeof InvitationStore>[0];

/** A store of the workspace acme over a service whose list answers list, and as overrides says. */
function storeOf(list: WorkspaceInvitation[], overrides: Partial<Service> = {}) {
  return new InvitationStore(
    {
      list: async () => list,
      create: async (_slug, body) => ({ ...invitation(body.email.split("@")[0] ?? ""), role: body.role }),
      remove: async () => {},
      ...overrides,
    },
    "acme"
  );
}

const ids = (store: InvitationStore) => store.list?.map((i) => i.id);

test("the list is the workspace's, and an invitation sent goes first", async () => {
  const asked: string[] = [];
  const store = storeOf([], {
    list: async (slug) => {
      asked.push(slug);
      return [invitation("old")];
    },
    create: async (slug, body) => {
      asked.push(slug);
      return { ...invitation("new"), email: body.email, role: body.role };
    },
  });
  await store.load();

  const sent = await store.invite({ email: "new@example.com", role: "guest" });

  expect([asked, ids(store), sent.role]).toEqual([["acme", "acme"], ["new", "old"], "guest"]);
});

// A read that goes out after the creation was committed, and comes back
// before the creation's answer, holds the new invitation already.
test("a read that holds an invitation being sent keeps it once", async () => {
  let answerCreate: ((created: WorkspaceInvitation) => void) | undefined;
  let reads = 0;
  const store = storeOf([], {
    list: async () => (++reads === 1 ? [invitation("old")] : [invitation("new"), invitation("old")]),
    create: () => new Promise<WorkspaceInvitation>((resolve) => (answerCreate = resolve)),
  });
  await store.load();

  const sending = store.invite({ email: "new@example.com", role: "member" });
  await store.load();
  answerCreate?.(invitation("new"));
  await sending;

  expect(ids(store)).toEqual(["new", "old"]);
});

// A read answered after a write's answer may hold the list from before,
// and must not undo the write. Each write is held to it.
test.each([
  [
    "an invitation",
    (store: InvitationStore) => store.invite({ email: "c@example.com", role: "member" }),
    ["c", "a", "b"],
  ],
  ["a withdrawal", (store: InvitationStore) => store.withdraw("a"), ["b"]],
])("a read answered after %s keeps it", async (_, write: (store: InvitationStore) => Promise<unknown>, want) => {
  let answerRead: ((list: WorkspaceInvitation[]) => void) | undefined;
  let reads = 0;
  const store = storeOf([], {
    list: () =>
      ++reads === 1
        ? Promise.resolve([invitation("a"), invitation("b")])
        : new Promise<WorkspaceInvitation[]>((resolve) => (answerRead = resolve)),
  });
  await store.load();

  const read = store.load();
  await write(store);
  answerRead?.([invitation("a"), invitation("b")]);
  await read;

  expect(ids(store)).toEqual(want);
});

test("an invitation withdrawn leaves; one gone already leaves as well", async () => {
  const gone = new ApiError(404, { status: 404, code: "workspace.invitation_not_found", title: "" });
  const forbidden = new ApiError(403, { status: 403, code: "forbidden", title: "" });
  let refusal: ApiError | undefined;
  const store = storeOf([invitation("a"), invitation("b"), invitation("c")], {
    remove: () => (refusal === undefined ? Promise.resolve() : Promise.reject(refusal)),
  });
  await store.load();

  await store.withdraw("a");
  refusal = gone;
  await store.withdraw("b");
  expect(ids(store)).toEqual(["c"]);

  refusal = forbidden;
  await expect(store.withdraw("c")).rejects.toBe(forbidden);
  expect(ids(store)).toEqual(["c"]);
});
