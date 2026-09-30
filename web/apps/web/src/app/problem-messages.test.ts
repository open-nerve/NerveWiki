import { describe, expect, test } from "vitest";

import spec from "../../../../../api/dist/openapi.yaml?raw";
import { translator } from "../i18n/i18n";
import { ApiError } from "../services/api";
import { SessionChangedError, SessionUnavailableError } from "../session/token-manager";
import { errorText, fieldErrors, formErrors, problemMessages } from "./problem-messages";

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

// Every code of the operations whose errors a page shows has a message: a
// code added to the contract fails here until it has one (M1/P5 design 3.4).
test.each(["getInstance", "register", "login", "getMe", "updateMe", "recordOnboardingStep"])(
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
    ["a code without one", new ApiError(409, problem(409, "page.locked")), "Something went wrong (page.locked)."],
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
    ["a change of session", new SessionChangedError(), undefined],
  ])("%s", (_name, error, want) => {
    expect(errorText(error, t)).toBe(want);
  });
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

test("formErrors puts a 422's problems under the fields only, and the rest above the form", () => {
  const onFields = new ApiError(422, {
    ...problem(422, "validation_failed"),
    errors: [{ field: "email", code: "invalid_format" }],
  });
  const conflict = new ApiError(409, problem(409, "identity.email_taken"));

  expect(formErrors(undefined, t)).toEqual({ banner: undefined, fields: {} });
  expect(formErrors(onFields, t)).toEqual({ banner: undefined, fields: { email: "Not a valid e-mail address." } });
  expect(formErrors(new ApiError(422, problem(422, "validation_failed")), t).banner).toBe("Some values are not valid.");
  expect(formErrors(conflict, t)).toEqual({
    banner: "An account with this e-mail address already exists.",
    fields: {},
  });
});
