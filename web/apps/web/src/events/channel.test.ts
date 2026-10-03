import { describe, expect, test } from "vitest";

import { TabChannel, type Message } from "./channel";
import { FakeChannels } from "./testing/fake-channels";

describe("TabChannel", () => {
  test("hears its login's messages from the other tabs, not another login's, not its own, not once closed", async () => {
    const channels = new FakeChannels();
    const a = new TabChannel(channels.port("nwiki.events"), "login-1");
    const b = new TabChannel(channels.port("nwiki.events"), "login-1");
    const other = new TabChannel(channels.port("nwiki.events"), "login-2");
    const heard: Message[] = [];
    const heardByA: Message[] = [];
    b.listen((m) => heard.push(m));
    a.listen((m) => heardByA.push(m));

    a.post({ kind: "beat", heartbeatSeconds: 20 });
    other.post({ kind: "yield" });
    await Promise.resolve();
    b.close();
    a.post({ kind: "reconnecting", heartbeatSeconds: 20 });
    await Promise.resolve();

    expect(heard).toEqual([{ kind: "beat", heartbeatSeconds: 20, loginId: "login-1" }]);
    expect(heardByA).toEqual([]);
  });

  test("drops what is not a message, and stops hearing once unsubscribed", async () => {
    const channels = new FakeChannels();
    const port = channels.port("nwiki.events");
    const b = new TabChannel(channels.port("nwiki.events"), "login-1");
    const heard: Message[] = [];
    const unsubscribe = b.listen((m) => heard.push(m));

    // oxlint-disable-next-line unicorn/require-post-message-target-origin -- a BroadcastChannel has no target origin
    port.postMessage({ kind: "nonsense", loginId: "login-1" });
    // oxlint-disable-next-line unicorn/require-post-message-target-origin -- a BroadcastChannel has no target origin
    port.postMessage(null);
    await Promise.resolve();
    unsubscribe();
    // oxlint-disable-next-line unicorn/require-post-message-target-origin -- a BroadcastChannel has no target origin
    port.postMessage({ kind: "yield", loginId: "login-1" });
    await Promise.resolve();

    expect(heard).toEqual([]);
  });
});
