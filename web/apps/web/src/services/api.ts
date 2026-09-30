import type { Problem } from "@nervewiki/api-client";

/**
 * ApiError is an answer of the API that is not a success: its status, and
 * the problem's code when the body is problem+json (a proxy in front of
 * nervewiki may answer with something else).
 */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string | undefined;

  constructor(status: number, body: unknown) {
    const problem = isProblem(body) ? body : undefined;
    super(problem?.detail ?? problem?.title ?? `HTTP ${status}`);
    this.name = "ApiError";
    this.status = status;
    this.code = problem?.code;
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
