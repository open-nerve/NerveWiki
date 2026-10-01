import { accountIdOf } from "../../fixtures/assert/identity";
import { expectDeletedWithItsMembers, expectRenamed } from "../../fixtures/assert/workspace";
import { bearer, createToken, emailFor, register } from "../../fixtures/auth";
import { expect, test } from "../../fixtures/test";
import { checkSlug, createWorkspace, deleteWorkspace, renameWorkspace, slugFor } from "../../fixtures/workspaces";

// W4, renaming and deleting a workspace (M2 design 3). What its admin does;
// the other members' 403 comes with invitations (M2/P3).

test("W4 (API): the admin renames the workspace, then deletes it with its members, and its slug is free", async ({
  api,
  db,
}, testInfo) => {
  const email = emailFor(testInfo);
  const session = await register(api, email);
  const pat = (await createToken(api, session.access_token, { name: "W4" })).token;
  const adminId = await accountIdOf(db, email);
  const slug = slugFor(testInfo);
  const created = await createWorkspace(api, pat, "Acme", slug);

  const renamed = await renameWorkspace(api, pat, slug, "  Acme Labs ");
  expect(renamed).toEqual({ ...created, name: "Acme Labs", updated_at: renamed.updated_at });
  expect(new Date(renamed.updated_at).getTime()).toBeGreaterThanOrEqual(new Date(created.updated_at).getTime());
  await expectRenamed(db, renamed, adminId);

  await deleteWorkspace(api, pat, slug);
  await expectDeletedWithItsMembers(db, created.id, adminId);

  // No one sees it, and its slug can name a new workspace at once.
  const gone = await api.GET("/api/v0/workspaces/{slug}", { params: { path: { slug } }, headers: bearer(pat) });
  expect(gone.response.status).toBe(404);
  expect(gone.error?.code).toBe("workspace.not_found");
  expect((await api.GET("/api/v0/workspaces", { headers: bearer(pat) })).data).toEqual({ data: [] });
  expect(await checkSlug(api, pat, slug)).toEqual({ available: true });
  const again = await createWorkspace(api, pat, "Acme again", slug);
  expect(again.id).not.toBe(created.id);
});
