import { accountIdOf, expectDisplayName } from "../../fixtures/assert/identity";
import { bearer, createToken, displayNameOf, emailFor, register, registerOnboarded } from "../../fixtures/auth";
import { accountMenu } from "../../fixtures/auth-pages";
import { failedToLoad } from "../../fixtures/browser";
import { answerTo, noteOf } from "../../fixtures/settings-pages";
import { expect, test } from "../../fixtures/test";

// A8, the profile and the preferences (M1 design 3); the theme and the language are this browser's (M1/P6 design 3.3).

test("A8 (API): a token changes the display name; a blank or long one answers 422", async ({ api, db }, testInfo) => {
  const email = emailFor(testInfo);
  const tokens = await register(api, email);
  const userId = await accountIdOf(db, email);
  const token = await createToken(api, tokens.access_token, { name: "A8" });
  const updateMe = (body: { display_name?: string }) => api.PATCH("/api/v0/me", { body, headers: bearer(token.token) });

  // The blanks around the name are dropped.
  const changed = await updateMe({ display_name: "  Alice Liddell  " });
  expect(changed.response.status).toBe(200);
  expect(changed.data?.display_name).toBe("Alice Liddell");
  await expectDisplayName(db, userId, "Alice Liddell");

  // A body without a field changes nothing and answers the account.
  const unchanged = await updateMe({});
  expect(unchanged.response.status).toBe(200);
  expect(unchanged.data).toEqual(changed.data);

  const refusals = [
    ["   ", "required"],
    ["a".repeat(101), "too_long"],
    ["Alice\u0007", "invalid_format"],
  ] as const;
  const answers = await Promise.all(refusals.map(([name]) => updateMe({ display_name: name })));
  for (const [i, { response, error }] of answers.entries()) {
    const [name, code] = refusals[i] ?? [];
    expect(response.status, JSON.stringify(name)).toBe(422);
    expect(error?.code).toBe("validation_failed");
    expect(error?.errors?.map((e) => ({ field: e.field, code: e.code }))).toEqual([{ field: "display_name", code }]);
  }
  await expectDisplayName(db, userId, "Alice Liddell");
});

test("A8 (page): the settings change the display name, and this browser's theme and language", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const email = emailFor(testInfo);
  const page = await signedInPage(await registerOnboarded(api, email));
  const userId = await accountIdOf(db, email);
  await page.goto("/settings/profile");
  const name = page.getByLabel("Display name", { exact: true });
  await expect(name).toHaveValue(displayNameOf(email));
  const save = async () => {
    const answer = answerTo(page, "PATCH", "/api/v0/me");
    await page.getByRole("button", { name: "Save" }).click();
    return (await answer).status();
  };

  // One the server refuses shows under the field.
  await name.fill("A".repeat(101));
  expect(await save()).toBe(422);
  await expect.poll(() => noteOf(name)).toBe("At most 100 characters.");
  pageWatch.expectConsole({ errors: [failedToLoad(422)] });

  await name.fill("  Ada Lovelace  ");
  expect(await save()).toBe(200);
  await expect(page.getByRole("status")).toHaveText("Saved.");
  await expect(accountMenu(page, "Ada Lovelace")).toBeVisible();
  await expectDisplayName(db, userId, "Ada Lovelace");

  // The theme and the language apply at once, as the top bar's menus show them.
  const html = page.locator("html");
  await page.getByRole("radio", { name: "Dark" }).check();
  await expect(html).toHaveClass(/\bdark\b/);
  await page.getByRole("radio", { name: "简体中文" }).check();
  await expect(page.getByRole("heading", { level: 1, name: "设置" })).toBeVisible();
  await expect(html).toHaveAttribute("lang", "zh-CN");

  // All of it holds after a reload: the name on the server, the preferences in this browser.
  await page.reload();
  await expect(page.getByRole("heading", { level: 1, name: "设置" })).toBeVisible();
  await expect(html).toHaveClass(/\bdark\b/);
  await expect(page.getByRole("radio", { name: "深色" })).toBeChecked();
  await expect(page.getByLabel("显示名", { exact: true })).toHaveValue("Ada Lovelace");
  await expect(accountMenu(page, "Ada Lovelace")).toBeVisible();
});
