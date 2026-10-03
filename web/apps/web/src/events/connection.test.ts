import { describe, expect, test } from "vitest";

import { SessionChangedError } from "../session/token-manager";
import { connect } from "./connection";
import type { Frame } from "./frames";

/** A stream body the test writes to, piece by piece. */
function body() {
  let controller!: ReadableStreamDefaultController<Uint8Array>;
  const stream = new ReadableStream<Uint8Array>({ start: (c) => void (controller = c) });
  const encoder = new TextEncoder();
  return {
    stream,
    write: (text: string) => controller.enqueue(encoder.encode(text)),
    close: () => controller.close(),
    fail: (error: unknown) => controller.error(error),
  };
}

const hello = 'event: hello\ndata: {"heartbeat_seconds":20}\n\n';

describe("connect", () => {
  test("hands each frame on in order and ends with the server's reset", async () => {
    const b = body();
    const frames: Frame[] = [];
    b.write(hello);
    b.write(": heartbeat\n\n");
    b.write('event: reset\ndata: {"reason":"expired"}\n\n');

    const ending = await connect(
      async () => b.stream,
      new AbortController().signal,
      (f) => frames.push(f)
    );

    expect(ending).toEqual({ ended: "reset", reason: "expired" });
    expect(frames.map((f) => f.type)).toEqual(["hello", "beat"]);
  });

  test("ends closed when the stream ends without a reset, failed when reading fails", async () => {
    const closed = body();
    closed.write(hello);
    closed.close();
    const failing = body();
    failing.fail(new TypeError("network error"));

    expect(
      await connect(
        async () => closed.stream,
        new AbortController().signal,
        () => {}
      )
    ).toEqual({ ended: "closed" });
    expect(
      await connect(
        async () => failing.stream,
        new AbortController().signal,
        () => {}
      )
    ).toMatchObject({
      ended: "failed",
      error: new TypeError("network error"),
    });
  });

  test("ends failed with the opening's error", async () => {
    const busy = Object.assign(new Error("not ready"), { status: 503, retryAfter: 1 });

    const ending = await connect(
      async () => {
        throw busy;
      },
      new AbortController().signal,
      () => {}
    );

    expect(ending).toEqual({ ended: "failed", error: busy });
  });

  test("ends aborted when its signal aborts, and when the login changed", async () => {
    const b = body();
    const stop = new AbortController();
    const reading = connect(
      async () => b.stream,
      stop.signal,
      () => {}
    );
    stop.abort();
    b.fail(new DOMException("aborted", "AbortError"));

    expect(await reading).toEqual({ ended: "aborted" });
    expect(
      await connect(
        async () => {
          throw new SessionChangedError();
        },
        new AbortController().signal,
        () => {}
      )
    ).toEqual({ ended: "aborted" });
  });
});
