import type { EventHello, EventLock, EventPages, EventReset } from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import { bearer } from "./auth";

// The event stream of the stories, read with Node's fetch (M5 design 4.10;
// M5/P2 design 3.13): its frames one at a time, the heartbeat's comment
// lines left out.

/** The data of each frame, by its event. */
interface FrameData {
  hello: EventHello;
  pages: EventPages;
  lock: EventLock;
  reset: EventReset;
}

type FrameEvent = keyof FrameData;

/** A frame of a stream, its event and its data; "end" once the server has closed it. */
type Frame = { [E in FrameEvent]: { event: E; data: FrameData[E] } }[FrameEvent] | { event: "end" };

/** How long a story waits for a frame by default: the server writes one as the write commits. */
const frameTimeoutMs = 10_000;

/** An open event stream. */
export interface EventStream {
  /** What its hello frame said. */
  readonly hello: EventHello;
  /** The next frame, within timeoutMs. */
  next(timeoutMs?: number): Promise<Frame>;
  /** Expects the next frame to be event, and returns its data. */
  expectNext<E extends FrameEvent>(event: E, timeoutMs?: number): Promise<FrameData[E]>;
  /** Expects the next frame to be a reset for reason, then the stream's end. */
  expectReset(reason: EventReset["reason"], timeoutMs?: number): Promise<void>;
  /** Closes the stream, as a client that goes. */
  close(): void;
}

/** credential's request of an event stream at baseURL, as the server answers it: none for no credential. */
export async function fetchEvents(baseURL: string, credential?: string, signal?: AbortSignal): Promise<Response> {
  return fetch(`${baseURL}/api/v0/events`, { headers: credential === undefined ? {} : bearer(credential), signal });
}

/** Opens credential's event stream at baseURL, past its hello. */
export async function connectEvents(baseURL: string, credential: string): Promise<EventStream> {
  const controller = new AbortController();
  const response = await fetchEvents(baseURL, credential, controller.signal);
  expect(response.status, "open the event stream").toBe(200);
  expect(response.headers.get("content-type")).toBe("text/event-stream");
  if (!response.body) {
    throw new Error("the event stream answered 200 without a body");
  }
  const frames = new FrameQueue(response.body);
  const next = (timeoutMs = frameTimeoutMs) => frames.next(timeoutMs);
  const expectNext = async <E extends FrameEvent>(event: E, timeoutMs?: number): Promise<FrameData[E]> => {
    const frame = await next(timeoutMs);
    expect(frame.event, `the next frame: ${JSON.stringify(frame)}`).toBe(event);
    if (frame.event === "end") {
      throw new Error(`the event stream ended, want ${event}`);
    }
    return frame.data as FrameData[E];
  };
  return {
    hello: await expectNext("hello"),
    next,
    expectNext,
    async expectReset(reason, timeoutMs) {
      expect(await expectNext("reset", timeoutMs)).toEqual({ reason });
      expect((await next()).event, "the stream's end after its reset").toBe("end");
    },
    close: () => controller.abort(),
  };
}

/** The frames of a stream's body, read as they come, in order. */
class FrameQueue {
  private readonly frames: Frame[] = [];
  private readonly waiting: ((frame: Frame) => void)[] = [];
  private ended = false;

  constructor(body: ReadableStream<Uint8Array>) {
    void this.read(body);
  }

  /** The next frame, within timeoutMs: "end" once the body has ended. */
  next(timeoutMs: number): Promise<Frame> {
    const frame = this.frames.shift();
    if (frame) {
      return Promise.resolve(frame);
    }
    if (this.ended) {
      return Promise.resolve({ event: "end" });
    }
    return new Promise((resolve, reject) => {
      const take = (next: Frame) => {
        clearTimeout(timer);
        resolve(next);
      };
      const timer = setTimeout(() => {
        this.waiting.splice(this.waiting.indexOf(take), 1);
        reject(new Error(`no frame of the event stream within ${timeoutMs} ms`));
      }, timeoutMs);
      this.waiting.push(take);
    });
  }

  private deliver(frame: Frame): void {
    const take = this.waiting.shift();
    if (take) {
      take(frame);
    } else {
      this.frames.push(frame);
    }
  }

  private async read(body: ReadableStream<Uint8Array>): Promise<void> {
    const decoder = new TextDecoder();
    let buffered = "";
    let event = "";
    let data = "";
    try {
      for await (const chunk of body) {
        buffered += decoder.decode(chunk, { stream: true });
        for (let end = buffered.indexOf("\n"); end >= 0; end = buffered.indexOf("\n")) {
          const line = buffered.slice(0, end);
          buffered = buffered.slice(end + 1);
          if (line.startsWith("event: ")) {
            event = line.slice("event: ".length);
          } else if (line.startsWith("data: ")) {
            data = line.slice("data: ".length);
          } else if (line === "" && event !== "") {
            this.deliver({ event, data: JSON.parse(data) } as Frame);
            event = "";
            data = "";
          }
        }
      }
    } catch {
      // Closed by the story: the stream has ended.
    }
    this.ended = true;
    for (const take of this.waiting.splice(0)) {
      take({ event: "end" });
    }
  }
}
