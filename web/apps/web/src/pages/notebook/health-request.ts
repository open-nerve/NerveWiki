import type { SWRConfiguration } from "swr";

import { onErrorRetry } from "../../app/retry";
import { ApiError } from "../../services/api";

/** Request ownership only: health results live in the entry's SWR provider. */
export type HealthScope = {
  key: readonly unknown[];
  notebookId: string;
  current: () => boolean;
  controllers: Pick<Set<AbortController>, "add" | "delete">;
};

export function healthCancelled(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}

export async function healthRequest<T>(
  scope: HealthScope,
  read: (signal: AbortSignal) => Promise<T>,
  current = scope.current
): Promise<T> {
  const controller = new AbortController();
  scope.controllers.add(controller);
  const active = () => scope.current() && current() && !controller.signal.aborted;
  try {
    if (!active()) throw new DOMException("Health request ended", "AbortError");
    const result = await read(controller.signal);
    if (!active()) throw new DOMException("Health request ended", "AbortError");
    return result;
  } catch (error) {
    if (!active()) throw new DOMException("Health request ended", "AbortError");
    throw error;
  } finally {
    scope.controllers.delete(controller);
  }
}

/** Busy is an explicit retry; other errors keep the app's bounded policy, within their own scope. */
export function healthRetry(current: () => boolean): NonNullable<SWRConfiguration["onErrorRetry"]> {
  return (error, key, config, revalidate, options) => {
    if (!current() || healthCancelled(error) || (error instanceof ApiError && error.code === "server_busy")) return;
    onErrorRetry?.(
      error,
      key,
      config,
      (next) => {
        if (current()) void revalidate(next);
      },
      options
    );
  };
}
