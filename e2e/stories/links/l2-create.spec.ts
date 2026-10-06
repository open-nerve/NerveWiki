import { emailFor } from "../../fixtures/auth";
import { joinOnboarded } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, listNodes } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, pageTree, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam } from "../../fixtures/workspaces";

// L2, a link to a page not there (M6 design 9; M6/P6 design 7): a writer
// confirms, and the page is created where the server says the link
// leads, and opened; back on the page the link leads to it. The keyboard
// opens the dialog too, and closing it gives the focus back to the link.
// A reader is told the page is not there, without a question to the
// server.

test("L2 (page): a writer creates the page a link leads to where the server says, once confirmed, and the link leads to it; a reader is told it is not there", async ({
  api,
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
