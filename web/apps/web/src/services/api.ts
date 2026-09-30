import type { FieldError, Problem } from "@nervewiki/api-client";

export type { FieldError, Problem };

/**
 * ApiError is an answer of the API that is not a success: its status, and
 * the problem when the body is problem+json (a proxy in front of nervewiki
 * may answer with something else). A form shows problem.errors next to its
 * fields.
 */
export class ApiError extends Error {
  readonly status: number;
  readonly problem: Problem | undefined;
  /** Seconds to wait before trying again, from Retry-After (429 rate_limited, 503 server_busy). */
  readonly retryAfter: number | undefined;

  constructor(status: number, body: unknown, retryAfter?: number) {
    const problem = isProblem(body) ? body : undefined;
    super(problem?.detail ?? problem?.title ?? `HTTP ${status}`);
    this.name = "ApiError";
    this.status = status;
    this.problem = problem;
    this.retryAfter = retryAfter;
  }

  /** code is the problem's code, the one clients branch on. */
  get code(): string | undefined {
    return this.problem?.code;
  }
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
 * unwrap returns the data of an openapi-fetch result, undefined for a 204,
 * or throws its error as an ApiError; a success without the body the
 * operation declares is an error too. A network failure has already thrown,
 * as fetch's TypeError.
 */
export function unwrap<T>({ data, error, response }: Result<T>): T {
  if (!response.ok) {
    throw new ApiError(response.status, error, retryAfterOf(response));
  }
  if (data === undefined && response.status !== 204) {
    throw new ApiError(response.status, error);
  }
  return data as T;
}

/** The seconds of the response's Retry-After, which the API sends as a whole number. */
function retryAfterOf(response: Response): number | undefined {
  const value = response.headers.get("Retry-After");
  return value !== null && /^\d+$/.test(value) ? Number(value) : undefined;
}
