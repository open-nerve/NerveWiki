import type { Problem } from "@nervewiki/api-client";

/**
 * ApiError is an answer of the API that is not a success: its status, and
 * the problem when the body is problem+json (a proxy in front of nervewiki
 * may answer with something else). A form shows problem.errors next to its
 * fields.
 */
export class ApiError extends Error {
  readonly status: number;
  readonly problem: Problem | undefined;

  constructor(status: number, body: unknown) {
    const problem = isProblem(body) ? body : undefined;
    super(problem?.detail ?? problem?.title ?? `HTTP ${status}`);
    this.name = "ApiError";
    this.status = status;
    this.problem = problem;
  }

  /** code is the problem's code, the one clients branch on. */
  get code(): string | undefined {
    return this.problem?.code;
  }
}

/**
 * isRetryable tells SWR which failed loads to try again: not a request the
 * API refused (4xx), which it would refuse again; an answer of 5xx and a
 * network failure may pass later.
 */
export function isRetryable(error: unknown): boolean {
  return !(error instanceof ApiError && error.status < 500);
}

function isProblem(body: unknown): body is Problem {
  return typeof body === "object" && body !== null && typeof (body as { code?: unknown }).code === "string";
}

/** The part of an openapi-fetch result that unwrap reads. */
interface Result<T> {
  data?: T;
  error?: unknown;
  response: Response;
}

/**
 * unwrap returns the data of an openapi-fetch result for an operation that
 * answers with a body, or throws its error as an ApiError. A network failure
 * has already thrown, as fetch's TypeError.
 */
export function unwrap<T>({ data, error, response }: Result<T>): T {
  if (!response.ok || data === undefined) {
    throw new ApiError(response.status, error);
  }
  return data;
}
