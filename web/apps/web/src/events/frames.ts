import type { EventHello, EventLock, EventPages, EventReset } from "../services/event.service";

// The frames of the event stream (M5 design 4.10): server-sent events, each
// an event line and one line of JSON data ended by a blank line; a comment
// line is the server's heartbeat.

/** A frame of the stream, or a heartbeat; "other" is an event type this client does not know, which it skips. */
export type Frame =
  | { type: "hello"; data: EventHello }
  | { type: "pages"; data: EventPages }
  | { type: "lock"; data: EventLock }
  | { type: "reset"; data: EventReset }
  | { type: "beat" }
  | { type: "other"; event: string };

const known = new Set(["hello", "pages", "lock", "reset"]);

/**
 * FrameParser turns the stream's text, as it comes in pieces, into frames.
 * A frame whose data is not JSON is dropped: one bad frame does not end the
 * stream.
 */
export class FrameParser {
  #buffered = "";
  #event = "";
  #data: string[] = [];

  /** push adds the next piece of text and returns the frames it completes. */
  push(text: string): Frame[] {
    this.#buffered += text;
    const frames: Frame[] = [];
    for (let end = this.#buffered.indexOf("\n"); end >= 0; end = this.#buffered.indexOf("\n")) {
      const line = this.#buffered.slice(0, end).replace(/\r$/, "");
      this.#buffered = this.#buffered.slice(end + 1);
      const frame = this.#line(line);
      if (frame) {
        frames.push(frame);
      }
    }
    return frames;
  }

  #line(line: string): Frame | undefined {
    if (line === "") {
      return this.#dispatch();
    }
    if (line.startsWith(":")) {
      return { type: "beat" };
    }
    const colon = line.indexOf(":");
    const field = colon < 0 ? line : line.slice(0, colon);
    const value = colon < 0 ? "" : line.slice(colon + 1).replace(/^ /, "");
    if (field === "event") {
      this.#event = value;
    } else if (field === "data") {
      this.#data.push(value);
    }
    return undefined;
  }

  #dispatch(): Frame | undefined {
    const event = this.#event;
    const data = this.#data.join("\n");
    this.#event = "";
    this.#data = [];
    if (event === "" && data === "") {
      return undefined;
    }
    if (!known.has(event)) {
      return { type: "other", event };
    }
    try {
      return { type: event, data: JSON.parse(data) } as Frame;
    } catch {
      return undefined;
    }
  }
}
