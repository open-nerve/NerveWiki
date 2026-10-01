import type { ApiClient } from "@nervewiki/api-client";

import {
  nervewikiUsers,
  nervewikiUsersFails,
  nervewikiWorkspaces,
  nervewikiWorkspacesFails,
} from "../../fixtures/admin";
import { accountIdOf, expectDeactivated } from "../../fixtures/assert/identity";
import { expectMembership, membershipEndedAt } from "../../fixtures/assert/workspace";
import { bearer, emailFor, registerOnboarded } from "../../fixtures/auth";
import { answerTo, failedToLoad } from "../../fixtures/browser";
import { joinAs } from "../../fixtures/invitations";
import { memberOf, updateMember } from "../../fixtures/members";
import { expect, test } from "../../fixtures/test";
import { createWorkspace, newTeam, slugFor } from "../../fixtures/workspaces";

// W10, deactivation and the workspaces (M2 design 3; M2/P4 design 3.1,
// 3.3): rule two refuses the only admin of a workspace with other members,
// whichever way the account is deactivated, and says which; a deactivation
// ends the account's memberships; the server's administrator brings one
// back with users activate and workspaces reactivate-member.

/** The refusal of rule two for the workspace of slug. */
function soleAdminOf(slug: string): string {
  return `The account is the only admin of workspaces that have other members (${slug}): make another member an admin of each first.`;
}

/** Whether credential still authenticates. */
async function signedIn(api: ApiClient, credential: string): Promise<boolean> {
  return (await api.GET("/api/v0/me", { headers: bearer(credential) })).response.status === 200;
}

test("W10 (API): the only admin's deactivation is refused, naming the workspace; once another member is an admin, it ends every membership", async ({
  api,
  db,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const memberEmail = emailFor(testInfo, "member");
  const member = await joinAs(api, pat, workspace.slug, memberEmail, "member");
  const solo = await createWorkspace(api, pat, "Solo", slugFor(testInfo, "solo"));

  const refused = await api.POST("/api/v0/me/deactivate", { headers: bearer(pat) });
  expect([refused.response.status, refused.error?.code, refused.error?.detail]).toEqual([
    409,
    "workspace.sole_admin",
    soleAdminOf(workspace.slug),
  ]);
  expect(await signedIn(api, pat)).toBe(true);
  await expectMembership(db, workspace.id, adminId, "admin");

  const promoted = await updateMember(api, pat, (await memberOf(api, pat, workspace.slug, memberEmail)).id, "admin");
  expect(promoted.response.status).toBe(200);
  expect((await api.POST("/api/v0/me/deactivate", { headers: bearer(pat) })).response.status).toBe(204);

  await expectDeactivated(db, adminId);
  await expectMembership(db, workspace.id, adminId, "ended");
  await expectMembership(db, solo.id, adminId, "ended");
  expect(await signedIn(api, pat)).toBe(false);
  // The workspace goes on with its new admin.
  const seen = await api.GET("/api/v0/workspaces/{slug}", {
    params: { path: { slug: workspace.slug } },
    headers: bearer(member),
  });
  expect([seen.response.status, seen.data?.role]).toEqual([200, "admin"]);
});

test("W10 (command line): users deactivate refuses the only admin and says why; a member's deactivation ends the membership, and reactivate-member brings it back with its role", async ({
  api,
  db,
}, testInfo) => {
  const { adminEmail, adminId, pat, workspace } = await newTeam(api, testInfo);
  const slug = workspace.slug;
  const memberEmail = emailFor(testInfo, "member");
  const member = await joinAs(api, pat, slug, memberEmail, "member");
  const memberId = await accountIdOf(db, memberEmail);
  const [joined] = await db.query<{ created_at: string }>(
    "SELECT created_at::text FROM workspace_members WHERE workspace_id = $1 AND user_id = $2",
    [workspace.id, memberId]
  );

  await nervewikiUsersFails(db, ["deactivate", "--email", adminEmail], soleAdminOf(slug));
  expect(await signedIn(api, pat)).toBe(true);
  await expectMembership(db, workspace.id, adminId, "admin");

  await nervewikiUsers(db, ["deactivate", "--email", memberEmail]);
  await expectMembership(db, workspace.id, memberId, "ended");
  await nervewikiWorkspacesFails(
    db,
    ["reactivate-member", "--workspace", slug, "--email", memberEmail],
    "This account is deactivated."
  );

  await nervewikiUsers(db, ["activate", "--email", memberEmail]);
  const ended = await membershipEndedAt(db, workspace.id, memberId);
  expect(await nervewikiWorkspaces(db, ["reactivate-member", "--workspace", slug, "--email", memberEmail])).toBe(
    `reactivated ${memberEmail} in ${slug} as member; the membership had ended at ${ended}; ownerless notebooks returned: 0\n`
  );
  await expectMembership(db, workspace.id, memberId, "member", joined?.created_at);
  // The member's personal access token outlived the deactivation: the workspace is theirs again.
  const seen = await api.GET("/api/v0/workspaces/{slug}", { params: { path: { slug } }, headers: bearer(member) });
  expect([seen.response.status, seen.data?.role]).toEqual([200, "member"]);
  expect(await nervewikiWorkspaces(db, ["reactivate-member", "--workspace", slug, "--email", memberEmail])).toBe(
    `${memberEmail} is already a member of ${slug}\n`
  );
});

test("W10 (page): the only admin's deactivation is refused in the dialog, which says why; once another member is an admin, it goes through", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const email = emailFor(testInfo, "admin");
  const tokens = await registerOnboarded(api, email);
  const adminId = await accountIdOf(db, email);
  const workspace = await createWorkspace(api, tokens.access_token, "Acme", slugFor(testInfo));
  const memberEmail = emailFor(testInfo, "member");
  await joinAs(api, tokens.access_token, workspace.slug, memberEmail, "member");
  const page = await signedInPage(tokens);
  await page.goto("/settings/security");

  await page.getByRole("button", { name: "Deactivate account" }).click();
  const dialog = page.getByRole("alertdialog", { name: "Deactivate your account?" });
  const confirm = dialog.getByRole("button", { name: "Deactivate", exact: true });
  pageWatch.expectConsole({ errors: [failedToLoad(409)] });
  const refused = answerTo(page, "POST", "/api/v0/me/deactivate");
  await confirm.click();
  expect((await refused).status()).toBe(409);
  await expect(dialog.getByRole("alert")).toHaveText(
    "You are the only admin of a workspace that has other members. Make another member an admin there first (workspace settings, Members), then deactivate."
  );
  expect(await signedIn(api, tokens.access_token)).toBe(true);
  await expectMembership(db, workspace.id, adminId, "admin");

  // Another member becomes an admin, elsewhere: the dialog, still open, goes through.
  const membership = await memberOf(api, tokens.access_token, workspace.slug, memberEmail);
  expect((await updateMember(api, tokens.access_token, membership.id, "admin")).response.status).toBe(200);
  const deactivated = answerTo(page, "POST", "/api/v0/me/deactivate");
  await confirm.click();
  expect((await deactivated).status()).toBe(204);

  await expect(page.getByRole("heading", { level: 1, name: "Sign in" })).toBeVisible();
  await expectDeactivated(db, adminId);
  await expectMembership(db, workspace.id, adminId, "ended");
});
