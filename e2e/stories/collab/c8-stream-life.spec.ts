import { createClient } from "@nervewiki/api-client";

import { accountIdOf } from "../../fixtures/assert/identity";
import { emailFor, register } from "../../fixtures/auth";
import { fetchEvents } from "../../fixtures/events";
import { joinAs } from "../../fixtures/invitations";
import { addedNotebookMember, removeNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook, deleteNotebook } from "../../fixtures/notebooks";
import { createPage, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// C8, a stream's life (M5 design 4.10): it ends with a reset whenever what
// it may see or its credential changes, and the client reconnects; one
// reconnected after a change sees what the account sees now.

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
  openEvents,
}, testInfo) => {
  const { pat: a, workspace } = await newTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const b = await joinAs(api, a, workspace.slug, bEmail, "member");
  const eng = await createNotebook(api, a, workspace.slug, "Eng");
  const membership = await addedNotebookMember(api, a, eng.id, await accountIdOf(db, bEmail), "editor");
  const mine = await createNotebook(api, b, workspace.slug, "Mine");
  const control = await createPage(api, b, mine.id, "Control");
  const stream = await openEvents(b);

  expect((await removeNotebookMember(api, a, membership.id)).response.status).toBe(204);
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
  openEvents,
}, testInfo) => {
  const { pat: a, workspace } = await newTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const b = await joinAs(api, a, workspace.slug, bEmail, "member");
  const eng = await createNotebook(api, a, workspace.slug, "Eng");
  await addedNotebookMember(api, a, eng.id, await accountIdOf(db, bEmail), "reader");
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
