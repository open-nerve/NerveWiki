import { describe, expect, test } from "vitest";

import spec from "../../../../../api/dist/openapi.yaml?raw";
import { translator } from "../i18n/i18n";
import { ApiError } from "../services/api";
import { SessionChangedError, SessionStorageError, SessionUnavailableError } from "../session/token-manager";
import { errorText, fieldErrors, formErrors, lockedText, problemMessages } from "./problem-messages";

const t = translator("en");

/**
 * The problem codes an operation of the bundled contract lists, with the
 * ones every operation may answer; an operation with a bearer token may also
 * answer unauthorized.
 */
function codesOf(operationId: string): string[] {
  const lines = spec.split("\n");
  const listAfter = (start: number) => {
    const items: string[] = [];
    const indent = lines[start]?.search(/\S/) ?? 0;
    if (lines[start]?.trimEnd().endsWith("[]")) return items;
    for (const line of lines.slice(start + 1)) {
      const item = /^(\s*)- (\S+)$/.exec(line);
      if (!item || (item[1]?.length ?? 0) < indent) break;
      items.push(item[2] ?? "");
    }
    return items;
  };
  const at = lines.findIndex((line) => line.trim() === `operationId: ${operationId}`);
  expect(at, `operation ${operationId} in api/dist/openapi.yaml`).toBeGreaterThan(-1);
  const end = lines.findIndex((line, i) => i > at && /^\s{4}\w+:$/.test(line) && !line.startsWith("      "));
  const operation = lines.slice(at, end === -1 ? undefined : end);
  const own = listAfter(at + operation.findIndex((line) => line.trim().startsWith("x-problem-codes:")));
  const everywhere = listAfter(lines.findIndex((line) => line.startsWith("x-problem-codes:")));
  const bearer = !operation.some((line) => line.trim() === "security: []");
  return [...own, ...everywhere, ...(bearer ? ["unauthorized"] : [])];
}

/**
 * The operations whose errors no page shows: the token manager answers them itself; an attachment's content is opened
 * by the browser at its signed address, never fetched by a page.
 */
const unshown = ["refreshTokens", "logout", "getAssetContent"];

const operations = [...spec.matchAll(/^\s+operationId: (\S+)$/gm)].map((match) => match[1] ?? "");

test("the contract has the operations the check skips, and others", () => {
  expect(operations).toEqual(expect.arrayContaining(unshown));
  expect(operations.length).toBeGreaterThan(unshown.length);
});

// Every code of every operation but the unshown ones has a message: a code,
// or an operation, added to the contract fails here until it has one (M1/P5
// design 3.4).
test.each(operations.filter((operationId) => !unshown.includes(operationId)))(
  "%s: every problem code has a message",
  (operationId) => {
    const codes = codesOf(operationId);
    expect(codes.length).toBeGreaterThan(0);
    expect(codes.filter((code) => !Object.hasOwn(problemMessages, code))).toEqual([]);
  }
);

const problem = (status: number, code: string) => ({ status, code, title: "" });

describe("errorText", () => {
  test.each([
    [
      "a code with a message",
      new ApiError(401, problem(401, "identity.invalid_credentials")),
      "The e-mail address or the password is incorrect.",
    ],
    [
      "429 with Retry-After",
      new ApiError(429, problem(429, "rate_limited"), 12),
      "Too many attempts. Try again in 12 s.",
    ],
    ["429 without", new ApiError(429, problem(429, "rate_limited")), "Too many attempts. Try again later."],
    ["a code without one", new ApiError(409, problem(409, "page.frozen")), "Something went wrong (page.frozen)."],
    ["a proxy's answer", new ApiError(502, "<html>"), "Something went wrong (HTTP 502)."],
    [
      "a network failure",
      new TypeError("Failed to fetch"),
      "Cannot reach the server. Check the connection and try again.",
    ],
    [
      "a refresh that failed for now",
      new SessionUnavailableError(0),
      "Cannot reach the server for now. You are still signed in; try again in a moment.",
    ],
    [
      "a browser that will not save the session",
      new SessionStorageError(new DOMException("The quota has been exceeded.", "QuotaExceededError")),
      "This browser could not save the sign-in: its storage for this site is full or blocked. Free some space or allow this site's data, then try again.",
    ],
    ["a change of session", new SessionChangedError(), undefined],
  ])("%s", (_name, error, want) => {
    expect(errorText(error, t)).toBe(want);
  });
});

/** page.locked of the page p2, which userId holds as Bob. */
const lockedBy = (userId: string) =>
  new ApiError(409, {
    status: 409,
    code: "page.locked",
    title: "Conflict",
    lock: { page_id: "p2", user_id: userId, display_name: "Bob" },
  });
/** The tree's titles: p2 is Linux. */
const titleOf = (id: string) => (id === "p2" ? "Linux" : undefined);

test("lockedText names who edits which page, the account itself elsewhere; nothing it cannot name", () => {
  expect(lockedText(lockedBy("u-bob"), t, "u-ada", titleOf)).toBe("Bob is editing “Linux”.");
  expect(lockedText(lockedBy("u-ada"), t, "u-ada", titleOf)).toBe("You are editing “Linux” elsewhere.");
  expect(lockedText(lockedBy("u-bob"), t, "u-ada", () => undefined)).toBeUndefined();
  expect(
    lockedText(new ApiError(409, { status: 409, code: "page.locked", title: "Conflict" }), t, "u-ada", titleOf)
  ).toBe(undefined);
  expect(lockedText(new ApiError(404, { status: 404, code: "page.not_found", title: "" }), t, "u-ada", titleOf)).toBe(
    undefined
  );
  expect(translator("zh-CN")("page.lockedTitled", { name: "Bob", page: "Linux" })).toBe("Bob 正在编辑「Linux」。");
});

test("fieldErrors says each field's first problem, better for the fields it knows", () => {
  const error = new ApiError(422, {
    status: 422,
    code: "validation_failed",
    title: "",
    errors: [
      { field: "email", code: "invalid_format", message: "is not a valid e-mail address" },
      { field: "password", code: "too_short", message: "must be at least 8 characters" },
      { field: "password", code: "common_password", message: "is too common" },
      { field: "step", code: "invalid_format", message: "must be …" },
    ],
  });

  expect(fieldErrors(error, t)).toEqual({
    email: "Not a valid e-mail address.",
    password: "At least 8 characters.",
    step: "Not in the right form.",
  });
  expect(fieldErrors(new TypeError("Failed to fetch"), t)).toEqual({});
});

// A form's own texts say its fields' codes first; the codes they do not name
// keep the field's text, and other fields theirs (v0.1 design 13.2, item 11).
test("fieldErrors says a field's code the form's way where it has a text of its own", () => {
  const error = new ApiError(422, {
    status: 422,
    code: "validation_failed",
    title: "",
    errors: [
      { field: "name", code: "too_long", message: "must be at most 255 bytes" },
      { field: "email", code: "invalid_format", message: "is not a valid e-mail address" },
    ],
  });
  const fieldTexts = { "name.too_long": "field.notebook_name.too_long", "email.too_long": "field.too_long" } as const;

  expect(fieldErrors(error, t, fieldTexts)).toEqual({
    name: "At most 255 bytes: 255 Latin letters, or about 85 Chinese characters.",
    email: "Not a valid e-mail address.",
  });
  expect(fieldErrors(error, t)).toEqual({
    name: "At most 100 characters.",
    email: "Not a valid e-mail address.",
  });
});

test("formErrors puts a 422's problems under the fields shown only, and the rest above the form", () => {
  const onEmail = new ApiError(422, {
    ...problem(422, "validation_failed"),
    errors: [{ field: "email", code: "invalid_format" }],
  });
  const conflict = new ApiError(409, problem(409, "identity.email_taken"));
  const shown = ["email", "password"];

  expect(formErrors(undefined, t, shown)).toEqual({ banner: undefined, fields: {} });
  expect(formErrors(onEmail, t, shown)).toEqual({
    banner: undefined,
    fields: { email: "Not a valid e-mail address." },
  });
  expect(formErrors(onEmail, t, ["display_name"]).banner).toBe("Some values are not valid.");
  expect(formErrors(new ApiError(422, problem(422, "validation_failed")), t, shown).banner).toBe(
    "Some values are not valid."
  );
  expect(formErrors(conflict, t, shown)).toEqual({
    banner: "An account with this e-mail address already exists.",
    fields: {},
  });
});

test("formErrors shows a problem code of onField under its field, and nothing above", () => {
  const wrong = new ApiError(422, problem(422, "identity.current_password_incorrect"));
  const onField = { "identity.current_password_incorrect": "current_password" };

  expect(formErrors(wrong, t, ["current_password"], { onField })).toEqual({
    banner: undefined,
    fields: { current_password: "The current password is incorrect." },
  });
  expect(formErrors(wrong, t, ["current_password"]).banner).toBe("The current password is incorrect.");
});

test("a page's own text of a code takes the place of the usual one, under a field or above the form", () => {
  const refused = new ApiError(403, problem(403, "identity.signup_disabled"));
  const texts = { "identity.signup_disabled": "invitation.signUpRefused" } as const;
  const own =
    "This server takes new accounts only for the addresses invited: sign up with the one the invitation was sent to.";

  expect(errorText(refused, t, texts)).toBe(own);
  expect(errorText(refused, t)).toBe("Sign-up is disabled on this server.");
  expect(formErrors(refused, t, ["email"], { texts }).banner).toBe(own);
  expect(formErrors(refused, t, ["email"], { texts, onField: { "identity.signup_disabled": "email" } })).toEqual({
    banner: undefined,
    fields: { email: own },
  });
});
