import { expectIndexedLinks } from "../../fixtures/assert/links";
import type { Database } from "../../fixtures/db";
import { emailFor } from "../../fixtures/auth";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { getLinkLanding } from "../../fixtures/links";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, getView, listNodes } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, pageTree, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// L2, a link to a page not there (M6 design 9; M6/P6 design 7): a writer
// confirms, and the page is created where the server says the link
// leads, and opened; back on the page the link leads to it. The keyboard
// opens the dialog too, and closing it gives the focus back to the link.
// A reader is told the page is not there, without a question to the
// server. Through the API, a writer reads where the page goes and creates
// it there; a reader may not read it.

/** The links of Source once Next is created where its link leads: the index has it lead there. */
async function expectNextCreated(db: Database, sourceId: string, nextId: string): Promise<void> {
  await expectIndexedLinks(db, sourceId, [
    { kind: "wikilink", property: null, target: "Hub/Next", resolved: nextId },
    { kind: "wikilink", property: null, target: "Elsewhere", resolved: null },
  ]);
}

test("L2 (API): a writer reads where a page made for a link goes, under the path's page, creates it there, and the link leads to it; a reader may not read where it goes", async ({
  api,
  db,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans", "viewer");
  const hub = await createPage(api, pat, notebook.id, "Hub");
  const source = await createPage(api, pat, notebook.id, "Source", null, "[[Hub/Next]] and [[Elsewhere]]\n");

  const elsewhere = await getLinkLanding(api, pat, source.id, "Elsewhere");
  expect([elsewhere.response.status, elsewhere.data]).toEqual([
    200,
    { node_id: null, landing: { parent_id: null, title: "Elsewhere" }, reason: null },
  ]);
  const landing = (await getLinkLanding(api, pat, source.id, "Hub/Next")).data;
  expect(landing).toEqual({ node_id: null, landing: { parent_id: hub.id, title: "Next" }, reason: null });
  const next = await createPage(api, pat, notebook.id, landing?.landing?.title ?? "", landing?.landing?.parent_id);
  expect((await getView(api, pat, source.id)).data?.html).toContain(
    `<a class="nw-wikilink" data-nw-node="${next.id}">Hub/Next</a>`
  );
  await expectNextCreated(db, source.id, next.id);
  // Asked again, the link leads to the page made.
  expect((await getLinkLanding(api, pat, source.id, "Hub/Next")).data).toEqual({
    node_id: next.id,
    landing: null,
    reason: null,
  });

  // A reader may not read where a page would go.
  const reader = await joinAs(api, pat, workspace.slug, emailFor(testInfo, "reader"), "member");
  const refused = await getLinkLanding(api, reader, source.id, "Elsewhere");
  expect([refused.response.status, refused.error?.code]).toEqual([403, "forbidden"]);
});

test("L2 (page): a writer creates the page a link leads to where the server says, once confirmed, and the link leads to it; a reader is told it is not there", async ({
  api,
  db,
  signedInPage,
  anotherPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans", "viewer");
  const hub = await createPage(api, pat, notebook.id, "Hub");
  const source = await createPage(api, pat, notebook.id, "Source", null, "[[Hub/Next]] and [[Elsewhere]]\n");
  const sourcePath = wikiPagePath(workspace.slug, notebook.id, source.id);
  const page = await signedInPage(tokens);
  await page.goto(sourcePath);
  const article = page.getByRole("article");

  // The keyboard opens the dialog; cancelled, the focus is back on the link.
  const elsewhere = article.getByRole("button", { name: "Elsewhere", exact: true });
  await elsewhere.focus();
  await page.keyboard.press("Enter");
  let dialog = page.getByRole("alertdialog", { name: "Create page “Elsewhere”?" });
  await expect(dialog).toContainText("It goes at the notebook's top level.");
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(elsewhere).toBeFocused();

  // Confirmed, the page is created under the path's page, and opened.
  await article.getByRole("button", { name: "Hub/Next", exact: true }).click();
  dialog = page.getByRole("alertdialog", { name: "Create page “Next”?" });
  await expect(dialog).toContainText("It goes under “Hub”.");
  await dialog.getByRole("button", { name: "Create page" }).click();
  await expect(pageHeading(page, "Next")).toBeFocused();
  const next = (await listNodes(api, pat, notebook.id)).find((node) => node.name === "Next");
  expect(next?.parent_id).toBe(hub.id);
  const nextPath = wikiPagePath(workspace.slug, notebook.id, next?.id ?? "");
  await expect(page).toHaveURL(nextPath);
  await expect(pageTree(page, "Plans").getByRole("link", { name: "Next", exact: true })).toBeVisible();

  // Back on the page, the link leads to it.
  await page.goBack();
  await expect(article.getByRole("link", { name: "Hub/Next", exact: true })).toHaveAttribute("href", nextPath);
  await expectNextCreated(db, source.id, next?.id ?? "");

  // A reader is told the page is not there, and asks the server nothing.
  const reader = await anotherPage(
    await joinOnboarded(api, pat, workspace.slug, emailFor(testInfo, "reader"), "member")
  );
  const landings: string[] = [];
  reader.on("request", (request) => {
    if (request.url().includes("/link-landing")) {
      landings.push(request.url());
    }
  });
  await reader.goto(sourcePath);
  const missing = reader.getByRole("article").getByRole("button", { name: "Elsewhere", exact: true });
  await missing.click();
  const notice = reader.getByRole("alertdialog", { name: "“Elsewhere” does not exist" });
  await expect(notice).toContainText("The notebook's editors can create it from this link.");
  await notice.getByRole("button", { name: "OK" }).click();
  await expect(missing).toBeFocused();
  expect(landings).toEqual([]);
});
