import { aliveSessionsOf } from "../../fixtures/assert/collab";
import { expectOneSessionRevision } from "../../fixtures/assert/page";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, readContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { contentWrites, editStatus, startEditing, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam } from "../../fixtures/workspaces";

// C5, autosave (M5 design 4.8; M5/P5 design 3.4): what is typed is saved
// once it rests about 2 seconds, one changeset for the session; an input
// method's composition holds the save until it ends; Ctrl+S saves at once.
// A page version only: it is the browser's behaviour (M5 design 3). The
// page's clock is paused once the editor is open: autosave's 2 seconds
// pass as the story runs them.

test("C5 (page): A's typing is saved once it rests 2 seconds, no key pressed, and typing on stays one changeset; while the input method composes nothing is saved however long it rests, and its end saves the word; Ctrl+S saves at once", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const notes = await createPage(api, pat, notebook.id, "Notes", null, "Drafted.\n");
  const page = await signedInPage(tokens);
  const writes = contentWrites(page, notes.id);
  await page.clock.install();

  await page.goto(wikiPagePath(workspace.slug, notebook.id, notes.id));
  await startEditing(page);
  await page.keyboard.press("ControlOrMeta+End");
  await page.clock.pauseAt(await page.evaluate(() => Date.now() + 1_000));
  await page.keyboard.type("One");
  await page.clock.runFor(1_900);
  await expect(editStatus(page)).toHaveText("Unsaved changes");
  expect(await writes.all()).toEqual([]);
  await page.clock.runFor(100);
  await expect(editStatus(page)).toHaveText("Saved.");
  expect(await writes.all()).toEqual([{ status: 200 }]);
  const [session] = await aliveSessionsOf(db, notes.id);

  await page.keyboard.type(" two");
  await expect(editStatus(page)).toHaveText("Unsaved changes");
  await page.clock.runFor(2_000);
  await expect(editStatus(page)).toHaveText("Saved.");
  expect(await writes.all()).toHaveLength(2);
  // Both saves are the session's one version, from the revision it began on.
  await expectOneSessionRevision(db, session ?? "", notes.id, 1, (await writes.saved()).revision);

  const cdp = await page.context().newCDPSession(page);
  await cdp.send("Input.imeSetComposition", { text: "ni", selectionStart: 2, selectionEnd: 2 });
  await page.clock.runFor(10_000);
  await expect(editStatus(page)).toHaveText("Unsaved changes");
  expect(await writes.all()).toHaveLength(2);
  await cdp.send("Input.insertText", { text: "你" });
  // The composition's end settles, then the save that waited for it goes.
  await page.clock.runFor(100);
  await expect(editStatus(page)).toHaveText("Saved.");
  expect(await writes.all()).toHaveLength(3);
  expect((await readContent(api, pat, notes.id)).content).toBe("Drafted.\nOne two你");
  await page.clock.runFor(2_000);
  expect(await writes.all()).toHaveLength(3);

  await page.keyboard.type(" three");
  await expect(editStatus(page)).toHaveText("Unsaved changes");
  await page.keyboard.press("ControlOrMeta+s");
  await expect(editStatus(page)).toHaveText("Saved.");
  expect(await writes.all()).toHaveLength(4);
  await expectOneSessionRevision(db, session ?? "", notes.id, 1, (await writes.saved()).revision);
});
