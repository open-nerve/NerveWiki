import {
  accountIdOf,
  countIdentity,
  expectNewToken,
  expectNothingAdded,
  expectTokenRevoked,
  expectTokenUsed,
} from "../../fixtures/assert/identity";
import type { ApiTokenCreated } from "@nervewiki/api-client";
import type { Page } from "@playwright/test";

import { bearer, createToken, emailFor, password, register, registerOnboarded } from "../../fixtures/auth";
import { answerTo, failedToLoad, noteOf } from "../../fixtures/browser";
import { createTokenWith, holdAnswer } from "../../fixtures/settings-pages";
import { expect, test } from "../../fixtures/test";

// A10, personal access tokens (M1 design 3).

test("A10 (API): a token asks for the password, is shown once, is listed without itself and stops when revoked", async ({
  api,
  db,
}, testInfo) => {
  const email = emailFor(testInfo);
  const session = await register(api, email);
  const userId = await accountIdOf(db, email);
  const list = async (credential: string) => {
    const { response, data } = await api.GET("/api/v0/me/api-tokens", { headers: bearer(credential) });
    expect(response.status).toBe(200);
    return data?.data ?? [];
  };
  const status = async (credential: string) =>
    (await api.GET("/api/v0/me", { headers: bearer(credential) })).response.status;

  // A wrong password creates nothing.
  const before = await countIdentity(db);
  const wrong = await api.POST("/api/v0/me/api-tokens", {
    body: { name: "CI", current_password: "Wr0ng-password" },
    headers: bearer(session.access_token),
  });
  expect(wrong.response.status).toBe(422);
  expect(wrong.error?.code).toBe("identity.current_password_incorrect");
  await expectNothingAdded(db, before);

  const created = await createToken(api, session.access_token, { name: " CI deploy " });
  expect(created.name).toBe("CI deploy");
  expect(created.expires_at).toBeNull();
  await expectNewToken(db, userId, created);

  // The list has the token without the token itself, never used so far.
  expect(await list(session.access_token)).toEqual([
    { id: created.id, name: "CI deploy", expires_at: null, last_used_at: null, created_at: created.created_at },
  ]);

  // A token acts as the account, and creates tokens itself; the newest is listed first.
  expect(await status(created.token)).toBe(200);
  await expectTokenUsed(db, created.id);
  const second = await createToken(api, created.token, { name: "backup" });
  await expectNewToken(db, userId, second);
  const listed = await list(second.token);
  expect(listed.map((t) => t.id)).toEqual([second.id, created.id]);
  expect(listed[1]?.last_used_at).not.toBeNull();
  expect(JSON.stringify(listed)).not.toContain(created.token);

  // Another account cannot see or revoke it.
  const stranger = await register(api, emailFor(testInfo, "stranger"));
  expect(await list(stranger.access_token)).toEqual([]);
  const foreign = await api.DELETE("/api/v0/api-tokens/{token_id}", {
    params: { path: { token_id: created.id } },
    headers: bearer(stranger.access_token),
  });
  expect(foreign.response.status).toBe(404);
  expect(foreign.error?.code).toBe("identity.api_token_not_found");

  // Revoked, by the other token, it stops at once; revoking again finds nothing.
  const revoke = () =>
    api.DELETE("/api/v0/api-tokens/{token_id}", {
      params: { path: { token_id: created.id } },
      headers: bearer(second.token),
    });
  expect((await revoke()).response.status).toBe(204);
  await expectTokenRevoked(db, created.id);
  expect(await status(created.token)).toBe(401);
  const again = await revoke();
  expect(again.response.status).toBe(404);
  expect(again.error?.code).toBe("identity.api_token_not_found");
  expect((await list(second.token)).map((t) => t.id)).toEqual([second.id]);
});

test("A10 (API): a token stops when it expires", async ({ api, db }, testInfo) => {
  const email = emailFor(testInfo);
  const session = await register(api, email);

  // The past is refused.
  const past = await api.POST("/api/v0/me/api-tokens", {
    body: { name: "late", expires_at: new Date(Date.now() - 60_000).toISOString(), current_password: password },
    headers: bearer(session.access_token),
  });
  expect(past.response.status).toBe(422);
  expect(past.error?.errors?.map((e) => ({ field: e.field, code: e.code }))).toEqual([
    { field: "expires_at", code: "out_of_range" },
  ]);

  const expiresAt = new Date(Date.now() + 3_600_000);
  const created = await createToken(api, session.access_token, { name: "short", expires_at: expiresAt.toISOString() });
  await expectNewToken(db, await accountIdOf(db, email), created);
  const status = async () => (await api.GET("/api/v0/me", { headers: bearer(created.token) })).response.status;
  expect(await status()).toBe(200);

  // Its hour passes: the row moves two hours back, so that the server's own clock finds it expired, with no
  // waiting that a slow machine could outrun. The exact boundary is the unit tests' (authenticate_pat_test.go, a fixed clock).
  await db.query(
    "UPDATE api_tokens SET created_at = created_at - interval '2 hours', expires_at = expires_at - interval '2 hours' WHERE id = $1",
    [created.id]
  );
  expect(await status()).toBe(401);
});

/** Everywhere a token could be left behind in page: the document, its storage, its address. */
async function expectTokenGone(page: Page, token: string): Promise<void> {
  expect(await page.content()).not.toContain(token);
  const stored = await page.evaluate(() =>
    JSON.stringify([Object.entries(localStorage), Object.entries(sessionStorage)])
  );
  expect(stored).not.toContain(token);
  expect(page.url()).not.toContain(token);
}

test("A10 (page): a token asks for the password, is shown once, shows when it was used and stops when revoked", async ({
  api,
  db,
  context,
  pageWatch,
  signedInPage,
}, testInfo) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  const email = emailFor(testInfo);
  const page = await signedInPage(await registerOnboarded(api, email));
  const userId = await accountIdOf(db, email);
  const console: string[] = [];
  page.on("console", (message) => void console.push(message.text()));
  await page.goto("/settings/tokens");
  await expect(page.getByText("You have no access tokens.")).toBeVisible();

  // A wrong password shows under it, and creates nothing.
  const before = await countIdentity(db);
  expect((await createTokenWith(page, { name: "CI", password: "Wr0ng-password" })).status()).toBe(422);
  const dialog = page.getByRole("dialog", { name: "Create an access token" });
  await expect
    .poll(() => noteOf(dialog.getByLabel("Current password", { exact: true })))
    .toBe("The current password is incorrect.");
  await expectNothingAdded(db, before);
  pageWatch.expectConsole({ errors: [failedToLoad(422)] });
  await dialog.getByRole("button", { name: "Cancel" }).click();

  // The token is shown this once, to copy; after Done it is nowhere.
  const answer = await createTokenWith(page, { name: "CI deploy", password, expiry: "In a year" });
  expect(answer.status()).toBe(201);
  const created = (await answer.json()) as ApiTokenCreated;
  await expectNewToken(db, userId, created);
  const shown = page.getByRole("dialog", { name: "Your new token" });
  await expect(shown.getByLabel("Token", { exact: true })).toHaveValue(created.token);
  await shown.getByRole("button", { name: "Copy" }).click();
  await expect(shown.getByRole("status")).toHaveText("Copied.");
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(created.token);
  await shown.getByRole("button", { name: "Done" }).click();
  await expect(shown).toBeHidden();
  const row = page.getByRole("listitem").filter({ hasText: "CI deploy" });
  await expect(row).toContainText("Never used");
  await expectTokenGone(page, created.token);
  await page.reload();
  await expect(row).toContainText("Never used");
  await expectTokenGone(page, created.token);
  expect(console.filter((text) => text.includes(created.token))).toEqual([]);

  // Used once, the list says when (to the minute).
  expect((await api.GET("/api/v0/me", { headers: bearer(created.token) })).response.status).toBe(200);
  await expectTokenUsed(db, created.id);
  await page.reload();
  await expect(row).toContainText("Last used");

  // Revoked, it leaves the list and stops at once.
  await page.getByRole("button", { name: "Revoke CI deploy" }).click();
  const revoked = answerTo(page, "DELETE", `/api/v0/api-tokens/${created.id}`);
  await page.getByRole("alertdialog", { name: "Revoke CI deploy?" }).getByRole("button", { name: "Revoke" }).click();
  expect((await revoked).status()).toBe(204);
  await expect(page.getByText("You have no access tokens.")).toBeVisible();
  await expectTokenRevoked(db, created.id);
  expect((await api.GET("/api/v0/me", { headers: bearer(created.token) })).response.status).toBe(401);
});

// Cancelled while the creation is out, the dialog is gone when the token
// arrives: the page shows it nowhere, and the list has it, to revoke.
test("A10 (page): a creation answered after the dialog was cancelled shows its token nowhere", async ({
  api,
  signedInPage,
}, testInfo) => {
  const page = await signedInPage(await registerOnboarded(api, emailFor(testInfo)));
  await page.goto("/settings/tokens");
  await expect(page.getByText("You have no access tokens.")).toBeVisible();
  const release = await holdAnswer(page, "POST", "/api/v0/me/api-tokens");

  const answer = createTokenWith(page, { name: "late", password });
  const dialog = page.getByRole("dialog", { name: "Create an access token" });
  await expect(dialog.getByRole("button", { name: "Create", exact: true })).toBeDisabled();
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).toBeHidden();
  release();

  const created = (await (await answer).json()) as ApiTokenCreated;
  await expect(page.getByRole("listitem").filter({ hasText: "late" })).toBeVisible();
  await expectTokenGone(page, created.token);
  await page.getByRole("button", { name: "Create token" }).click();
  await expect(
    page.getByRole("dialog", { name: "Create an access token" }).getByLabel("Name", { exact: true })
  ).toHaveValue("");
  await expectTokenGone(page, created.token);
});
