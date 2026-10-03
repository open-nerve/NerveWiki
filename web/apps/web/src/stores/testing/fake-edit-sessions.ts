import { vi } from "vitest";

import type { HubEvent } from "../../events/hub";
import { FakePage } from "../../events/testing/fake-page";
import { ApiError } from "../../services/api";
import { EditSession, type SessionService } from "../edit-session";

// Edit sessions for the tests (M5/P4 design 3.2): a service whose answers
// the test sets, the page, the login's events, the ends sent as the page is
// left.

/** refusal is the API's answer status with code, and the problem's other members. */
export const refusal = (status: number, code: string, members: object = {}, retryAfter?: number) =>
  new ApiError(status, { status, code, title: code, ...members }, retryAfter);

/** lockedBy is page.locked, naming name as the holder. */
export const lockedBy = (name: string) =>
  refusal(409, "page.locked", { lock: { page_id: "p1", user_id: `u-${name}`, display_name: name } });

type Answer<T> = () => T | Promise<T>;

/**
 * fakeSessions answers the opens, beats and ends of edit sessions, as set
 * in answers; what went out is in sent: OPEN (TAKE when taking over),
 * BEAT id, END id.
 */
function fakeSessions(sent: string[] = []) {
  let opened = 0;
  const answers = {
    open: undefined as Answer<{ id: string }> | undefined,
    beat: undefined as Answer<unknown> | undefined,
    end: undefined as Answer<void> | undefined,
  };
  const service = {
    openEditSession: vi.fn(async (_: string, takeOver?: boolean) => {
      sent.push(takeOver === true ? "OPEN TAKE" : "OPEN");
      return answers.open === undefined ? { id: `s${++opened}` } : await answers.open();
    }) as never,
    heartbeatEditSession: vi.fn(async (id: string) => {
      sent.push(`BEAT ${id}`);
      return answers.beat === undefined ? {} : await answers.beat();
    }) as never,
    endEditSession: vi.fn(async (id: string) => {
      sent.push(`END ${id}`);
      await answers.end?.();
    }),
  } satisfies SessionService;
  return { service, sent, answers };
}

/** fakeEvents is a login's events: the test emits them to the subscribers. */
function fakeEvents() {
  const listeners = new Set<(event: HubEvent) => void>();
  return {
    subscribe: (listener: (event: HubEvent) => void) => {
      listeners.add(listener);
      return () => void listeners.delete(listener);
    },
    emit: (event: HubEvent) => {
      for (const listener of listeners) {
        listener(event);
      }
    },
    count: () => listeners.size,
  };
}

/** lockEvent is the lock event of the page pageId's session sessionId. */
export const lockEvent = (pageId: string, sessionId = "s1"): HubEvent => ({
  type: "lock",
  data: { workspace_id: "w1", notebook_id: "n1", page_id: pageId, session_id: sessionId },
});

/** fakeSession is the edit session of p1, over fakeSessions, a FakePage and fakeEvents; left has the ends sent as the page was left. */
export function fakeSession(sent: string[] = []) {
  const sessions = fakeSessions(sent);
  const page = new FakePage();
  const events = fakeEvents();
  const left: string[] = [];
  const session = new EditSession(
    { service: sessions.service, page, events: events.subscribe, leave: (id) => left.push(id) },
    "p1"
  );
  return { ...sessions, page, events, left, session };
}
