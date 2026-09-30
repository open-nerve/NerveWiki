import { expect, test } from "vitest";

import { ApiError } from "../services/api";
import type { ApiToken, ApiTokenCreated } from "../services/api-token.service";
import { ApiTokenStore } from "./api-token.store";

const token = (id: string, name = id): ApiToken => ({
  id,
  name,
  expires_at: null,
  last_used_at: null,
  created_at: "2026-10-01T08:00:00Z",
});
const created = (id: string): ApiTokenCreated => ({ ...token(id), token: `nwk_pat_${id}` });
const notFound = new ApiError(404, { status: 404, code: "identity.api_token_not_found", title: "" });

// SWR reads the list again on focus, and a creation may go out at the same
// moment: a read answered after it may hold the list from before, and must
// not take the new token away.
test("a read answered after a creation keeps the new token", async () => {
  let answerRead: ((tokens: ApiToken[]) => void) | undefined;
  let reads = 0;
  const store = new ApiTokenStore({
    list: () =>
      ++reads === 1 ? Promise.resolve([token("old")]) : new Promise<ApiToken[]>((resolve) => (answerRead = resolve)),
    create: async () => created("new"),
    revoke: async () => {},
  });
  await store.load();

  const read = store.load();
  await store.create({ name: "new", current_password: "pw" });
  answerRead?.([token("old")]);

  expect((await read).map((t) => t.id)).toEqual(["new", "old"]);
  expect(store.tokens?.map((t) => t.id)).toEqual(["new", "old"]);
});

test("the list gets a new token first, without the token itself", async () => {
  const store = new ApiTokenStore({
    list: async () => [token("old")],
    create: async () => created("new"),
    revoke: async () => {},
  });
  await store.load();

  const answer = await store.create({ name: "new", current_password: "pw" });

  expect(answer.token).toBe("nwk_pat_new");
  expect(store.tokens).toEqual([token("new"), token("old")]);
  expect(JSON.stringify(store.tokens)).not.toContain("nwk_pat_");
});

test("a revoked token leaves the list, one gone already too; any other refusal keeps it", async () => {
  let refusal: unknown = notFound;
  const store = new ApiTokenStore({
    list: async () => [token("a"), token("b")],
    create: async () => created("c"),
    revoke: (id) => (id === "a" ? Promise.resolve() : Promise.reject(refusal)),
  });
  await store.load();

  await store.revoke("a");
  expect(store.tokens?.map((t) => t.id)).toEqual(["b"]);

  refusal = new ApiError(503, { status: 503, code: "server_busy", title: "" });
  await expect(store.revoke("b")).rejects.toBe(refusal);
  expect(store.tokens?.map((t) => t.id)).toEqual(["b"]);

  refusal = notFound;
  await store.revoke("b");
  expect(store.tokens).toEqual([]);
});
