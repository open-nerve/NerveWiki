import type { EventDeps } from "../events/deps";
import { FakeChannels } from "../events/testing/fake-channels";
import { FakeLocks } from "../events/testing/fake-locks";
import { FakePage } from "../events/testing/fake-page";
import { SharedStorage } from "../session/testing/fake-browser";
import { AppStores } from "../stores/root.store";
import type { Answer } from "./fakes";

/** A stream of events the fake API answered: the test sends it frames; it ends when the tab aborts it. */
export type EventStreamBody = {
  ended: boolean;
  /** hello is the stream's first frame. */
  hello(heartbeatSeconds?: number): void;
  send(type: string, data: unknown): void;
};

/**
 * eventServer is the event stream of the fake API (M5/P3): answer answers
 * each GET /api/v0/events with a new stream, the last of which the test
 * writes to.
 */
export function eventServer() {
  const streams: EventStreamBody[] = [];
  const answer: Answer = (request) => {
    let controller!: ReadableStreamDefaultController<Uint8Array>;
    const body = new ReadableStream<Uint8Array>({ start: (c) => void (controller = c) });
    const encoder = new TextEncoder();
    const stream: EventStreamBody = {
      ended: false,
      hello: (heartbeatSeconds = 20) => stream.send("hello", { heartbeat_seconds: heartbeatSeconds }),
      send: (type, data) => controller.enqueue(encoder.encode(`event: ${type}\ndata: ${JSON.stringify(data)}\n\n`)),
    };
    request.signal.addEventListener("abort", () => {
      stream.ended = true;
      controller.error(new DOMException("The request was aborted.", "AbortError"));
    });
    streams.push(stream);
    return new Response(body, { status: 200, headers: { "Content-Type": "text/event-stream" } });
  };
  return {
    answer,
    streams,
    last(): EventStreamBody {
      const stream = streams.at(-1);
      if (stream === undefined) {
        throw new Error("no stream was opened");
      }
      return stream;
    },
  };
}

/** withEvents is app with the event stream's deps of one tab, whose page the test shows and hides. */
export function withEvents(app: AppStores, page = new FakePage()): AppStores {
  const storage = new SharedStorage().tab("tab-0");
  const channels = new FakeChannels();
  const deps: EventDeps = {
    locks: new FakeLocks(),
    storage,
    onStorage: storage.onStorage,
    channel: (name) => channels.port(name),
    page,
    now: () => Date.now(),
    tabId: "tab-0",
  };
  return new AppStores(app.preferences, app.session, deps);
}
