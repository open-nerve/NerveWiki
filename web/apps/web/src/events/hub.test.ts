import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";

import { SharedStorage } from "../session/testing/fake-browser";
import { SessionChangedError } from "../session/token-manager";
import { TabChannel } from "./channel";
import type { Open } from "./connection";
import { EventHub, type HubEvent, type PageLifecycle } from "./hub";
import { leaseLeadership, webLockLeadership } from "./leadership";
import { FakeChannels } from "./testing/fake-channels";
import { FakeLocks } from "./testing/fake-locks";

const LOGIN = "login-0";
const NAME = `nwiki.events.${LOGIN}`;

const hello = (seconds = 20) => `event: hello\ndata: {"heartbeat_seconds":${seconds}}\n\n`;
const beat = ": heartbeat\n\n";
const pages = 'event: pages\ndata: {"workspace_id":"w","notebook_id":"n","tree":true,"pages":[]}\n\n';
const lock = 'event: lock\ndata: {"workspace_id":"w","notebook_id":"n","page_id":"p","session_id":"s"}\n\n';
const reset = 'event: reset\ndata: {"reason":"expired"}\n\n';

/** A stream the server opened for a tab, which the test writes to and ends; it ends too when the tab aborts it. */
type Stream = { tab: string; opened: number; ended: boolean; write: (text: string) => void; close: () => void };

/** The server's side of one login's streams: each open is a stream, unless the test queued a refusal. */
class FakeServer {
  readonly streams: Stream[] = [];
  /** When each open came, refused or not. */
  readonly opens: number[] = [];
  readonly #refusals: unknown[] = [];

  opener(tab: string): Open {
    return async (signal) => {
      this.opens.push(Date.now());
      const refusal = this.#refusals.shift();
      if (refusal !== undefined) {
        throw refusal;
      }
      let controller!: ReadableStreamDefaultController<Uint8Array>;
      const body = new ReadableStream<Uint8Array>({ start: (c) => void (controller = c) });
      const encoder = new TextEncoder();
      const stream: Stream = {
        tab,
        opened: Date.now(),
        ended: false,
        write: (text) => controller.enqueue(encoder.encode(text)),
        close: () => {
          stream.ended = true;
          controller.close();
        },
      };
      signal.addEventListener("abort", () => {
        stream.ended = true;
        controller.error(new DOMException("The stream was aborted.", "AbortError"));
      });
      this.streams.push(stream);
      return body;
    };
  }

  /** refuse answers the next open with error. */
  refuse(error: unknown): void {
    this.#refusals.push(error);
  }

  /** The streams still open. */
  live(): Stream[] {
    return this.streams.filter((s) => !s.ended);
  }

  /** The last stream opened. */
  last(): Stream {
    const stream = this.streams.at(-1);
    if (!stream) {
      throw new Error("no stream was opened");
    }
    return stream;
  }
}

/** A tab's page: visible or hidden, and its lifecycle's events, which the test fires. */
class FakePage implements PageLifecycle {
  shown = true;
  readonly #listeners = new Map<string, Set<() => void>>();

  visible(): boolean {
    return this.shown;
  }

  on(event: string, listener: () => void): () => void {
    const set = this.#listeners.get(event) ?? new Set<() => void>();
    this.#listeners.set(event, set);
    set.add(listener);
    return () => set.delete(listener);
  }

  fire(event: string): void {
    for (const listener of this.#listeners.get(event) ?? []) {
      listener();
    }
  }

  show(shown: boolean): void {
    this.shown = shown;
    this.fire("visibilitychange");
  }

  /** How many listeners the hub keeps on the page. */
  listening(): number {
    return [...this.#listeners.values()].reduce((n, set) => n + set.size, 0);
  }
}

type Tab = { id: string; hub: EventHub; page: FakePage; events: HubEvent["type"][] };

/** One browser's tabs of one login, which elect with Web Locks or with a lease in storage, and the server. */
function browserOf(kind: "Web Locks" | "the lease") {
  const locks = new FakeLocks();
  const storage = new SharedStorage();
  const channels = new FakeChannels();
  const server = new FakeServer();
  const tab = (id: string, { visible = true } = {}): Tab => {
    const view = storage.tab(id);
    const page = new FakePage();
    page.shown = visible;
    const hub = new EventHub({
      open: server.opener(id),
      leadership: () =>
        kind === "Web Locks"
          ? webLockLeadership(locks, NAME)
          : leaseLeadership({ storage: view, onStorage: view.onStorage, now: () => Date.now(), tabId: id }, NAME),
      channel: () => new TabChannel(channels.port("nwiki.events"), LOGIN),
      page,
      now: () => Date.now(),
    });
    const events: HubEvent["type"][] = [];
    hub.subscribe((event) => events.push(event.type));
    hub.start();
    return { id, hub, page, events };
  };
  return { tab, server, locks, storage, channels };
}

/** Lets the hubs' timers and promises run for ms. */
async function settle(ms = 100): Promise<void> {
  await vi.advanceTimersByTimeAsync(ms);
}

/** The gaps between the opens, in ms. */
function gaps(server: FakeServer): number[] {
  return server.opens.slice(1).map((at, i) => at - (server.opens[i] ?? 0));
}

beforeEach(() => {
  vi.useFakeTimers({ now: 1_000_000 });
});

afterEach(() => {
  vi.useRealTimers();
});

describe.each(["Web Locks", "the lease"] as const)("the hub with %s", (kind) => {
  test("one tab holds the one stream and hands each event to its subscribers and to the other tabs", async () => {
    const browser = browserOf(kind);
    const [a, b, c] = [browser.tab("a"), browser.tab("b"), browser.tab("c")];
    await settle();
    expect(browser.server.streams).toHaveLength(1);

    const stream = browser.server.last();
    stream.write(hello());
    stream.write(pages);
    stream.write(beat);
    stream.write(lock);
    await settle();

    for (const t of [a, b, c]) {
      expect(t.events).toEqual(["connected", "pages", "lock"]);
    }
  });

  test("connects again at once after the server's reset, and after 1, 2, 4 … 30 seconds when a stream ends otherwise", async () => {
    const browser = browserOf(kind);
    browser.tab("a");
    await settle();
    const reconnected = async (end: () => void) => {
      const at = Date.now();
      end();
      await settle(31_000);
      return browser.server.last().opened - at;
    };

    browser.server.last().write(hello());
    const waits = [await reconnected(() => browser.server.last().write(reset))];
    for (let i = 0; i < 7; i++) {
      // oxlint-disable-next-line no-await-in-loop -- the reconnections come one after another
      waits.push(await reconnected(() => browser.server.last().close()));
    }
    browser.server.last().write(hello());
    waits.push(await reconnected(() => browser.server.last().close()));

    expect(waits).toEqual([0, 1_000, 2_000, 4_000, 8_000, 16_000, 30_000, 30_000, 1_000]);
  });

  test("waits as Retry-After says when the server is not ready, and backs off when opening fails otherwise", async () => {
    const browser = browserOf(kind);
    browser.server.refuse(Object.assign(new Error("not ready"), { status: 503, retryAfter: 7 }));
    browser.server.refuse(new TypeError("Failed to fetch"));
    browser.tab("a");
    await settle(31_000);

    expect(gaps(browser.server)).toEqual([7_000, 1_000]);
    expect(browser.server.streams).toHaveLength(1);
  });

  test("gives up a stream silent for three heartbeats and connects again", async () => {
    const browser = browserOf(kind);
    browser.tab("a");
    await settle();
    browser.server.last().write(hello(5));
    await settle(15_000);
    expect(browser.server.streams).toHaveLength(1);

    await settle(10_000);

    expect(browser.server.streams).toHaveLength(2);
    expect(browser.server.live()).toHaveLength(1);
  });

  test("a visible tab does not take over from a holder that tells it is reconnecting", async () => {
    const browser = browserOf(kind);
    browser.tab("a");
    await settle();
    browser.tab("b");

    for (let i = 0; i < 6; i++) {
      browser.server.last().close();
      // oxlint-disable-next-line no-await-in-loop -- the reconnections come one after another
      await settle(31_000);
    }

    expect(new Set(browser.server.streams.map((s) => s.tab))).toEqual(new Set(["a"]));
  });

  test("a visible tab takes over from a holder silent for three heartbeats; a hidden one only once it is shown", async () => {
    const browser = browserOf(kind);
    // A holder frozen in the background: it holds the lead and says nothing.
    if (kind === "Web Locks") {
      void browser.locks.request(NAME, {}, () => new Promise(() => undefined)).catch(() => undefined);
    } else {
      browser.storage.write(NAME, JSON.stringify({ tab: "frozen", until: Date.now() + 3_600_000 }));
    }
    await settle();
    const hidden = browser.tab("hidden", { visible: false });
    await settle(5 * 60_000);
    expect(browser.server.streams).toHaveLength(0);

    browser.tab("visible");
    await settle(50_000);
    expect(browser.server.streams).toHaveLength(0);
    await settle(40_000);
    expect(browser.server.live().map((s) => s.tab)).toEqual(["visible"]);

    browser.server.last().write(hello());
    browser.server.last().write(beat);
    await settle();
    hidden.page.show(true);
    await settle();
    expect(browser.server.live().map((s) => s.tab)).toEqual(["visible"]);
  });

  test("a tab once hidden takes over as soon as it is shown again", async () => {
    const browser = browserOf(kind);
    if (kind === "Web Locks") {
      void browser.locks.request(NAME, {}, () => new Promise(() => undefined)).catch(() => undefined);
    } else {
      browser.storage.write(NAME, JSON.stringify({ tab: "frozen", until: Date.now() + 3_600_000 }));
    }
    const tab = browser.tab("a", { visible: false });
    await settle(5 * 60_000);
    expect(browser.server.streams).toHaveLength(0);

    tab.page.show(true);
    await settle();

    expect(browser.server.live().map((s) => s.tab)).toEqual(["a"]);
  });

  test.each([
    ["pagehide", "pageshow"],
    ["freeze", "resume"],
  ])("the holder lets go on %s, another tab takes over at once, and it takes part again on %s", async (away, back) => {
    const browser = browserOf(kind);
    const a = browser.tab("a");
    await settle();
    const b = browser.tab("b");
    await settle();
    browser.server.last().write(hello());
    await settle();

    a.page.fire(away);
    await settle();

    expect(browser.server.live().map((s) => s.tab)).toEqual(["b"]);
    a.page.fire(back);
    b.hub.stop();
    await settle();
    expect(browser.server.live().map((s) => s.tab)).toEqual(["a"]);
  });

  test("stop aborts the stream, lets the lead go and leaves nothing running; start holds again", async () => {
    const browser = browserOf(kind);
    const a = browser.tab("a");
    await settle();
    browser.server.last().write(hello());
    await settle();

    a.hub.stop();
    await settle();

    expect(browser.server.live()).toHaveLength(0);
    expect(vi.getTimerCount()).toBe(0);
    expect(a.page.listening()).toBe(0);
    expect(browser.locks.held(NAME)).toBe(false);
    expect(browser.storage.data.has(NAME)).toBe(false);

    a.hub.start();
    await settle();
    browser.server.last().write(hello());
    await settle();
    expect(browser.server.live().map((s) => s.tab)).toEqual(["a"]);
    expect(a.events).toEqual(["connected", "connected"]);
  });

  test("stopped and started at once, as StrictMode does, it holds the stream without waiting", async () => {
    const browser = browserOf(kind);
    const a = browser.tab("a");
    await settle();

    a.hub.stop();
    a.hub.start();
    await settle();

    expect(browser.server.streams).toHaveLength(2);
    expect(browser.server.live().map((s) => s.tab)).toEqual(["a"]);
  });

  test("a hub whose login has changed stops", async () => {
    const browser = browserOf(kind);
    browser.server.refuse(new SessionChangedError());
    const a = browser.tab("a");
    await settle(60_000);

    expect(browser.server.opens).toHaveLength(1);
    expect(vi.getTimerCount()).toBe(0);
    expect(a.page.listening()).toBe(0);
  });
});
