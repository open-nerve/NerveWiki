import { accountIdOf } from "../../fixtures/assert/identity";
import {
  expectDeletedWithItsMembers,
  expectInvitationsDeletedWith,
  expectRenamed,
} from "../../fixtures/assert/workspace";
import { bearer, createToken, emailFor, register, registerOnboarded } from "../../fixtures/auth";
import { expect, test } from "../../fixtures/test";
import { accept, invite, joinAs, preview } from "../../fixtures/invitations";
import { deleteWorkspaceWith, renameWorkspaceWith, switcher, workspaceHeading } from "../../fixtures/workspace-pages";
import {
  checkSlug,
  createWorkspace,
  deleteWorkspace,
  newTeam,
  renameWorkspace,
  slugFor,
} from "../../fixtures/workspaces";

// W4, renaming and deleting a workspace (M2 design 3): what its admin does,
// and what the other members cannot (M2/P3), on the general settings page
// too (M2/P5 design 3.6).

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

test("W4 (API): a member or a guest cannot rename or delete; the deletion takes the pending invitations", async ({
  api,
  db,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const slug = workspace.slug;
  const others = [
    await joinAs(api, pat, slug, emailFor(testInfo, "member"), "member"),
    await joinAs(api, pat, slug, emailFor(testInfo, "guest"), "guest"),
  ];
  const pending = await invite(api, pat, slug, emailFor(testInfo, "invitee"));

  const refusals = await Promise.all(
    others.flatMap((credential) => [
      api.PATCH("/api/v0/workspaces/{slug}", {
        params: { path: { slug } },
        body: { name: "Mine" },
        headers: bearer(credential),
      }),
      api.DELETE("/api/v0/workspaces/{slug}", { params: { path: { slug } }, headers: bearer(credential) }),
    ])
  );
  expect(refusals.map((r) => [r.response.status, r.error?.code])).toEqual(refusals.map(() => [403, "forbidden"]));

  await deleteWorkspace(api, pat, slug);
  await expectDeletedWithItsMembers(db, workspace.id, adminId);
  await expectInvitationsDeletedWith(db, workspace.id, 1);
  const link = await preview(api, pending);
  expect([link.response.status, link.error?.code]).toEqual([404, "workspace.invitation_not_found"]);
});

test("W4 (page): the admin renames the workspace, then deletes it once its slug is typed, and lands on another", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const email = emailFor(testInfo);
  const tokens = await registerOnboarded(api, email);
  const adminId = await accountIdOf(db, email);
  const slug = slugFor(testInfo);
  const created = await createWorkspace(api, tokens.access_token, "Acme", slug);
  const beta = await createWorkspace(api, tokens.access_token, "Beta", slugFor(testInfo, "beta"));
  const page = await signedInPage(tokens);

  await page.goto(`/${slug}/settings`);
  await expect(page).toHaveURL(`/${slug}/settings/general`);
  const { status, renamed } = await renameWorkspaceWith(page, slug, "  Acme Labs ");
  expect(status).toBe(200);
  await expect(page.getByText("Saved.", { exact: true })).toBeVisible();
  await expect(switcher(page, "Acme Labs")).toBeVisible();
  expect(renamed).toEqual({ ...created, name: "Acme Labs", updated_at: renamed.updated_at });
  await expectRenamed(db, renamed, adminId);

  expect(await deleteWorkspaceWith(page, slug)).toBe(204);
  await expect(workspaceHeading(page, "Beta")).toBeVisible();
  await expect(page).toHaveURL(`/${beta.slug}`);
  await expectDeletedWithItsMembers(db, created.id, adminId);
  expect(await checkSlug(api, tokens.access_token, slug)).toEqual({ available: true });
});

test("W4 (page): a member sees the workspace's name and address, and no way to change them", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const email = emailFor(testInfo, "member");
  const tokens = await registerOnboarded(api, email);
  await accept(api, tokens.access_token, await invite(api, pat, workspace.slug, email, "member"));
  const page = await signedInPage(tokens);

  await page.goto(`/${workspace.slug}/settings/general`);

  await expect(page.getByText("Only the workspace's admins can rename it.", { exact: true })).toBeVisible();
  await expect(page.getByText(workspace.slug, { exact: true })).toBeVisible();
  await expect(page.getByRole("textbox")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Delete workspace" })).toHaveCount(0);
});
