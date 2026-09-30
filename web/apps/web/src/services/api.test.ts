import { describe, expect, test } from "vitest";

import { ApiError, unwrap } from "./api";

describe("ApiError", () => {
  test("keeps the problem, with its field errors", () => {
    const problem = {
      status: 422,
      code: "validation_failed",
      title: "Unprocessable Entity",
      detail: "The request has invalid values.",
      errors: [{ field: "title", code: "required" as const, message: "is required" }],
    };

    const error = new ApiError(422, problem);

    expect(error).toMatchObject({ status: 422, code: "validation_failed", message: "The request has invalid values." });
    expect(error.problem?.errors).toEqual(problem.errors);
  });

  test("has no problem for a body that is not one", () => {
    expect(new ApiError(502, "<html>Bad Gateway</html>")).toMatchObject({ problem: undefined, code: undefined });
  });
});

const ok = (status: number, data?: unknown) => ({ data, response: new Response(null, { status }) });

/** The error unwrap throws for a 429 with retryAfter as its Retry-After. */
function refused(retryAfter?: string) {
  try {
    unwrap({
      error: { status: 429, code: "rate_limited", title: "Too Many Requests" },
      response: new Response(null, { status: 429, headers: retryAfter ? { "Retry-After": retryAfter } : {} }),
    });
  } catch (error) {
    return error;
  }
  return undefined;
}

describe("unwrap", () => {
  test("returns the data, and undefined for a 204", () => {
    expect(unwrap(ok(200, { id: 1 }))).toEqual({ id: 1 });
    expect(unwrap(ok(204))).toBeUndefined();
  });

  test("throws a success without the body it declares", () => {
    expect(() => unwrap(ok(200))).toThrow(ApiError);
  });

  test("throws a refusal as an ApiError with the seconds of Retry-After", () => {
    expect(refused("7")).toMatchObject({ status: 429, code: "rate_limited", retryAfter: 7 });
    expect(refused()).toMatchObject({ status: 429, retryAfter: undefined });
    expect(refused("Wed, 21 Oct 2026 07:28:00 GMT")).toMatchObject({ retryAfter: undefined });
  });
});
