import { accountIdOf, expectDisplayName, expectOnboardingSteps } from "../../fixtures/assert/identity";
import { bearer, createToken, displayNameOf, emailFor, register } from "../../fixtures/auth";
import { failedToLoad } from "../../fixtures/browser";
import { displayNameField, profileStep, saveProfileStep } from "../../fixtures/onboarding-pages";
import { expect, test } from "../../fixtures/test";

// A9, onboarding (M1 design 3): the server records the steps the web app's registry defines.

test("A9 (API): a token records onboarding steps, each once, in order", async ({ api, db }, testInfo) => {
  const email = emailFor(testInfo);
  const tokens = await register(api, email);
  const userId = await accountIdOf(db, email);
  const token = await createToken(api, tokens.access_token, { name: "A9" });
  const record = (step: string) =>
    api.POST("/api/v0/me/onboarding-steps", { body: { step }, headers: bearer(token.token) });

  const expectRecorded = async (step: string, steps: string[]) => {
    const { response, data } = await record(step);
    expect(response.status).toBe(200);
    expect(data?.onboarding_steps).toEqual(steps);
  };
  await expectRecorded("profile", ["profile"]);
  await expectRecorded("profile", ["profile"]);
  await expectRecorded("welcome", ["profile", "welcome"]);

  const { response, error } = await record("Profile");
  expect(response.status).toBe(422);
  expect(error?.errors?.map((e) => ({ field: e.field, code: e.code }))).toEqual([
    { field: "step", code: "invalid_format" },
  ]);
  await expectOnboardingSteps(db, userId, ["profile", "welcome"]);
});

test("A9 (page): a new account is taken through onboarding, then to where it was going, and not again", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const email = emailFor(testInfo);
  const page = await signedInPage(await register(api, email));
  const userId = await accountIdOf(db, email);
  await page.goto("/acme?view=list");

  // Every page waits for the steps left: the first comes with its progress and the name the account has.
  await expect(profileStep(page)).toBeVisible();
  await expect(page).toHaveURL(`/onboarding?next=${encodeURIComponent("/acme?view=list")}`);
  await expect(page.getByText("Step 1 of 1", { exact: true })).toBeVisible();
  await expect(displayNameField(page)).toHaveValue(displayNameOf(email));

  // An empty name is caught before sending; one the server refuses shows under the field; the step stays.
  await displayNameField(page).fill("  ");
  await page.getByRole("button", { name: "Continue" }).click();
  await expect(page.getByText("Required.", { exact: true })).toBeVisible();
  await displayNameField(page).fill("A".repeat(101));
  const refused = page.waitForResponse((response) => response.request().method() === "PATCH");
  await page.getByRole("button", { name: "Continue" }).click();
  expect((await refused).status()).toBe(422);
  await expect(page.getByText("At most 100 characters.", { exact: true })).toBeVisible();
  await expect(profileStep(page)).toBeVisible();
  pageWatch.expectConsole({ errors: [failedToLoad(422)] });

  await displayNameField(page).fill("Ada Lovelace");
  expect(await saveProfileStep(page)).toBe(200);

  await expect(page.getByRole("heading", { level: 1, name: "Page not found" })).toBeVisible();
  await expect(page).toHaveURL("/acme?view=list");
  await expectDisplayName(db, userId, "Ada Lovelace");
  await expectOnboardingSteps(db, userId, ["profile"]);
  // Onboarding done, its page sends the account on.
  await page.goto("/onboarding");
  await expect(page.getByRole("heading", { level: 1, name: "Nerve Wiki" })).toBeVisible();
  await expect(page).toHaveURL("/");
});
