import { accountIdOf, expectDisplayName } from "../../fixtures/assert/identity";
import { bearer, createToken, emailFor, register } from "../../fixtures/auth";
import { expect, test } from "../../fixtures/test";

// A8, the profile (M1 design 3); the theme and the language are the web app's (P6).

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
