import type { Page } from "@playwright/test";

import { accountIdOf } from "../../fixtures/assert/identity";
import { emailFor } from "../../fixtures/auth";
import { followStreams } from "../../fixtures/events";
import { joinOnboarded } from "../../fixtures/invitations";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { newTeam } from "../../fixtures/workspaces";

// C9, one stream per browser (M5 design 4.11; M5/P3 design 3.4): the tabs
// of one login elect one that holds the event stream and forwards it to
// the others, with Web Locks, or with a lease in storage where the page has
// none (plain HTTP at a LAN address). Without TLS the browser keeps six
// connections to a server: a stream per tab would take them all.

const tabs = 10;

/** The reading view of Notes in tab. */
function content(tab: Page) {
  return tab.getByRole("article", { name: "Notes" });
}

for (const locks of [true, false]) {
  const how = locks ? "Web Locks" : "the localStorage lease, without navigator.locks";
  test(`C9 (page): ten tabs of B hold one event stream, each reading as usual; the holder closed, another tab holds it within seconds, and A's save reaches every tab (${how})`, async ({
    anotherTab,
    api,
    db,
    signedInPage,
  }, testInfo) => {
    const { pat: a, workspace } = await newTeam(api, testInfo);
    const bEmail = emailFor(testInfo, "b");
    const page = await signedInPage(await joinOnboarded(api, a, workspace.slug, bEmail, "member"));
    const context = page.context();
    if (!locks) {
      await context.addInitScript(() => {
        Reflect.deleteProperty(Navigator.prototype, "locks");
      });
    }
    const streams = followStreams(context);
    const eng = await createNotebook(api, a, workspace.slug, "Eng");
    await addedNotebookMember(api, a, eng.id, await accountIdOf(db, bEmail), "reader");
    const notes = await createPage(api, a, eng.id, "Notes", null, "Drafted.\n");

    // The test's page stays blank: the ten are tabs it can close. They open one after another, each loading
    // while the others hold on.
    const openTab = async () => {
      const tab = await anotherTab(page);
      await tab.goto(wikiPagePath(workspace.slug, eng.id, notes.id));
      await expect(pageHeading(tab, "Notes")).toBeVisible();
      await expect(content(tab)).toHaveText("Drafted.");
      expect(await tab.evaluate(() => "locks" in navigator)).toBe(locks);
      return tab;
    };
    const opened: Page[] = [];
    for (let i = 0; i < tabs; i++) {
      // oxlint-disable-next-line no-await-in-loop -- one tab after another
      opened.push(await openTab());
    }
    await expect.poll(() => streams.count()).toBe(1);

    const [holder] = streams.holders();
    await holder?.close();
    await expect.poll(() => streams.holders().length, { timeout: 5_000 }).toBe(1);
    expect(streams.holders()).not.toContain(holder);

    await writeContent(api, a, notes.id, { content: "Drafted.\n\nPushed.\n", base_revision: 1 });
    await Promise.all(
      opened.filter((each) => !each.isClosed()).map((tab) => expect(content(tab)).toContainText("Pushed."))
    );
    expect(streams.count()).toBe(1);
  });
}
