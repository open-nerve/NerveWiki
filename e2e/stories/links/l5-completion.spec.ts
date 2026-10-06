import type { Page } from "@playwright/test";

import { createNotebook } from "../../fixtures/notebooks";
import { countAnswers } from "../../fixtures/browser";
import { createPage, readContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, saveEdit, startEditing, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam } from "../../fixtures/workspaces";

// L5, the editor's completion (M6 design 9; M6/P7 design 4, 5): after [[
// the notebook's pages and their aliases, after # its tags; a pick writes
// the link or the tag, which the reading view shows resolved once saved.
// Nothing completes in code.

/** The editor's completion: CodeMirror's list. */
function completion(page: Page) {
  return page.getByRole("listbox", { name: "Completions", exact: true });
}

/** What the completion lists: each option's text, and its detail. */
function completions(page: Page): Promise<string[][]> {
  return completion(page)
    .getByRole("option")
    .evaluateAll((options) =>
      options.map((option) =>
        [".cm-completionLabel", ".cm-completionDetail"].map((part) => option.querySelector(part)?.textContent ?? "")
      )
    );
}

/** Picks the option selected with Enter, once CodeMirror lets a completion just opened be (interactionDelay, 75 ms). */
async function pick(page: Page): Promise<void> {
  await expect(completion(page)).toBeVisible();
  await page.waitForTimeout(150);
  await page.keyboard.press("Enter");
  await expect(completion(page)).toHaveCount(0);
}

test("L5 (page): [[ completes a page by its title and by an alias, # a tag; nothing completes in code; saved, the view shows the links resolved and the tag", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const target = await createPage(api, pat, notebook.id, "Target", null, "---\naliases: [Goal]\n---\n");
  await createPage(api, pat, notebook.id, "Tagged", null, "#project/alpha\n");
  const source = await createPage(api, pat, notebook.id, "Source", null, "");
  const page = await signedInPage(tokens);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, source.id));
  await expect(pageHeading(page, "Source")).toBeVisible();
  await startEditing(page);

  await page.keyboard.type("See [[Tar");
  await expect.poll(() => completions(page)).toEqual([["Target", ""]]);
  await pick(page);
  await page.keyboard.type(" and [[Goa");
  await expect.poll(() => completions(page)).toEqual([["Goal", "→ Target"]]);
  await pick(page);
  await page.keyboard.type(" #proj");
  await expect.poll(() => completions(page)).toEqual([["project/alpha", "1 page"]]);
  await pick(page);

  // In inline code, between its backticks, nothing completes.
  await page.keyboard.type(" ``");
  await page.keyboard.press("ArrowLeft");
  await page.keyboard.type("[[Tar");
  await page.waitForTimeout(500);
  await expect(completion(page)).toHaveCount(0);

  await saveEdit(page);
  expect((await readContent(api, pat, source.id)).content).toBe(
    "See [[Target]] and [[Target|Goal]] #project/alpha `[[Tar`"
  );
  await page.keyboard.press("ControlOrMeta+e");
  const article = page.getByRole("article", { name: "Source" });
  const targetPath = wikiPagePath(workspace.slug, notebook.id, target.id);
  await expect(article.getByRole("link", { name: "Target", exact: true })).toHaveAttribute("href", targetPath);
  await expect(article.getByRole("link", { name: "Goal", exact: true })).toHaveAttribute("href", targetPath);
  await expect(article.getByRole("link", { name: "#project/alpha", exact: true })).toHaveAttribute(
    "href",
    `/${workspace.slug}/notebooks/${notebook.id}/tags/${encodeURIComponent("project/alpha")}`
  );
  await expect(article.locator("code")).toHaveText("[[Tar");
});

test("L5 (page, input method): a composition closes the completion, none opens while it composes, and once it ends the page of its text is listed and picked; one read for the [[", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  await createPage(api, pat, notebook.id, "会议纪要");
  await createPage(api, pat, notebook.id, "Plans");
  const source = await createPage(api, pat, notebook.id, "Source", null, "");
  const page = await signedInPage(tokens);
  const targetReads = countAnswers(page, "GET", `/api/v0/notebooks/${notebook.id}/link-targets`);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, source.id));
  await expect(pageHeading(page, "Source")).toBeVisible();
  await startEditing(page);
  // Chromium's input method, as the DevTools protocol drives it.
  const ime = await page.context().newCDPSession(page);

  await page.keyboard.type("[[");
  await expect.poll(() => completions(page)).toEqual(expect.arrayContaining([["Plans", ""]]));
  for (const text of ["h", "hu", "hui", "huiy", "huiyi"]) {
    // oxlint-disable-next-line no-await-in-loop -- one composition step after another
    await ime.send("Input.imeSetComposition", { text, selectionStart: text.length, selectionEnd: text.length });
    // oxlint-disable-next-line no-await-in-loop -- as above
    await expect(completion(page)).toHaveCount(0);
  }
  // A while for a completion to open, which none does.
  await page.waitForTimeout(500);
  await expect(completion(page)).toHaveCount(0);

  await ime.send("Input.insertText", { text: "会议" });
  await expect.poll(() => completions(page)).toEqual([["会议纪要", ""]]);
  await pick(page);
  await saveEdit(page);
  expect((await readContent(api, pat, source.id)).content).toBe("[[会议纪要]]");
  expect(targetReads()).toBe(1);
});
