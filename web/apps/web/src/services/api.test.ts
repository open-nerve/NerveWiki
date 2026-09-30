import { describe, expect, test } from "vitest";

import { ApiError, isRetryable } from "./api";

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

describe("isRetryable", () => {
  test.each([
    ["a refusal (404)", new ApiError(404, { status: 404, code: "not_found", title: "Not Found" }), false],
    ["a refusal (400)", new ApiError(400, undefined), false],
    [
      "a server failure (503)",
      new ApiError(503, { status: 503, code: "server_busy", title: "Service Unavailable" }),
      true,
    ],
    ["a proxy's 502", new ApiError(502, "<html>"), true],
    ["a network failure", new TypeError("Failed to fetch"), true],
  ])("%s: %s", (_name, error, want) => {
    expect(isRetryable(error)).toBe(want);
  });
});
