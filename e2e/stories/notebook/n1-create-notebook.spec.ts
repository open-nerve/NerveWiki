import { accountIdOf } from "../../fixtures/assert/identity";
import { countNotebooks, expectNewNotebook } from "../../fixtures/assert/notebook";
import { bearer, emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook, listNotebooks } from "../../fixtures/notebooks";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// N1, creating a notebook (M3 design 3). The page version comes with M3/P4.

test("N1 (API): a member creates a notebook as its admin, the guest cannot, and a name a file could not take is refused", async ({
  api,
  db,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const memberEmail = emailFor(testInfo, "member");
  const memberPat = await joinAs(api, adminPat, workspace.slug, memberEmail, "member");
  const guestPat = await joinAs(api, adminPat, workspace.slug, emailFor(testInfo, "guest"), "guest");
  const memberId = await accountIdOf(db, memberEmail);

  const created = await createNotebook(api, memberPat, workspace.slug, "  我的 Notes ");
  expect(created).toMatchObject({
    workspace_id: workspace.id,
    name: "我的 Notes",
    workspace_access: "none",
    role: "admin",
    member_count: 1,
  });
  await expectNewNotebook(db, created, memberId);
  // Private with one member: the member's own ("My notebooks").
  expect(await listNotebooks(api, memberPat, workspace.slug)).toEqual([created]);

  // The guest cannot create one; names a file could not take are refused,
  // every problem at once; nothing is added.
  const before = await countNotebooks(db);
  const refused = await api.POST("/api/v0/workspaces/{slug}/notebooks", {
    params: { path: { slug: workspace.slug } },
    body: { name: "Guest's" },
    headers: bearer(guestPat),
  });
  expect(refused.response.status).toBe(403);
  expect(refused.error?.code).toBe("forbidden");
  const invalid = [
    ["Plans/2026", "invalid_format"],
    ["con.txt", "not_allowed"],
    ["名".repeat(86), "too_long"],
  ] as const;
  const answers = await Promise.all(
    invalid.map(([name]) =>
      api.POST("/api/v0/workspaces/{slug}/notebooks", {
        params: { path: { slug: workspace.slug } },
        body: { name },
        headers: bearer(memberPat),
      })
    )
  );
  expect(answers.map((a) => [a.response.status, a.error?.errors?.map((e) => [e.field, e.code])])).toEqual(
    invalid.map(([, code]) => [422, [["name", code]]])
  );
  expect(await countNotebooks(db)).toEqual(before);
});
