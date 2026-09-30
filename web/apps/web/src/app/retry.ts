import type { SWRConfiguration } from "swr";

import { ApiError } from "../services/api";
import { SessionChangedError } from "../session/token-manager";

const maxRetries = 5;
const firstBackoffMs = 5_000;
const maxBackoffMs = 60_000;

/**
 * retryDelay is how long to wait before loading again after the attempt-th
 * failure (1 for the first), or undefined to stop (M1/P5 design 3.4): a
 * request of a session that ended is over; a refusal that says when to come
 * back (429, 503 with Retry-After) waits that long; any other refusal (4xx)
 * would be refused again; a server failure or a network failure backs off
 * from 5 s to 60 s. Five retries at most.
 */
export function retryDelay(error: unknown, attempt: number): number | undefined {
  if (attempt > maxRetries || error instanceof SessionChangedError) {
    return undefined;
  }
  if (error instanceof ApiError) {
    if (error.retryAfter !== undefined) {
      return error.retryAfter * 1000;
    }
    if (error.status < 500) {
      return undefined;
    }
  }
  return Math.min(firstBackoffMs * 2 ** (attempt - 1), maxBackoffMs);
}

/** SWR's onErrorRetry by retryDelay. */
export const onErrorRetry: SWRConfiguration["onErrorRetry"] = (error, _key, _config, revalidate, { retryCount }) => {
  const delay = retryDelay(error, retryCount);
  if (delay !== undefined) {
    setTimeout(() => void revalidate({ retryCount }), delay);
  }
};
