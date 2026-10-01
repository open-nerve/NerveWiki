import { accountIdOf } from "../../fixtures/assert/identity";
import { emailFor, signInContext } from "../../fixtures/auth";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { notebookGroups, notebookPath, workspaceNav } from "../../fixtures/notebook-pages";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook, getNotebook, listNotebooks } from "../../fixtures/notebooks";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// N2, a private notebook (M3 design 3): private, then with a second member
// (M3/P2); on the page, it moves from "My notebooks" to the team's (M3/P4
// design 3.3).

test("N2 (API): a private notebook is seen by its members alone, not by the workspace's admin nor its other members", async ({
  api,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const ownerPat = await joinAs(api, adminPat, workspace.slug, emailFor(testInfo, "owner"), "member");
  const otherPat = await joinAs(api, adminPat, workspace.slug, emailFor(testInfo, "other"), "member");
  const notebook = await createNotebook(api, ownerPat, workspace.slug, "Diary");

  expect(await listNotebooks(api, ownerPat, workspace.slug)).toEqual([notebook]);
  const outsiders = [adminPat, otherPat];
  const lists = await Promise.all(outsiders.map((pat) => listNotebooks(api, pat, workspace.slug)));
  expect(lists).toEqual([[], []]);
  const reads = await Promise.all(outsiders.map((pat) => getNotebook(api, pat, notebook.id)));
  expect(reads.map((r) => [r.response.status, r.error?.code])).toEqual([
    [404, "notebook.not_found"],
    [404, "notebook.not_found"],
  ]);
});

test("N2 (API): a private notebook with a second member is listed to both, two members strong, and still to no one else", async ({
  api,
  db,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const ownerPat = await joinAs(api, adminPat, workspace.slug, emailFor(testInfo, "owner"), "member");
  const secondEmail = emailFor(testInfo, "second");
  const secondPat = await joinAs(api, adminPat, workspace.slug, secondEmail, "member");
  const notebook = await createNotebook(api, ownerPat, workspace.slug, "Diary");

  await addedNotebookMember(api, ownerPat, notebook.id, await accountIdOf(db, secondEmail), "editor");

  const [owners, seconds, admins] = await Promise.all([
    listNotebooks(api, ownerPat, workspace.slug),
    listNotebooks(api, secondPat, workspace.slug),
    listNotebooks(api, adminPat, workspace.slug),
  ]);
  expect(owners.map((n) => [n.name, n.workspace_access, n.role, n.member_count])).toEqual([
    ["Diary", "none", "admin", 2],
  ]);
  expect(seconds.map((n) => [n.name, n.workspace_access, n.role, n.member_count])).toEqual([
    ["Diary", "none", "editor", 2],
  ]);
  expect(admins).toEqual([]);
});

test("N2 (page): a private notebook is in no one else's left column, nor found at its address; with a second member, it is both's team notebook", async ({
  api,
  baseURL,
  browser,
  db,
  signedInPage,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const ownerTokens = await joinOnboarded(api, adminPat, workspace.slug, emailFor(testInfo, "owner"), "member");
  const secondEmail = emailFor(testInfo, "second");
  const page = await signedInPage(await joinOnboarded(api, adminPat, workspace.slug, secondEmail, "member"));
  const notebook = await createNotebook(api, ownerTokens.access_token, workspace.slug, "Diary");

  await page.goto(notebookPath(workspace.slug, notebook.id));
  await expect(page.getByRole("heading", { level: 1, name: "Page not found" })).toBeVisible();
  await expect(workspaceNav(page, "Acme").getByText("No notebooks yet.", { exact: true })).toBeVisible();

  // The owner sees it as their own, until the second member comes in.
  const owner = await browser.newContext({ baseURL });
  await signInContext(owner, baseURL ?? "", ownerTokens);
  const ownerPage = await owner.newPage();
  await ownerPage.goto(`/${workspace.slug}`);
  await expect.poll(() => notebookGroups(ownerPage, "Acme")).toEqual({ "My notebooks": ["Diary"] });
  await addedNotebookMember(api, ownerTokens.access_token, notebook.id, await accountIdOf(db, secondEmail), "editor");

  await Promise.all([page, ownerPage].map((tab) => tab.goto(`/${workspace.slug}`)));
  await Promise.all(
    [page, ownerPage].map((tab) =>
      expect.poll(() => notebookGroups(tab, "Acme")).toEqual({ "Team notebooks": ["Diary"] })
    )
  );
  await owner.close();
});
