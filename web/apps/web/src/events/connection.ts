import { SessionChangedError } from "../session/token-manager";
import type { EventReset } from "../services/event.service";
import { FrameParser, type Frame } from "./frames";

/** How a connection ended: by the server's reset, without one, by a failure (opening or reading), or aborted. */
export type Ending =
  | { ended: "reset"; reason: EventReset["reason"] }
  | { ended: "closed" }
  | { ended: "failed"; error: unknown }
  | { ended: "aborted" };

/** Opens the stream: EventService.open. */
export type Open = (signal: AbortSignal) => Promise<ReadableStream<Uint8Array>>;

/**
 * connect opens the stream and hands each of its frames but the reset to
 * onFrame, in order, until it ends. The end of the login (the session's
 * SessionChangedError) counts as aborted: the generation that opened it is
 * over.
 */
export async function connect(open: Open, signal: AbortSignal, onFrame: (frame: Frame) => void): Promise<Ending> {
  let body: ReadableStream<Uint8Array>;
  try {
    body = await open(signal);
  } catch (error) {
    return endingOf(error, signal);
  }
  const reader = body.getReader();
  const decoder = new TextDecoder();
  const parser = new FrameParser();
  try {
    for (;;) {
      // oxlint-disable-next-line no-await-in-loop -- a stream's chunks come one after another
      const { value, done } = await reader.read();
      if (done) {
        return { ended: "closed" };
      }
      for (const frame of parser.push(decoder.decode(value, { stream: true }))) {
        // A connection aborted hands on nothing more, though the rest of the chunk was read.
        if (signal.aborted) {
          return { ended: "aborted" };
        }
        if (frame.type === "reset") {
          return { ended: "reset", reason: frame.data.reason };
        }
        onFrame(frame);
      }
    }
  } catch (error) {
    return endingOf(error, signal);
  } finally {
    reader.cancel().catch(() => undefined);
  }
}

function endingOf(error: unknown, signal: AbortSignal): Ending {
  return signal.aborted || error instanceof SessionChangedError ? { ended: "aborted" } : { ended: "failed", error };
}
