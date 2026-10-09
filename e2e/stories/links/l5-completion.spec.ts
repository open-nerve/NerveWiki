import type { Page } from "@playwright/test";

import { expectIndexedAliases, expectIndexedLinks, expectIndexedTags } from "../../fixtures/assert/links";
import { countAnswers } from "../../fixtures/browser";
import type { Database } from "../../fixtures/db";
import { getPageProperties, listLinkTargets, listTags } from "../../fixtures/links";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, getView, readContent, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, saveEdit, startEditing, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// L5, the editor's completion (M6 design 9; M6/P7 design 4, 5): after [[
// the notebook's pages and their aliases, after # its tags; a pick writes
// the link or the tag, which the reading view shows resolved once saved.
// Nothing completes in code. Through the API, what the completion reads,
// and a content written with what it lists; the completion itself, its
// list, keys and input method, is the page's alone.

/** What Source is written with, the page version's picks: a link, one by an alias, a tag; a link in code. */
const written = "See [[Target]] and [[Target|Goal]] #project/alpha `[[Tar`";

/** Source written so, as the index has it: both links lead to Target, whose alias is Goal; the tag. */
async function expectWritten(db: Database, sourceId: string, targetId: string): Promise<void> {
  await expectIndexedLinks(db, sourceId, [
    { kind: "wikilink", property: null, target: "Target", resolved: targetId },
    { kind: "wikilink", property: null, target: "Target", resolved: targetId },
  ]);
  await expectIndexedTags(db, sourceId, [{ tag: "project/alpha", count: 1 }]);
  await expectIndexedAliases(db, targetId, ["Goal"]);
}

test("L5 (API): what [[ and # complete with, the notebook's pages with their links (by path where two share a title) and aliases and its tags with their pages; a content written with them shows the links resolved and the tag", async ({
  api,
  db,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const target = await createPage(api, pat, notebook.id, "Target", null, "---\naliases: [Goal]\n---\n");
  const tagged = await createPage(api, pat, notebook.id, "Tagged", null, "#project/alpha\n");
  const source = await createPage(api, pat, notebook.id, "Source", null, "");
  // Two pages of one title: each is written by its path.
  const archive = await createPage(api, pat, notebook.id, "Archive");
  const drafts = await createPage(api, pat, notebook.id, "Drafts");
  const archived = await createPage(api, pat, notebook.id, "Notes", archive.id);
  const drafted = await createPage(api, pat, notebook.id, "Notes", drafts.id);

  const targets = await listLinkTargets(api, pat, notebook.id);
  expect([targets.response.status, targets.data?.data.toSorted((a, b) => a.link.localeCompare(b.link))]).toEqual([
    200,
    [
      { id: archive.id, kind: "page", name: "Archive", link: "Archive", aliases: [] },
      { id: archived.id, kind: "page", name: "Notes", link: "Archive/Notes", aliases: [] },
      { id: drafts.id, kind: "page", name: "Drafts", link: "Drafts", aliases: [] },
      { id: drafted.id, kind: "page", name: "Notes", link: "Drafts/Notes", aliases: [] },
      { id: source.id, kind: "page", name: "Source", link: "Source", aliases: [] },
      { id: tagged.id, kind: "page", name: "Tagged", link: "Tagged", aliases: [] },
      { id: target.id, kind: "page", name: "Target", link: "Target", aliases: ["Goal"] },
    ],
  ]);
  expect((await listTags(api, pat, notebook.id)).data?.data).toEqual([{ tag: "project/alpha", count: 1 }]);

  await writeContent(api, pat, source.id, { content: written, base_revision: 1 });
  const html = (await getView(api, pat, source.id)).data?.html ?? "";
  for (const element of [
    `<a class="nw-wikilink" data-nw-node="${target.id}">Target</a>`,
    `<a class="nw-wikilink" data-nw-node="${target.id}">Goal</a>`,
    '<a class="nw-tag" data-nw-tag="project/alpha">#project/alpha</a>',
    "<code>[[Tar</code>",
  ]) {
    expect(html).toContain(element);
  }
  await expectWritten(db, source.id, target.id);
});

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
  db,
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
  // A screen reader hears the option's text and its detail apart.
  await expect(completion(page).getByRole("option", { name: "project/alpha, 1 page", exact: true })).toBeVisible();
  await pick(page);

  // In inline code, between its backticks, nothing completes.
  await page.keyboard.type(" ``");
  await page.keyboard.press("ArrowLeft");
  await page.keyboard.type("[[Tar");
  await page.waitForTimeout(500);
  await expect(completion(page)).toHaveCount(0);

  await saveEdit(page);
  expect((await readContent(api, pat, source.id)).content).toBe(written);
  await expectWritten(db, source.id, target.id);
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
  // What the composition's text matches: a completion that opened while it composes would list it.
  await createPage(api, pat, notebook.id, "Huiyi notes");
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

test("L5 (page, frontmatter): a page and an alias picked in a property's quotes are written as its YAML string writes them; saved, the frontmatter is valid, its values as picked, its links leading to the page (Codex review R1)", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const target = await createPage(
    api,
    pat,
    notebook.id,
    "Bob's",
    null,
    "---\naliases: ['He said \"Hi\"', 'C:\\x']\n---\n"
  );
  const source = await createPage(api, pat, notebook.id, "Source", null, "");
  const page = await signedInPage(tokens);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, source.id));
  await expect(pageHeading(page, "Source")).toBeVisible();
  await startEditing(page);

  await page.keyboard.type("---\nref: '[[Bo");
  await expect.poll(() => completions(page)).toEqual([["Bob's", ""]]);
  await pick(page);
  await page.keyboard.type("'\nalt: \"[[He");
  await expect.poll(() => completions(page)).toEqual([['He said "Hi"', "→ Bob's"]]);
  await pick(page);
  await page.keyboard.type('"\nwin: "[[C:');
  await expect.poll(() => completions(page)).toEqual([[String.raw`C:\x`, "→ Bob's"]]);
  await pick(page);
  await page.keyboard.type('"\n---\n');
  await saveEdit(page);

  expect((await readContent(api, pat, source.id)).content).toBe(
    "---\nref: '[[Bob''s]]'\n" +
      String.raw`alt: "[[Bob's|He said \"Hi\"]]"` +
      "\n" +
      String.raw`win: "[[Bob's|C:\\x]]"` +
      "\n---\n"
  );
  expect((await getPageProperties(api, pat, source.id)).data).toEqual({
    valid: true,
    assets_expire_at: null,
    properties: [
      { key: "ref", value: "[[Bob's]]" },
      { key: "alt", value: `[[Bob's|He said "Hi"]]` },
      { key: "win", value: String.raw`[[Bob's|C:\x]]` },
    ],
    links: [
      { key: "ref", node_id: target.id, kind: "page", url: null, inline: null },
      { key: "alt", node_id: target.id, kind: "page", url: null, inline: null },
      { key: "win", node_id: target.id, kind: "page", url: null, inline: null },
    ],
  });
  await expectIndexedLinks(
    db,
    source.id,
    ["ref", "alt", "win"].map((property) => ({ kind: "wikilink", property, target: "Bob's", resolved: target.id }))
  );
});
