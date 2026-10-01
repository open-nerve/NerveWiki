import { accountIdOf } from "../../fixtures/assert/identity";
import { countWorkspaces, expectNewWorkspace, expectNoWorkspaceAdded } from "../../fixtures/assert/workspace";
import { bearer, createToken, emailFor, register } from "../../fixtures/auth";
import { expect, test } from "../../fixtures/test";
import { checkSlug, createWorkspace, slugFor } from "../../fixtures/workspaces";

// W1, creating a workspace (M2 design 3).

test("W1 (API): a workspace is created with the caller as its admin, its slug checked first", async ({
  api,
  db,
}, testInfo) => {
  const email = emailFor(testInfo);
  const session = await register(api, email);
  const pat = (await createToken(api, session.access_token, { name: "W1" })).token;
  const userId = await accountIdOf(db, email);
  const slug = slugFor(testInfo);

  expect(await checkSlug(api, pat, slug)).toEqual({ available: true });
  expect(await checkSlug(api, pat, "Acme")).toEqual({ available: false, reason: "invalid" });
  expect(await checkSlug(api, pat, "settings")).toEqual({ available: false, reason: "reserved" });

  const created = await createWorkspace(api, pat, "  Acme 研发 ", slug);
  expect(created).toMatchObject({ slug, name: "Acme 研发", role: "admin" });
  await expectNewWorkspace(db, created, userId);
  expect(await checkSlug(api, pat, slug)).toEqual({ available: false, reason: "taken" });

  // The slug taken, a reserved one: nothing is added.
  const before = await countWorkspaces(db);
  const taken = await api.POST("/api/v0/workspaces", { body: { name: "Other", slug }, headers: bearer(pat) });
  expect(taken.response.status).toBe(409);
  expect(taken.error?.code).toBe("workspace.slug_taken");
  const reserved = await api.POST("/api/v0/workspaces", { body: { name: "Other", slug: "api" }, headers: bearer(pat) });
  expect(reserved.response.status).toBe(422);
  expect(reserved.error?.errors).toEqual([{ field: "slug", code: "not_allowed", message: "is reserved" }]);
  await expectNoWorkspaceAdded(db, before);

  // The workspace reads back as it was created, and is the account's one.
  const got = await api.GET("/api/v0/workspaces/{slug}", { params: { path: { slug } }, headers: bearer(pat) });
  expect(got.data).toEqual(created);
  const list = await api.GET("/api/v0/workspaces", { headers: bearer(pat) });
  expect(list.data).toEqual({ data: [created] });

  // Another account does not see it.
  const other = await register(api, emailFor(testInfo, "other"));
  const hidden = await api.GET("/api/v0/workspaces/{slug}", {
    params: { path: { slug } },
    headers: bearer(other.access_token),
  });
  expect(hidden.response.status).toBe(404);
  expect(hidden.error?.code).toBe("workspace.not_found");
});
