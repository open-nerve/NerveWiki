import type { Page } from "@playwright/test";

import { expectIndexedAliases, expectIndexedLinks, expectIndexedTags } from "../../fixtures/assert/links";
import { countAnswers } from "../../fixtures/browser";
import { focusContrast, indicatorContrast, setColorScheme, textContrast } from "../../fixtures/contrast";
import type { Database } from "../../fixtures/db";
import { getPageProperties, listLinkTargets, listTags } from "../../fixtures/links";
import { goto } from "../../fixtures/navigation";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, getView, readContent, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { editorContent, pageHeading, saveEdit, startEditing, wikiPagePath } from "../../fixtures/wiki-pages";
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
  await goto(page, wikiPagePath(workspace.slug, notebook.id, source.id));
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
  // The selected option's detail reads against it, and the editor's amber frame and the option selected show where
  // the focus is, against what is around them, in either theme no fainter than they were (M13/P1/S1; 00-M13 4.7).
  // Floors at what M13/P1/S1 measured, less 0.1, the light amber's at what M13/P1/S2 did once it was a shade deeper
  // (decision 766): no colour is to fade by accident (00-M13 4.7). Measured on Chromium: light, the detail 5.281, the
  // editor's frame and the view's link 3.098, the option selected and the menu's current item, their rings inset,
  // 2.646; dark 4.849, 7.923, 5.476. The search panel's as M13/P4/S3 styled it (decision 910): light, the text of the
  // field focused and a button's word 17.578, of the other field 15.992, a label 6.183; dark 12.987, 12.353, 12.353,
  // 6.637.
  const floors = {
    light: {
      detail: 5.18,
      frame: 2.99,
      selected: 2.54,
      link: 2.99,
      current: 2.54,
      field: 17.47,
      idle: 15.89,
      button: 17.47,
      label: 6.08,
    },
    dark: {
      detail: 4.74,
      frame: 7.82,
      selected: 5.37,
      link: 7.82,
      current: 5.37,
      field: 12.88,
      idle: 12.25,
      button: 12.25,
      label: 6.53,
    },
  } as const;
  const completionShown = async (scheme: "light" | "dark") => {
    await setColorScheme(page, scheme);
    await expect(completion(page)).toBeVisible();
    const detail = await textContrast(completion(page).locator("[aria-selected] .cm-completionDetail"));
    expect(detail.ratio, `${scheme} ${JSON.stringify(detail)}`).toBeGreaterThanOrEqual(floors[scheme].detail);
    const frame = await indicatorContrast(page.locator(".cm-editor.cm-focused"));
    expect(frame.ratio, `${scheme} ${JSON.stringify(frame)}`).toBeGreaterThanOrEqual(floors[scheme].frame);
    const selected = await indicatorContrast(completion(page).locator("li[aria-selected]"));
    expect(selected.ratio, `${scheme} ${JSON.stringify(selected)}`).toBeGreaterThanOrEqual(floors[scheme].selected);
  };
  await completionShown("light");
  await completionShown("dark");
  await setColorScheme(page, "light");
  // Forced colours draw neither the option selected's background nor its ring: it has an outline inside it, which they
  // draw (review B's P4S3B-M4).
  await page.emulateMedia({ forcedColors: "active" });
  const option = completion(page).locator("li[aria-selected]");
  await expect(option).toHaveCSS("outline-style", "solid");
  await expect(option).toHaveCSS("outline-offset", "-2px");
  await page.emulateMedia({ forcedColors: "none" });
  await pick(page);

  // In inline code, between its backticks, nothing completes.
  await page.keyboard.type(" ``");
  await page.keyboard.press("ArrowLeft");
  await page.keyboard.type("[[Tar");
  await page.waitForTimeout(500);
  await expect(completion(page)).toHaveCount(0);

  // The search panel (Mod+F) in the app's colours, light and dark, its field and buttons the app's (decision 910;
  // review B's P4S3B-I1): what is typed in its field, the other field's, not focused, a button's word and a label read
  // against what is under them; a button draws no picture of its own, which the measure would not see; the panel on
  // the editor's background.
  const search = page.locator(".cm-panel.cm-search");
  const searchShown = async (scheme: "light" | "dark") => {
    await setColorScheme(page, scheme);
    const field = await textContrast(search.locator(".cm-textfield").first());
    expect(field.ratio, `${scheme} ${JSON.stringify(field)}`).toBeGreaterThanOrEqual(floors[scheme].field);
    const idle = await textContrast(search.locator(".cm-textfield").nth(1));
    expect(idle.ratio, `${scheme} ${JSON.stringify(idle)}`).toBeGreaterThanOrEqual(floors[scheme].idle);
    const button = search.locator(".cm-button").first();
    await expect(button).toHaveCSS("background-image", "none");
    const word = await textContrast(button);
    expect(word.ratio, `${scheme} ${JSON.stringify(word)}`).toBeGreaterThanOrEqual(floors[scheme].button);
    const label = await textContrast(search.locator("label").first());
    expect(label.ratio, `${scheme} ${JSON.stringify(label)}`).toBeGreaterThanOrEqual(floors[scheme].label);
    const backgrounds = await page
      .locator(".cm-editor")
      .evaluate((editor) =>
        [editor, editor.querySelector(".cm-panels")].map((element) =>
          element === null ? null : getComputedStyle(element).backgroundColor
        )
      );
    expect(backgrounds[1], `${scheme} the panel's background`).toBe(backgrounds[0]);
  };
  await page.keyboard.press("ControlOrMeta+f");
  await expect(search.locator(".cm-textfield").first()).toBeFocused();
  await page.keyboard.type("Target");
  await searchShown("light");
  await searchShown("dark");
  await setColorScheme(page, "light");
  await page.keyboard.press("Escape");
  await expect(search).toHaveCount(0);
  await expect(editorContent(page)).toBeFocused();

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

  // A link of the reading view shows the focus the keyboard gives it, and a menu its current item, in either theme.
  const link = article.getByRole("link", { name: "Target", exact: true });
  const menu = page.getByRole("button", { name: "Page actions", exact: true });
  const readingShown = async (scheme: "light" | "dark") => {
    await setColorScheme(page, scheme);
    await link.focus();
    const ring = await focusContrast(link);
    expect(ring.ratio, `${scheme} ${JSON.stringify(ring)}`).toBeGreaterThanOrEqual(floors[scheme].link);
    await menu.focus();
    await page.keyboard.press("Enter");
    const item = page.getByRole("menuitem").first();
    await expect(item).toBeFocused();
    const current = await indicatorContrast(item);
    expect(current.ratio, `${scheme} ${JSON.stringify(current)}`).toBeGreaterThanOrEqual(floors[scheme].current);
    await page.keyboard.press("Escape");
    // Closed, the menu is gone at once, a frame later: it has no exit's animation, which would keep it mounted, a
    // second menu as another opens (00-M13 737; M13/P1/S2).
    const menus = await page.evaluate(
      () =>
        new Promise<number>((resolve) => {
          requestAnimationFrame(() => resolve(document.querySelectorAll('[role="menu"]').length));
        })
    );
    expect(menus, "menus a frame after Escape").toBe(0);
    await expect(item).toBeHidden();
  };
  await readingShown("light");
  await readingShown("dark");
  await setColorScheme(page, "light");
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
  await goto(page, wikiPagePath(workspace.slug, notebook.id, source.id));
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
  await goto(page, wikiPagePath(workspace.slug, notebook.id, source.id));
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
