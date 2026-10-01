import { accountIdOf } from "../../fixtures/assert/identity";
import { countNotebooks, expectNewNotebook } from "../../fixtures/assert/notebook";
import { bearer, emailFor } from "../../fixtures/auth";
import { failedToLoad } from "../../fixtures/browser";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import {
  createNotebookWith,
  notebookGroups,
  notebookHeading,
  notebookPath,
  workspaceNav,
} from "../../fixtures/notebook-pages";
import { createNotebook, listNotebooks } from "../../fixtures/notebooks";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// N1, creating a notebook (M3 design 3; M3/P4 design 3.3 for the page).

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

test("N1 (page): a member creates a notebook from the left column and arrives on it, their own; a name a file could not take is refused under the field", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const memberEmail = emailFor(testInfo, "member");
  const page = await signedInPage(await joinOnboarded(api, adminPat, workspace.slug, memberEmail, "member"));
  const memberId = await accountIdOf(db, memberEmail);
  await page.goto(`/${workspace.slug}`);
  await expect(workspaceNav(page, "Acme").getByText("No notebooks yet.", { exact: true })).toBeVisible();

  // The server's rule of a title shows under the name, the dialog open; nothing is added.
  const before = await countNotebooks(db);
  expect((await createNotebookWith(page, workspace, "Plans/2026")).status).toBe(422);
  const dialog = page.getByRole("dialog", { name: "New notebook", exact: true });
  await expect(
    dialog.getByText('Cannot contain / \\ : * ? " < > | # ^ [ ] or control characters, nor start or end with a dot.', {
      exact: true,
    })
  ).toBeVisible();
  await expect(dialog.getByLabel("Name", { exact: true })).toBeFocused();
  pageWatch.expectConsole({ errors: [failedToLoad(422)] });
  expect(await countNotebooks(db)).toEqual(before);
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();

  const { status, created } = await createNotebookWith(page, workspace, "  我的 Notes ");
  expect(status).toBe(201);
  await expect(notebookHeading(page, "我的 Notes")).toBeFocused();
  await expect(page).toHaveURL(notebookPath(workspace.slug, created.id));
  expect(await notebookGroups(page, "Acme")).toEqual({ "My notebooks": ["我的 Notes"] });
  await expectNewNotebook(db, created, memberId);
});

test("N1 (page): a guest is not offered New notebook", async ({ api, signedInPage }, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const page = await signedInPage(
    await joinOnboarded(api, adminPat, workspace.slug, emailFor(testInfo, "guest"), "guest")
  );

  await page.goto(`/${workspace.slug}`);

  await expect(workspaceNav(page, "Acme").getByText("No notebooks yet.", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "New notebook" })).toHaveCount(0);
});
