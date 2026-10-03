import type { StreamEvent, Message, TabChannel } from "./channel";
import { connect, type Open } from "./connection";
import type { Frame } from "./frames";
import type { Leadership } from "./leadership";

// One login's event stream in this tab (M5 design 4.11): the tab that holds
// it reads the stream and forwards it to the others, which hear it on the
// channel; either way the tab's subscribers get each event, and a
// "connected" whenever the stream connects, after which they refresh what
// they show: what was written while it was not connected went unseen.

/** What a tab's subscribers get. */
export type HubEvent = StreamEvent | { type: "connected" };

/** The page as the hub follows it: visible or not, hidden for good or frozen, back. */
export type PageLifecycle = {
  visible(): boolean;
  on(event: "visibilitychange" | "pagehide" | "pageshow" | "freeze" | "resume", listener: () => void): () => void;
};

export type EventHubDeps = {
  open: Open;
  /** A new part in the election and a new channel for each start. */
  leadership: () => Leadership;
  channel: () => TabChannel;
  page: PageLifecycle;
  now: () => number;
};

/** The heartbeat until the holder's hello tells: the server's default. */
const DEFAULT_HEARTBEAT_SECONDS = 20;
/** How many heartbeats a holder may be silent before a visible tab takes over, and a lease lasts. */
const SILENT_BEATS = 3;
const FIRST_BACKOFF_MS = 1_000;
const MAX_BACKOFF_MS = 30_000;

export class EventHub {
  readonly #listeners = new Set<(event: HubEvent) => void>();
  #stop: AbortController | undefined;
  /** The end of the last start's election: the next one starts after it, a lead or a lease never held twice. */
  #ended: Promise<void> = Promise.resolve();
  #heartbeatSeconds = DEFAULT_HEARTBEAT_SECONDS;
  #heard = 0;
  #leading = false;

  constructor(private readonly deps: EventHubDeps) {}

  /** subscribe calls listener with each event of this tab; it returns the unsubscribe. */
  subscribe(listener: (event: HubEvent) => void): () => void {
    this.#listeners.add(listener);
    return () => this.#listeners.delete(listener);
  }

  /** start takes part in the election and listens to the other tabs, until stop; a stopped hub starts again. */
  start(): void {
    if (this.#stop) {
      return;
    }
    const stop = new AbortController();
    this.#stop = stop;
    const leadership = this.deps.leadership();
    const channel = this.deps.channel();
    this.#heard = this.deps.now();
    const offs = [
      channel.listen((message) => this.#hear(message)),
      this.deps.page.on("pagehide", () => this.#yield(leadership, channel)),
      this.deps.page.on("freeze", () => this.#yield(leadership, channel)),
      this.deps.page.on("pageshow", () => leadership.rejoin()),
      this.deps.page.on("resume", () => leadership.rejoin()),
      this.deps.page.on("visibilitychange", () => this.#check(leadership)),
    ];
    let watch: ReturnType<typeof setTimeout> | undefined;
    const tick = () => {
      this.#check(leadership);
      watch = setTimeout(tick, this.#heartbeatSeconds * 1000);
    };
    watch = setTimeout(tick, this.#heartbeatSeconds * 1000);
    stop.signal.addEventListener(
      "abort",
      () => {
        clearTimeout(watch);
        for (const off of offs) off();
        channel.close();
      },
      { once: true }
    );
    this.#ended = this.#ended
      .catch(() => undefined)
      .then(() =>
        stop.signal.aborted ? undefined : leadership.run((lost) => this.#lead(lost, leadership, channel), stop.signal)
      );
  }

  /** stop aborts the connection, lets the lead go, and closes the channel. */
  stop(): void {
    this.#stop?.abort();
    this.#stop = undefined;
  }

  /** yield lets the lead go, telling the others, which then wait for the next holder rather than take over. */
  #yield(leadership: Leadership, channel: TabChannel): void {
    if (this.#leading) {
      channel.post({ kind: "yield" });
    }
    leadership.yield();
  }

  /** check takes the lead from a holder silent for SILENT_BEATS heartbeats, if this tab is visible. */
  #check(leadership: Leadership): void {
    if (this.#leading || !this.deps.page.visible()) {
      return;
    }
    if (this.deps.now() - this.#heard > SILENT_BEATS * this.#heartbeatSeconds * 1000) {
      this.#heard = this.deps.now();
      leadership.steal();
    }
  }

  #hear(message: Message): void {
    this.#heard = this.deps.now();
    switch (message.kind) {
      case "event":
        this.#emit(message.event);
        break;
      case "connected":
        this.#heartbeatSeconds = message.heartbeatSeconds;
        this.#emit({ type: "connected" });
        break;
      case "beat":
      case "reconnecting":
        this.#heartbeatSeconds = message.heartbeatSeconds;
        break;
      case "yield":
        break;
    }
  }

  /**
   * lead holds the stream until lost aborts: it connects again at once after
   * the server's reset, after Retry-After when the server is not ready, and
   * after a backoff otherwise, a connection silent for SILENT_BEATS
   * heartbeats (a link the network dropped without a word) included;
   * meanwhile it tells the others it is reconnecting, so that they do not
   * take over. Once the login has changed, the hub stops.
   */
  async #lead(lost: AbortSignal, leadership: Leadership, channel: TabChannel): Promise<void> {
    this.#leading = true;
    let backoff = FIRST_BACKOFF_MS;
    let connected = false;
    let connection = new AbortController();
    let silent = false;
    let heard = this.deps.now();
    let timer: ReturnType<typeof setTimeout> | undefined;
    const tell = () => {
      if (this.deps.now() - heard > this.#ttl()) {
        silent = true;
        connection.abort();
      }
      if (!connected) {
        leadership.renew(this.#ttl());
        channel.post({ kind: "reconnecting", heartbeatSeconds: this.#heartbeatSeconds });
      }
      timer = setTimeout(tell, this.#heartbeatSeconds * 1000);
    };
    timer = setTimeout(tell, this.#heartbeatSeconds * 1000);
    const onFrame = (frame: Frame) => {
      heard = this.deps.now();
      if (frame.type === "hello") {
        connected = true;
        backoff = FIRST_BACKOFF_MS;
        this.#heartbeatSeconds = frame.data.heartbeat_seconds;
      }
      leadership.renew(this.#ttl());
      this.#forward(frame, channel);
    };
    const cut = () => connection.abort();
    lost.addEventListener("abort", cut, { once: true });
    while (!lost.aborted) {
      connected = false;
      connection = new AbortController();
      silent = false;
      heard = this.deps.now();
      // oxlint-disable-next-line no-await-in-loop -- one connection after another
      const ending = await connect(this.deps.open, connection.signal, onFrame);
      if (lost.aborted) {
        break;
      }
      if (ending.ended === "aborted" && !silent) {
        this.stop();
        break;
      }
      if (ending.ended === "reset") {
        continue;
      }
      const retryAfter = ending.ended === "failed" ? retryAfterOf(ending.error) : undefined;
      // oxlint-disable-next-line no-await-in-loop -- one connection after another
      await pause(retryAfter === undefined ? backoff : retryAfter * 1000, lost);
      if (retryAfter === undefined) {
        backoff = Math.min(backoff * 2, MAX_BACKOFF_MS);
      }
    }
    lost.removeEventListener("abort", cut);
    clearTimeout(timer);
    this.#leading = false;
    this.#heard = this.deps.now();
  }

  /** forward hands a frame of the stream to this tab's subscribers and to the other tabs. */
  #forward(frame: Frame, channel: TabChannel): void {
    switch (frame.type) {
      case "hello":
        channel.post({ kind: "connected", heartbeatSeconds: this.#heartbeatSeconds });
        this.#emit({ type: "connected" });
        break;
      case "pages":
      case "lock": {
        const event: StreamEvent =
          frame.type === "pages" ? { type: "pages", data: frame.data } : { type: "lock", data: frame.data };
        channel.post({ kind: "event", event });
        this.#emit(event);
        break;
      }
      case "beat":
        channel.post({ kind: "beat", heartbeatSeconds: this.#heartbeatSeconds });
        break;
      case "reset":
      case "other":
        break;
    }
  }

  #emit(event: HubEvent): void {
    for (const listener of this.#listeners) {
      listener(event);
    }
  }

  /** How long a lease lasts: SILENT_BEATS heartbeats. */
  #ttl(): number {
    return SILENT_BEATS * this.#heartbeatSeconds * 1000;
  }
}

/** The seconds an error's Retry-After asked to wait, if any: the ApiError of a 503 not_ready. */
function retryAfterOf(error: unknown): number | undefined {
  const retryAfter = (error as { retryAfter?: unknown } | null)?.retryAfter;
  return typeof retryAfter === "number" ? retryAfter : undefined;
}

/** pause waits ms, or until signal aborts. */
function pause(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    if (signal.aborted) {
      resolve();
      return;
    }
    const done = () => {
      clearTimeout(timer);
      signal.removeEventListener("abort", done);
      resolve();
    };
    const timer = setTimeout(done, ms);
    signal.addEventListener("abort", done, { once: true });
  });
}
