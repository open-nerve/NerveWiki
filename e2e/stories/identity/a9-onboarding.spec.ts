import { accountIdOf, expectOnboardingSteps } from "../../fixtures/assert/identity";
import { bearer, createToken, emailFor, register } from "../../fixtures/auth";
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
