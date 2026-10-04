import { expect, test } from "vitest";

import { browserPageLifecycle } from "./deps";

// The page's lifecycle as the browser fires it (M4–M5 Codex review R6):
// pagehide and pageshow on the window, the others on the document; the
// hub's tests fire them on a fake page, which does not tell where.

const targets = { pagehide: "window", pageshow: "window", freeze: "document", resume: "document" } as const;

test.each(Object.entries(targets))("%s is heard on the %s, and on the other no more", (event, where) => {
  const heard: { persisted?: boolean }[] = [];
  const off = browserPageLifecycle().on(event as keyof typeof targets, (fired) => heard.push(fired));
  const [on, other] = where === "window" ? [window, document] : [document, window];

  other.dispatchEvent(new Event(event));
  expect(heard).toEqual([]);
  on.dispatchEvent(new Event(event));
  expect(heard).toEqual([{ persisted: false }]);

  off();
  on.dispatchEvent(new Event(event));
  expect(heard).toHaveLength(1);
});

test("visibilitychange is heard on the document, and the page is visible as the document says", () => {
  let heard = 0;
  const page = browserPageLifecycle();
  const off = page.on("visibilitychange", () => (heard += 1));

  document.dispatchEvent(new Event("visibilitychange"));
  expect(heard).toBe(1);
  expect(page.visible()).toBe(document.visibilityState === "visible");
  off();
});
