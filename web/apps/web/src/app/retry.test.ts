import { expect, test } from "vitest";

import { ApiError } from "../services/api";
import { SessionChangedError } from "../session/token-manager";
import { retryDelay } from "./retry";

const problem = (status: number, code: string) => ({ status, code, title: "" });

test.each([
  ["a session that ended", new SessionChangedError(), 1, undefined],
  ["a refusal (404)", new ApiError(404, problem(404, "not_found")), 1, undefined],
  ["a refusal (422)", new ApiError(422, problem(422, "validation_failed")), 1, undefined],
  ["429 with Retry-After", new ApiError(429, problem(429, "rate_limited"), 7), 1, 7_000],
  ["503 server_busy with Retry-After", new ApiError(503, problem(503, "server_busy"), 1), 3, 1_000],
  ["429 without Retry-After", new ApiError(429, problem(429, "rate_limited")), 1, undefined],
  ["a server failure, first", new ApiError(500, problem(500, "internal_error")), 1, 5_000],
  ["a proxy's 502, third", new ApiError(502, "<html>"), 3, 20_000],
  ["a network failure, fifth", new TypeError("Failed to fetch"), 5, 60_000],
  ["a network failure, sixth", new TypeError("Failed to fetch"), 6, undefined],
  ["429 with Retry-After, sixth", new ApiError(429, problem(429, "rate_limited"), 7), 6, undefined],
])("%s (attempt %i): %s", (_name, error, attempt, want) => {
  expect(retryDelay(error, attempt)).toBe(want);
});
