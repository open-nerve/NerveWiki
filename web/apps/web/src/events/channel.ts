import type { EventLock, EventPages } from "../services/event.service";

// The messages between the tabs of a login (M5 design 4.11): the holder of
// the stream forwards its events and tells it is alive, with the server's
// heartbeat, which the others need to tell a silent holder.

/** An event of the stream, as each tab handles it. */
type StreamEvent = { type: "pages"; data: EventPages } | { type: "lock"; data: EventLock };

/**
 * What the holder tells: an event; that it connected (each tab refreshes
 * what it shows); a heartbeat; that it is reconnecting; that it let go.
 */
export type Message =
  | { kind: "event"; event: StreamEvent }
  | { kind: "connected"; heartbeatSeconds: number }
  | { kind: "beat"; heartbeatSeconds: number }
  | { kind: "reconnecting"; heartbeatSeconds: number }
  | { kind: "yield" };

/** The part of a BroadcastChannel the tabs use. */
export type Port = {
  postMessage(message: unknown): void;
  close(): void;
  addEventListener(type: "message", listener: (event: MessageEvent) => void): void;
  removeEventListener(type: "message", listener: (event: MessageEvent) => void): void;
};

const kinds = new Set(["event", "connected", "beat", "reconnecting", "yield"]);

/**
 * TabChannel sends and receives the messages of one login over port: each
 * carries the login's id, and another login's are dropped, as a tab of the
 * next login may still hear the last one's.
 */
export class TabChannel {
  constructor(
    private readonly port: Port,
    private readonly loginId: string
  ) {}

  post(message: Message): void {
    // oxlint-disable-next-line unicorn/require-post-message-target-origin -- a BroadcastChannel has no target origin
    this.port.postMessage({ ...message, loginId: this.loginId });
  }

  /** listen calls listener with each message of this login from another tab; it returns the unsubscribe. */
  listen(listener: (message: Message) => void): () => void {
    const handle = (event: MessageEvent) => {
      const data = event.data as (Message & { loginId?: unknown }) | null;
      if (data && data.loginId === this.loginId && kinds.has(data.kind)) {
        listener(data);
      }
    };
    this.port.addEventListener("message", handle);
    return () => this.port.removeEventListener("message", handle);
  }

  close(): void {
    this.port.close();
  }
}
