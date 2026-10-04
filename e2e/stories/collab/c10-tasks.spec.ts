import type { Page } from "@nervewiki/api-client";

import { expectToggleRevision } from "../../fixtures/assert/collab";
import { accountIdOf } from "../../fixtures/assert/identity";
import { expectContentWritten } from "../../fixtures/assert/page";
import { displayNameOf, emailFor } from "../../fixtures/auth";
import { answerTo, failedToLoad } from "../../fixtures/browser";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { addNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook } from "../../fixtures/notebooks";
import {
  createPage,
  endSession,
  getView,
  openSession,
  readContent,
  toggleTask,
  writeContent,
} from "../../fixtures/pages";
import { expect, test, watchOf } from "../../fixtures/test";
import { startEditing, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// C10, task items (M5 design 3, 4.12; M5/P6 design 3.3–3.6): a writer
// ticks and clears one in the reading view, the server changing the one
// byte between its brackets, at the position its checkbox carries; a
// reader cannot; the edit lock guards a toggle as it guards a write.

/** offsetOf is the byte position in content of the character between the brackets of the item that starts item. */
function offsetOf(content: string, item: string): number {
  return Buffer.byteLength(content.slice(0, content.indexOf(item))) + 3;
}

test("C10 (API): an editor ticks and clears task items in a content with a byte order mark, CRLF and a decomposed é, that one byte changing, at the position the view's checkbox carries; an item in that state already is the page as it is; a reader is 403; an offset of no item, a negative one and a tick that makes a link reference definition are 422; a base passed is 409 page.revision_mismatch; while A edits, B's toggle and A's own are 409 page.locked naming A, and once A is done B's goes", async ({
  api,
  db,
}, testInfo) => {
  const { adminEmail, adminId, pat: a, workspace } = await newTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const b = await joinAs(api, a, workspace.slug, bEmail, "member");
  const bId = await accountIdOf(db, bEmail);
  const readerEmail = emailFor(testInfo, "reader");
  const reader = await joinAs(api, a, workspace.slug, readerEmail, "guest");
  const notebook = await createNotebook(api, a, workspace.slug, "Plans", "editor");
  expect(
    (await addNotebookMember(api, a, notebook.id, await accountIdOf(db, readerEmail), "reader")).response.status
  ).toBe(201);
  const content = "\ufeff# Cafe\u0301\r\n\r\n- [ ] open\r\n- [X] done\r\n- [ ]: /u\r\n";
  const page = await createPage(api, a, notebook.id, "Tasks", null, content);
  const [open, done, definition] = [
    offsetOf(content, "- [ ] open"),
    offsetOf(content, "- [X] done"),
    offsetOf(content, "- [ ]: /u"),
  ];
  const view = await getView(api, b, page.id);
  for (const [offset, checked] of [
    [open, false],
    [done, true],
    [definition, false],
  ] as const) {
    expect(view.data?.html).toContain(
      `<input ${checked ? 'checked="" ' : ""}disabled="" type="checkbox" data-task="${offset.toString()}">`
    );
  }

  const ticked = await toggleTask(api, b, page.id, { base_revision: 1, offset: open, checked: true });
  expect([ticked.response.status, ticked.data?.revision]).toEqual([200, 2]);
  let expected = "\ufeff# Cafe\u0301\r\n\r\n- [x] open\r\n- [X] done\r\n- [ ]: /u\r\n";
  expect(await readContent(api, b, page.id)).toMatchObject({ content: expected, revision: 2 });
  await expectContentWritten(db, ticked.data as Page, expected, bId);
  await expectToggleRevision(db, page.id, 2, bId, "api");
  const already = await toggleTask(api, b, page.id, { base_revision: 2, offset: done, checked: true });
  expect([already.response.status, already.data?.revision]).toEqual([200, 2]);
  const cleared = await toggleTask(api, b, page.id, { base_revision: 2, offset: done, checked: false });
  expect([cleared.response.status, cleared.data?.revision]).toEqual([200, 3]);
  expected = "\ufeff# Cafe\u0301\r\n\r\n- [x] open\r\n- [ ] done\r\n- [ ]: /u\r\n";
  expect(await readContent(api, b, page.id)).toMatchObject({ content: expected, revision: 3 });
  await expectContentWritten(db, cleared.data as Page, expected, bId);
  await expectToggleRevision(db, page.id, 3, bId, "api");

  const refused = [
    await toggleTask(api, reader, page.id, { base_revision: 3, offset: open, checked: false }),
    await toggleTask(api, b, page.id, { base_revision: 3, offset: open + 1, checked: true }),
    await toggleTask(api, b, page.id, { base_revision: 3, offset: -1, checked: true }),
    await toggleTask(api, b, page.id, { base_revision: 3, offset: definition, checked: true }),
    await toggleTask(api, b, page.id, { base_revision: 2, offset: open, checked: false }),
  ];
  expect(refused.map((r) => [r.response.status, r.error?.code, r.error?.errors?.[0]?.field])).toEqual([
    [403, "forbidden", undefined],
    [422, "validation_failed", "offset"],
    [422, "validation_failed", "offset"],
    [422, "validation_failed", "offset"],
    [409, "page.revision_mismatch", undefined],
  ]);
  expect(await readContent(api, b, page.id)).toMatchObject({ content: expected, revision: 3 });

  const session = await openSession(api, a, page.id);
  const locked = [
    await toggleTask(api, b, page.id, { base_revision: 3, offset: open, checked: false }),
    await toggleTask(api, a, page.id, { base_revision: 3, offset: open, checked: false }),
  ];
  const lock = { page_id: page.id, user_id: adminId, display_name: displayNameOf(adminEmail) };
  expect(locked.map((r) => [r.response.status, r.error?.code, r.error?.lock])).toEqual([
    [409, "page.locked", lock],
    [409, "page.locked", lock],
  ]);
  expect((await endSession(api, a, session.id)).response.status).toBe(204);
  const after = await toggleTask(api, b, page.id, { base_revision: 3, offset: open, checked: false });
  expect([after.response.status, after.data?.revision]).toEqual([200, 4]);
  expect((await readContent(api, a, page.id)).content).toBe(
    "\ufeff# Cafe\u0301\r\n\r\n- [ ] open\r\n- [ ] done\r\n- [ ]: /u\r\n"
  );
});

test("C10 (page): A ticks a task item in the reading view, its checkbox named by its text, the content written and the focus on the same checkbox, and the space key clears it; a reader's checkboxes are disabled; B's click on a revision passed says the page changed; while A edits, B's click is refused as Edit is, the note naming A taking the focus, no alert, the checkbox as it was", async ({
  anotherPage,
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { adminEmail, adminId, pat: a, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const b = await anotherPage(await joinOnboarded(api, a, workspace.slug, emailFor(testInfo, "b"), "member"));
  const readerEmail = emailFor(testInfo, "reader");
  const reader = await anotherPage(await joinOnboarded(api, a, workspace.slug, readerEmail, "guest"));
  const notebook = await createNotebook(api, a, workspace.slug, "Plans", "editor");
  expect(
    (await addNotebookMember(api, a, notebook.id, await accountIdOf(db, readerEmail), "reader")).response.status
  ).toBe(201);
  const content = "- [ ] open\n- [x] done\n";
  const tasks = await createPage(api, a, notebook.id, "Tasks", null, content);
  const path = wikiPagePath(workspace.slug, notebook.id, tasks.id);
  const page = await signedInPage(tokens);
  const boxesOf = (tab: typeof page) => tab.getByRole("main").getByRole("article").getByRole("checkbox");

  await page.goto(path);
  const boxes = boxesOf(page);
  await expect(boxes).toHaveCount(2);
  const toggles = `/api/v0/pages/${tasks.id}/toggle-task`;
  const tick = answerTo(page, "POST", toggles);
  await page.getByRole("main").getByRole("article").getByRole("checkbox", { name: "open", exact: true }).click();

  await expect(boxes.first()).toBeChecked();
  await expect(boxes.first()).toBeFocused();
  expect(await readContent(api, a, tasks.id)).toMatchObject({ content: "- [x] open\n- [x] done\n", revision: 2 });
  await expectContentWritten(db, (await (await tick).json()) as Page, "- [x] open\n- [x] done\n", adminId);
  await expectToggleRevision(db, tasks.id, 2, adminId, "web");
  const clear = answerTo(page, "POST", toggles);
  await page.keyboard.press("Space");
  await expect(boxes.first()).not.toBeChecked();
  await expect(boxes.first()).toBeFocused();
  expect(await readContent(api, a, tasks.id)).toMatchObject({ content, revision: 3 });
  await expectContentWritten(db, (await (await clear).json()) as Page, content, adminId);
  await expectToggleRevision(db, tasks.id, 3, adminId, "web");

  await reader.goto(path);
  await expect(boxesOf(reader)).toHaveCount(2);
  await expect(boxesOf(reader).first()).toBeDisabled();
  await expect(boxesOf(reader).nth(1)).toBeDisabled();

  // B's view is not read again while A writes: B's toggle goes on revision 3, which A's write has passed. The
  // reads let through before the hold, the stream's refresh among them, are answered before A writes: one on its
  // way could show B revision 4.
  const views = `**/api/v0/pages/${tasks.id}/view`;
  let holding = false;
  let released = false;
  let passed = 0;
  let answered = 0;
  let releaseViews: (() => void) | undefined;
  const viewsHeld = new Promise<void>((resolve) => {
    releaseViews = resolve;
  });
  await b.route(views, async (route) => {
    if (released) {
      await route.fallback();
      return;
    }
    const held = holding;
    if (held) {
      await viewsHeld;
    } else {
      passed += 1;
    }
    const answer = await route.fetch();
    if (!held) {
      answered += 1;
    }
    await route.fulfill({ response: answer });
  });
  await b.goto(path);
  await expect(boxesOf(b)).toHaveCount(2);
  holding = true;
  await expect.poll(() => answered).toBe(passed);
  const moved = `- [ ] new\n${content}`;
  await writeContent(api, a, tasks.id, { content: moved, base_revision: 3 });
  const refused = answerTo(b, "POST", toggles);
  await boxesOf(b).first().click();
  expect((await refused).status()).toBe(409);
  released = true;
  releaseViews?.();
  await expect(b.getByRole("main").getByRole("alert")).toHaveText("This page has changed since you read it.");
  await expect(boxesOf(b)).toHaveCount(3);
  // The route stays, letting the views through once released: unrouting the page's last route turns Playwright's
  // interception off, and a request sent in that instant may stay paused, never sent nor failed (as holdContentWrites
  // says).
  expect(await readContent(api, a, tasks.id)).toMatchObject({ content: moved, revision: 4 });

  await startEditing(page);
  await boxesOf(b).nth(1).click();

  const holder = `${displayNameOf(adminEmail)} is editing this page.`;
  await expect(b.getByText(holder, { exact: true })).toBeVisible();
  await expect.poll(() => b.evaluate(() => document.activeElement?.textContent ?? "")).toContain(holder);
  await expect(b.getByRole("main").getByRole("alert")).toHaveCount(0);
  await expect(boxesOf(b).nth(1)).not.toBeChecked();
  expect(await readContent(api, a, tasks.id)).toMatchObject({ content: moved, revision: 4 });
  watchOf(b).expectConsole({ errors: [failedToLoad(409), failedToLoad(409)] });
});
