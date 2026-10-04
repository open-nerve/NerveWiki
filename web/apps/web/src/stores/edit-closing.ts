import type { Port } from "../events/channel";

// A sign-out closes the edits of every tab of its login (M4–M5 Codex review
// R2): the logout ends the session the other tabs save with, so the tab
// signing out first asks them to save and end their edits, as it does its
// own, and waits for those that have edits to say they did.

/** What the tabs tell: close your edits; I have edits and close them; I closed them. */
type Message = { loginId: string; kind: "close" | "closing" | "closed"; id: string; tab: string };

/** How long the tabs that have edits have to say so: a channel's message comes within milliseconds. */
export const closingAnswerWait = 100;

/**
 * answerClosings closes this tab's edits through close each time another
 * tab of the login asks, while it has edits (hasEdits): it tells that it
 * closes them, and once close resolves that it did, unless unsubscribed
 * by then. It returns the unsubscribe.
 */
export function answerClosings(
  port: Port,
  { loginId, tabId }: { loginId: string; tabId: string },
  hasEdits: () => boolean,
  close: () => Promise<void>
): () => void {
  let subscribed = true;
  const tell = (kind: "closing" | "closed", id: string) => {
    // Unsubscribed, the port may be closed: posting on it would throw.
    if (subscribed) {
      post(port, { loginId, kind, id, tab: tabId });
    }
  };
  const handle = (event: MessageEvent) => {
    const asked = messageOf(event, loginId);
    if (asked?.kind !== "close" || asked.tab === tabId || !hasEdits()) {
      return;
    }
    tell("closing", asked.id);
    void close()
      .catch(() => undefined)
      .then(() => tell("closed", asked.id));
  };
  port.addEventListener("message", handle);
  return () => {
    subscribed = false;
    port.removeEventListener("message", handle);
  };
}

/**
 * closeOtherTabs asks the login's other tabs to close their edits, and
 * resolves once each that said it has edits said it closed them, or none
 * said so within closingAnswerWait, or after within ms at most.
 */
export function closeOtherTabs(
  port: Port,
  { loginId, tabId }: { loginId: string; tabId: string },
  id: string,
  within: number
): Promise<void> {
  return new Promise((resolve) => {
    const closing = new Set<string>();
    let answered = false;
    const done = () => {
      clearTimeout(answers);
      clearTimeout(limit);
      port.removeEventListener("message", handle);
      resolve();
    };
    const handle = (event: MessageEvent) => {
      const told = messageOf(event, loginId);
      if (told === undefined || told.id !== id) {
        return;
      }
      if (told.kind === "closing") {
        closing.add(told.tab);
      } else if (told.kind === "closed") {
        closing.delete(told.tab);
        if (answered && closing.size === 0) {
          done();
        }
      }
    };
    port.addEventListener("message", handle);
    const answers = setTimeout(
      () => {
        answered = true;
        if (closing.size === 0) {
          done();
        }
      },
      Math.min(closingAnswerWait, within)
    );
    const limit = setTimeout(done, within);
    post(port, { loginId, kind: "close", id, tab: tabId });
  });
}

function post(port: Port, message: Message): void {
  // oxlint-disable-next-line unicorn/require-post-message-target-origin -- a BroadcastChannel has no target origin
  port.postMessage(message);
}

/** messageOf is event's message of the login, undefined for anything else. */
function messageOf(event: MessageEvent, loginId: string): Message | undefined {
  const data = event.data as Partial<Message> | null;
  return data?.loginId === loginId &&
    (data.kind === "close" || data.kind === "closing" || data.kind === "closed") &&
    typeof data.id === "string" &&
    typeof data.tab === "string"
    ? (data as Message)
    : undefined;
}
