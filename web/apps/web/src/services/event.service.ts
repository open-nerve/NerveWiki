import type { ApiClient, EventHello, EventLock, EventPages, EventReset } from "@nervewiki/api-client";

import { unwrap } from "./api";

export type { EventHello, EventLock, EventPages, EventReset };

/**
 * EventService opens the account's event stream (M5 design 4.10): GET
 * /api/v0/events, which stays open until a reset or its end. The session's
 * middleware checks the login when the answer's head comes; whoever reads
 * the body aborts it, through signal, when the login changes.
 */
export class EventService {
  constructor(private readonly api: ApiClient) {}

  /**
   * open answers the stream's body once its head is a 200; any other answer
   * throws its ApiError, a 503 not_ready with its retryAfter.
   */
  async open(signal: AbortSignal): Promise<ReadableStream<Uint8Array>> {
    const body = unwrap(await this.api.GET("/api/v0/events", { parseAs: "stream", signal }));
    if (!body) {
      throw new Error("the event stream answered 200 without a body");
    }
    return body;
  }
}
