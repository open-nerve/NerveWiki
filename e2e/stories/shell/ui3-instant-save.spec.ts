import type { Notebook } from "@nervewiki/api-client";
import type { Locator, Page } from "@playwright/test";

import { accountIdOf } from "../../fixtures/assert/identity";
import { expectNotebook } from "../../fixtures/assert/notebook";
import { displayNameOf, emailFor } from "../../fixtures/auth";
import { failedToLoad, noteOf } from "../../fixtures/browser";
import { holdStream } from "../../fixtures/events";
import { joinOnboarded } from "../../fixtures/invitations";
import { who } from "../../fixtures/member-pages";
import { goto } from "../../fixtures/navigation";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { changeAccessWith, changeNotebookRoleWith, notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import { settingsTitle } from "../../fixtures/settings-pages";
import { expect, test } from "../../fixtures/test";
import { newOnboardedTeam } from "../../fixtures/workspaces";

// UI3, the settings saved as they are chosen (M13/P3/S2; 00-M13 4.4.3, 4.8): the notebook's workspace access, a
// select, goes out as it is chosen, its Saved. said with an amber check that fades and fades again the next time;
// refused, the select shows the list's access again and says why. In the three browsers, the page's version only: N3
// has the API's and MCP's. Only selectOption, a choice as the pointer makes it: on Linux, WebKit uses an arrow on a
// closed select at once (M12-polish item 8); the keys' steps and Escape are native-select.test.tsx's and
// general-page.test.tsx's.

/** The general page's access, its select, and what its row says: its own output, not the name's (review C's M4). */
function accessRow(page: Page): { access: Locator; saved: Locator; alert: Locator } {
  const access = page.getByLabel("Workspace access", { exact: true });
  const row = page.locator("form", { has: access });
  return { access, saved: row.getByRole("status"), alert: row.getByRole("alert") };
}

/** The opacity of locator's element as it is computed now. */
function opacityOf(locator: Locator): Promise<string> {
  return locator.evaluate((element) => getComputedStyle(element).opacity);
}

test("UI3 (page): the workspace access goes out as it is chosen, Saved. said with its check, which fades", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { adminId, pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const page = await signedInPage(tokens);
  await goto(page, notebookPath(workspace.slug, notebook.id, "general"));
  await expect(settingsTitle(page, "General")).toBeVisible();
  const { access, saved } = accessRow(page);

  // selectOption does not focus the select (review C's I2): as one who chose it with the pointer would have it.
  await access.focus();
  const viewer = await changeAccessWith(page, notebook.id, "Workspace can read");
  expect(viewer.status()).toBe(200);
  // Once answered: a select disabled while it was out would have lost the focus to the body in Chromium.
  await expect(access).toBeFocused();
  await expect(access).toHaveValue("viewer");
  expect(await noteOf(access)).toBe("The workspace's admins and members can read it; a guest, once added.");
  await expect(saved).toHaveText("Saved.");
  // The check is drawn before the text, not in it (decision 823).
  expect(await saved.evaluate((element) => getComputedStyle(element, "::before").content)).not.toBe("none");
  await expect.poll(() => opacityOf(saved), { timeout: 5_000 }).toBe("0");
  await expect(saved).toHaveText("Saved.");
  await expectNotebook(db, (await viewer.json()) as Notebook, adminId);

  // Chosen again: emptied as it went out, written again in its other round, the fade starts anew (decision 841).
  const editor = await changeAccessWith(page, notebook.id, "Workspace can edit");
  expect(editor.status()).toBe(200);
  await expect(saved).toHaveText("Saved.");
  await expect.poll(() => opacityOf(saved)).toBe("1");
  await expect.poll(() => opacityOf(saved), { timeout: 5_000 }).toBe("0");
  await expect(access).toHaveValue("editor");
  await expectNotebook(db, (await editor.json()) as Notebook, adminId);
});

test("UI3 (page): chosen after another admin made the account an editor, the access is refused: the list's again, and why", async ({
  api,
  db,
  signedInPage,
  anotherPage,
  pageWatch,
}, testInfo) => {
  const { adminEmail, adminId, pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const otherEmail = emailFor(testInfo, "other");
  const otherTokens = await joinOnboarded(api, pat, workspace.slug, otherEmail, "member");
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  await addedNotebookMember(api, pat, notebook.id, await accountIdOf(db, otherEmail), "admin");
  const page = await signedInPage(tokens);
  // The account's stream held: the demotion's event does not reach its page, which still shows the admin's.
  const letStreamIn = await holdStream(page);
  await goto(page, notebookPath(workspace.slug, notebook.id, "general"));
  const { access, saved, alert } = accessRow(page);
  await expect(access).toHaveValue("none");

  // The other admin, in another browser, makes the account an editor (the notebook's roles: admin, editor, reader).
  const other = await anotherPage(otherTokens);
  await goto(other, notebookPath(workspace.slug, notebook.id, "members"));
  expect((await changeNotebookRoleWith(other, who(displayNameOf(adminEmail), adminEmail), "Editor")).status()).toBe(
    200
  );

  await access.focus();
  const refused = await changeAccessWith(page, notebook.id, "Workspace can edit");
  expect(refused.status()).toBe(403);
  await expect(access).toHaveValue("none");
  await expect(access).toBeFocused();
  await expect(alert).toHaveText("Only the notebook's admins can change its workspace access.");
  await expect(saved).toHaveText("");
  pageWatch.expectConsole({ errors: [failedToLoad(403)] });
  await expectNotebook(db, notebook, adminId);
  letStreamIn();
});
