import type { Port } from "../channel";

// The BroadcastChannels of one browser for the tests: what a port posts
// reaches every other open port of the same name, in a microtask, never
// itself; a closed port throws as it is posted on, as a browser's does,
// and hears nothing; a deaf one, as a frozen page's, does not hear.

export class FakeChannels {
  readonly #ports = new Map<string, Set<FakePort>>();

  port(name: string): Port & { deaf: boolean } {
    const ports = this.#ports.get(name) ?? new Set<FakePort>();
    this.#ports.set(name, ports);
    const port = new FakePort(ports);
    ports.add(port);
    return port;
  }
}

class FakePort implements Port {
  readonly #listeners = new Set<(event: MessageEvent) => void>();
  #open = true;
  /** Whether what the others post is lost on this port. */
  deaf = false;

  constructor(private readonly ports: Set<FakePort>) {}

  postMessage(message: unknown): void {
    if (!this.#open) {
      throw new DOMException("Channel is closed", "InvalidStateError");
    }
    const data = structuredClone(message);
    for (const port of this.ports) {
      if (port !== this) {
        queueMicrotask(() => port.#deliver(data));
      }
    }
  }

  close(): void {
    this.#open = false;
    this.ports.delete(this);
  }

  addEventListener(_type: "message", listener: (event: MessageEvent) => void): void {
    this.#listeners.add(listener);
  }

  removeEventListener(_type: "message", listener: (event: MessageEvent) => void): void {
    this.#listeners.delete(listener);
  }

  #deliver(data: unknown): void {
    if (!this.#open || this.deaf) {
      return;
    }
    for (const listener of this.#listeners) {
      listener(new MessageEvent("message", { data }));
    }
  }
}
