import { createClient } from "@nervewiki/api-client";

import { accountIdOf } from "../../fixtures/assert/identity";
import { displayNameOf, emailFor, register } from "../../fixtures/auth";
import { signOutThroughMenu } from "../../fixtures/auth-pages";
import { countAnswers } from "../../fixtures/browser";
import { fetchEvents, followStreams, holdStream, settleEvents } from "../../fixtures/events";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { memberOf, removeMember } from "../../fixtures/members";
import { addedNotebookMember, removeNotebookMember } from "../../fixtures/notebook-members";
import { notebookGroups } from "../../fixtures/notebook-pages";
import { createNotebook, deleteNotebook } from "../../fixtures/notebooks";
import { createPage, writeContent } from "../../fixtures/pages";
import { expect, test, watchOf } from "../../fixtures/test";
import { pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { workspaceHeading } from "../../fixtures/workspace-pages";
import { createWorkspace, newTeam, slugFor } from "../../fixtures/workspaces";

// C8, a stream's life (M5 design 4.10): it ends with a reset whenever what
// it may see or its credential changes, and the client reconnects; one
// reconnected after a change sees what the account sees now. In the browser
// (M5/P3 design 3.6, 3.8), each connection reads again what the tab shows.

test("C8 (API): a stream ends with reset expired as its access token expires", async ({
  db,
  nervewikiWith,
  openEvents,
}, testInfo) => {
  const shortLived = await nervewikiWith(db.url, { env: { NWIKI_AUTH__ACCESS_TOKEN_TTL: "3s" } });
  const session = await register(createClient({ baseUrl: shortLived.baseURL }), emailFor(testInfo));

  const stream = await openEvents(session.access_token, shortLived.baseURL);

  await stream.expectReset("expired");
});

test("C8 (API): B removed from Eng gets reset access; reconnected, B receives nothing of Eng", async ({
  api,
  db,
  nervewiki,
  openEvents,
}, testInfo) => {
  const { pat: a, workspace } = await newTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const b = await joinAs(api, a, workspace.slug, bEmail, "member");
  const eng = await createNotebook(api, a, workspace.slug, "Eng");
  const membership = await addedNotebookMember(api, a, eng.id, await accountIdOf(db, bEmail), "editor");
  const mine = await createNotebook(api, b, workspace.slug, "Mine");
  const control = await createPage(api, b, mine.id, "Control");
  await settleEvents(api, nervewiki.baseURL, a, (await createPage(api, a, eng.id, "Marker")).id);
  const stream = await openEvents(b);

  expect((await removeNotebookMember(api, a, membership.id)).response.status).toBe(204);
  await stream.expectReset("access");

  const reconnected = await openEvents(b);
  await createPage(api, a, eng.id, "Notes");
  await writeContent(api, b, control.id, { content: "# Control\n", base_revision: 1 });
  expect(await reconnected.expectNext("pages")).toMatchObject({ notebook_id: mine.id, pages: [{ id: control.id }] });
});

test("C8 (API): B removed from Acme gets reset access; reconnected, B receives nothing of Acme's notebooks", async ({
  api,
  nervewiki,
  openEvents,
}, testInfo) => {
  const { pat: a, workspace } = await newTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const b = await joinAs(api, a, workspace.slug, bEmail, "member");
  const eng = await createNotebook(api, a, workspace.slug, "Eng", "editor");
  const own = await createWorkspace(api, b, "Own", slugFor(testInfo, "own"));
  const mine = await createNotebook(api, b, own.slug, "Mine");
  const control = await createPage(api, b, mine.id, "Control");
  await settleEvents(api, nervewiki.baseURL, a, (await createPage(api, a, eng.id, "Marker")).id);
  const stream = await openEvents(b);

  expect((await removeMember(api, a, (await memberOf(api, a, workspace.slug, bEmail)).id)).response.status).toBe(204);
  await stream.expectReset("access");

  const reconnected = await openEvents(b);
  await createPage(api, a, eng.id, "Notes");
  await writeContent(api, b, control.id, { content: "# Control\n", base_revision: 1 });
  expect(await reconnected.expectNext("pages")).toMatchObject({ notebook_id: mine.id, pages: [{ id: control.id }] });
});

test("C8 (API): B signs out elsewhere; within a heartbeat, B's stream ends with reset unauthenticated", async ({
  db,
  nervewikiWith,
  openEvents,
}, testInfo) => {
  const beating = await nervewikiWith(db.url, { env: { NWIKI_EVENTS__HEARTBEAT_INTERVAL: "5s" } });
  const api = createClient({ baseUrl: beating.baseURL });
  const session = await register(api, emailFor(testInfo, "b"));
  const stream = await openEvents(session.access_token, beating.baseURL);
  expect(stream.hello).toEqual({ heartbeat_seconds: 5 });

  expect(
    (await api.POST("/api/v0/auth/logout", { body: { refresh_token: session.refresh_token } })).response.status
  ).toBe(204);

  await stream.expectReset("unauthenticated", 5_000 + 2_000);
});

test("C8 (API): Eng deleted, B's stream ends with reset notebooks_deleted", async ({
  api,
  db,
  nervewiki,
  openEvents,
}, testInfo) => {
  const { pat: a, workspace } = await newTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const b = await joinAs(api, a, workspace.slug, bEmail, "member");
  const eng = await createNotebook(api, a, workspace.slug, "Eng");
  await addedNotebookMember(api, a, eng.id, await accountIdOf(db, bEmail), "reader");
  await settleEvents(api, nervewiki.baseURL, a, (await createPage(api, a, eng.id, "Marker")).id);
  const stream = await openEvents(b);

  expect((await deleteNotebook(api, a, eng.id)).response.status).toBe(204);

  await stream.expectReset("notebooks_deleted");
});

test("C8 (API): a stream without a credential is 401", async ({ nervewiki }) => {
  const response = await fetchEvents(nervewiki.baseURL);

  expect(response.status).toBe(401);
  expect(response.headers.get("content-type")).toBe("application/problem+json");
  expect(((await response.json()) as { code: string }).code).toBe("unauthorized");
});

test("C8 (page): with access tokens of 3 s, B's stream ends as each expires and connects again; A's saves still reach B's page", async ({
  db,
  nervewikiWith,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const shortLived = await nervewikiWith(db.url, { env: { NWIKI_AUTH__ACCESS_TOKEN_TTL: "3s" } });
  const api = createClient({ baseUrl: shortLived.baseURL });
  const { pat: a, workspace } = await newTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const page = await signedInPage(await joinOnboarded(api, a, workspace.slug, bEmail, "member"), shortLived.baseURL);
  const streams = followStreams(page.context());
  const eng = await createNotebook(api, a, workspace.slug, "Eng");
  await addedNotebookMember(api, a, eng.id, await accountIdOf(db, bEmail), "reader");
  const notes = await createPage(api, a, eng.id, "Notes", null, "Drafted.\n");
  await page.goto(`${shortLived.baseURL}${wikiPagePath(workspace.slug, eng.id, notes.id)}`);
  const content = page.getByRole("article", { name: "Notes" });
  await expect(content).toHaveText("Drafted.");

  // Two of B's access tokens expire: the stream ends with each, and B's tab connects again with the next.
  await expect.poll(() => streams.opened(), { timeout: 20_000 }).toBeGreaterThanOrEqual(3);
  await writeContent(api, a, notes.id, { content: "Drafted.\n\nPushed.\n", base_revision: 1 });

  await expect(content).toContainText("Pushed.");
  expect(pageWatch.eventStreamErrors).toEqual([]);
});

test("C8 (page): B, reading a page of Eng, is removed from it: the page is not found, and Eng leaves B's notebooks", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { pat: a, workspace } = await newTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const page = await signedInPage(await joinOnboarded(api, a, workspace.slug, bEmail, "member"));
  const eng = await createNotebook(api, a, workspace.slug, "Eng");
  const membership = await addedNotebookMember(api, a, eng.id, await accountIdOf(db, bEmail), "reader");
  const notes = await createPage(api, a, eng.id, "Notes");
  await page.goto(wikiPagePath(workspace.slug, eng.id, notes.id));
  await expect(pageHeading(page, "Notes")).toBeVisible();
  expect(Object.values(await notebookGroups(page, "Acme")).flat()).toContain("Eng");

  expect((await removeNotebookMember(api, a, membership.id)).response.status).toBe(204);

  // What was in Eng is not read again once Eng is gone: nothing answers not found.
  await expect(pageHeading(page, "Page not found")).toBeVisible();
  await expect.poll(async () => Object.values(await notebookGroups(page, "Acme")).flat()).not.toContain("Eng");
  expect(pageWatch.eventStreamErrors).toEqual([]);
});

test("C8 (page): B, reading a page of Acme, is removed from Acme: the workspace is not found, nothing in it read as not found", async ({
  api,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { pat: a, workspace } = await newTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const page = await signedInPage(await joinOnboarded(api, a, workspace.slug, bEmail, "member"));
  const eng = await createNotebook(api, a, workspace.slug, "Eng", "editor");
  const notes = await createPage(api, a, eng.id, "Notes");
  const letStreamIn = await holdStream(page);
  const reads = ["view", "edit-lock", "backlinks", "properties"].map((what) =>
    countAnswers(page, "GET", `/api/v0/pages/${notes.id}/${what}`)
  );
  await page.goto(wikiPagePath(workspace.slug, eng.id, notes.id));
  await expect(pageHeading(page, "Notes")).toBeVisible();
  // B's stream connects once the page shows, and its refresh reads it all again, the page's view, lock, backlinks and
  // properties last: once they have answered, B's tab reads nothing until an event comes. Removed sooner, B would have a read of the page
  // still to come, which the removal answers not found before the stream's reset reaches the tab.
  letStreamIn();
  await expect.poll(() => Math.min(...reads.map((answers) => answers()))).toBeGreaterThanOrEqual(2);

  expect((await removeMember(api, a, (await memberOf(api, a, workspace.slug, bEmail)).id)).response.status).toBe(204);

  // Read again from the outside in: the workspace goes before what is in it would be read, not found.
  await expect(pageHeading(page, "Page not found")).toBeVisible();
  expect(pageWatch.eventStreamErrors).toEqual([]);
});

test("C8 (page): B signs out in a second tab: both tabs sign out, and B's stream closes, not opened again", async ({
  anotherTab,
  api,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { pat: a, workspace } = await newTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const page = await signedInPage(await joinOnboarded(api, a, workspace.slug, bEmail, "member"));
  const streams = followStreams(page.context());
  await page.goto(`/${workspace.slug}`);
  await expect(workspaceHeading(page, "Acme")).toBeVisible();
  const second = await anotherTab(page);
  await second.goto(`/${workspace.slug}`);
  await expect(workspaceHeading(second, "Acme")).toBeVisible();
  await expect.poll(() => streams.count()).toBe(1);

  await signOutThroughMenu(second, displayNameOf(bEmail));

  await Promise.all(
    [page, second].map((tab) => expect(tab.getByRole("heading", { level: 1, name: "Sign in" })).toBeVisible())
  );
  await expect.poll(() => streams.count()).toBe(0);
  // Signed out, no tab asks for a stream again, which would be 401: not at once, nor after the first backoff (1 s).
  const opened = streams.opened();
  await page.waitForTimeout(1_500);
  expect(streams.opened()).toBe(opened);
  expect([pageWatch.eventStreamErrors, watchOf(second).eventStreamErrors]).toEqual([[], []]);
});

test("C8 (page): Eng deleted, it leaves B's notebooks", async ({ api, db, signedInPage }, testInfo) => {
  const { pat: a, workspace } = await newTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const page = await signedInPage(await joinOnboarded(api, a, workspace.slug, bEmail, "member"));
  const eng = await createNotebook(api, a, workspace.slug, "Eng");
  await addedNotebookMember(api, a, eng.id, await accountIdOf(db, bEmail), "reader");
  await page.goto(`/${workspace.slug}`);
  await expect(workspaceHeading(page, "Acme")).toBeVisible();
  await expect.poll(async () => Object.values(await notebookGroups(page, "Acme")).flat()).toContain("Eng");

  expect((await deleteNotebook(api, a, eng.id)).response.status).toBe(204);

  await expect.poll(async () => Object.values(await notebookGroups(page, "Acme")).flat()).not.toContain("Eng");
});
