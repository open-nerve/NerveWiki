import { expect, test, vi } from "vitest";

import type { User } from "../services/account.service";
import { AccountStore } from "./account.store";

const user = (display_name: string, onboarding_steps: string[] = []): User => ({
  id: "0199a2b4-0000-7000-8000-000000000001",
  email: "ada@example.com",
  display_name,
  onboarding_steps,
});

/** The service's calls these tests do not make. */
const notCalled = {
  changePassword: () => Promise.reject(new Error("not called")),
  deactivate: () => Promise.reject(new Error("not called")),
};

// The changes go out one at a time (M1/P5 design 3.3): the second is sent
// only once the first is answered, and the account kept is the last one
// answered.
test("sends the changes one at a time and keeps the last answer", async () => {
  const sent: string[] = [];
  const answers: ((u: User) => void)[] = [];
  const answer = (what: string) =>
    new Promise<User>((resolve) => {
      sent.push(what);
      answers.push(resolve);
    });
  const store = new AccountStore({
    ...notCalled,
    getMe: () => answer("get"),
    updateMe: (changes) => answer(`update ${changes.display_name}`),
    recordStep: (step) => answer(`record ${step}`),
  });

  const renamed = store.update({ display_name: "Ada" });
  const recorded = store.recordStep("profile");
  await Promise.resolve();
  expect(sent).toEqual(["update Ada"]);

  answers[0]?.(user("Ada"));
  await renamed;
  await vi.waitFor(() => expect(sent).toEqual(["update Ada", "record profile"]));
  answers[1]?.(user("Ada", ["profile"]));
  await recorded;

  expect(store.me).toEqual(user("Ada", ["profile"]));
});

// SWR reads the account again on focus, and a change may go out at the
// same moment: a read answered after the change may hold the account from
// before it, and must not undo it.
test("a read answered after a change keeps the change", async () => {
  let answerRead: ((u: User) => void) | undefined;
  const store = new AccountStore({
    ...notCalled,
    getMe: () => new Promise<User>((resolve) => (answerRead = resolve)),
    updateMe: async () => user("Ada"),
    recordStep: async (step) => user("ada", [step]),
  });

  const read = store.load();
  await store.recordStep("profile");
  answerRead?.(user("ada"));

  expect(await read).toEqual(user("ada", ["profile"]));
  expect(store.me).toEqual(user("ada", ["profile"]));
  // With no change meanwhile, a read keeps what it read.
  const next = store.load();
  answerRead?.(user("Ada lovelace", ["profile"]));
  expect(await next).toEqual(user("Ada lovelace", ["profile"]));
  expect(store.me).toEqual(user("Ada lovelace", ["profile"]));
});
