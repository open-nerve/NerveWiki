import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook, getNotebook, listNotebooks } from "../../fixtures/notebooks";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// N2, a private notebook (M3 design 3): its private part. The second member
// comes with M3/P2, the page version with M3/P4.

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
