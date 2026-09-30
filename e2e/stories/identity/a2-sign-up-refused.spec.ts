import { createClient, type ApiClient } from "@nervewiki/api-client";

import { countIdentity, expectNothingAdded } from "../../fixtures/assert/identity";
import { emailFor, password, register } from "../../fixtures/auth";
import { expect, test } from "../../fixtures/test";

// A2, sign-up refused (M1 design 3).

async function expectRefused(
  api: ApiClient,
  body: { email: string; password: string },
  want: { status: number; code: string; fields?: { field: string; code: string }[] }
): Promise<void> {
  const { response, error } = await api.POST("/api/v0/auth/register", { body });
  const label = `${body.email} ${body.password}`;
  expect(response.status, label).toBe(want.status);
  expect(error?.code, label).toBe(want.code);
  if (want.fields) {
    expect(
      error?.errors?.map((e) => ({ field: e.field, code: e.code })),
      label
    ).toEqual(want.fields);
  }
}

test("A2 (API): a refused sign-up answers why and adds nothing", async ({ api, db, nervewikiWith }, testInfo) => {
  const email = emailFor(testInfo);
  await register(api, email);
  const before = await countIdentity(db);
  const newEmail = emailFor(testInfo, "new");
  const invalid = (pw: string, code: string) =>
    expectRefused(
      api,
      { email: newEmail, password: pw },
      { status: 422, code: "validation_failed", fields: [{ field: "password", code }] }
    );

  await Promise.all([
    // The address is taken, whatever its case.
    expectRefused(api, { email: email.toUpperCase(), password }, { status: 409, code: "identity.email_taken" }),
    // Too short, too long, common ones, and one made of the address.
    invalid("seven77", "too_short"),
    invalid("x".repeat(129), "too_long"),
    invalid("password", "common_password"),
    invalid("Password1!~", "common_password"),
    invalid(`${newEmail.slice(0, newEmail.indexOf("@"))}!2026`, "common_password"),
    // Every problem at once.
    expectRefused(
      api,
      { email: "not an address", password: "" },
      {
        status: 422,
        code: "validation_failed",
        fields: [
          { field: "email", code: "invalid_format" },
          { field: "password", code: "required" },
        ],
      }
    ),
  ]);

  // With sign-up off, the instance says so, and every address gets the same
  // answer, a taken one too, before its body is looked at.
  const closed = createClient({
    baseUrl: (await nervewikiWith(db.url, { env: { NWIKI_AUTH__SIGNUP_ENABLED: "false" } })).baseURL,
  });
  const info = await closed.GET("/api/v0/instance");
  expect(info.data?.signup_enabled).toBe(false);
  await Promise.all(
    [
      { email: newEmail, password },
      { email, password },
      { email: "not an address", password: "" },
    ].map((body) => expectRefused(closed, body, { status: 403, code: "identity.signup_disabled" }))
  );

  await expectNothingAdded(db, before);
});
