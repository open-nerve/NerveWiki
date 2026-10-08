import { expect, test } from "vitest";

import { UnloadWarning } from "./unload-warning";

/** leaving is whether the page, left now, would ask first: its beforeunload's default prevented. */
function leaving(target: EventTarget): boolean {
  const event = new Event("beforeunload", { cancelable: true });
  target.dispatchEvent(event);
  return event.defaultPrevented;
}

test("the page asks before it is left while any holder asks it to, and no longer once none does", () => {
  const target = new EventTarget();
  const warning = new UnloadWarning(target);
  expect(leaving(target)).toBe(false);

  warning.set("n1", true);
  warning.set("n2", true);
  warning.set("n1", true);
  expect(leaving(target)).toBe(true);
  warning.set("n1", false);
  expect(leaving(target)).toBe(true);
  warning.set("n2", false);
  expect(leaving(target)).toBe(false);
  warning.set("n2", false);
  expect(leaving(target)).toBe(false);
});
