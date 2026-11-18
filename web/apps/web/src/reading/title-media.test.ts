import { afterEach, expect, test, vi } from "vitest";

import { translator } from "../i18n/i18n";
import type { ReadingContext } from "./enhancement";
import { titleMedia } from "./title-media";

// A control of a medium in a folded callout's title (M12/P2/S5, 15.2). jsdom draws no controls: the events are
// dispatched and open is set by hand, in the order Chromium has them as it toggles the callout (the medium's
// DOMActivate, then the toggle).

afterEach(() => {
  vi.useRealTimers();
  document.body.replaceChildren();
});

const context: ReadingContext = {
  workspace: "lab",
  notebook: "n",
  page: "p",
  revision: 1,
  role: "reader",
  t: translator("en"),
  locale: "en",
  theme: () => "light",
  onThemeChange: () => () => undefined,
  reload: () => {},
  assetsExpire: null,
  assetAddress: () => Promise.reject(new Error("no attachments")),
  navigate: () => {},
  report: () => {},
  unresolved: () => {},
};

/** callout is a view of a callout, folded unless open, an audio in its title and another in its body. */
function callout(open = false) {
  vi.useFakeTimers();
  const container = document.createElement("article");
  container.innerHTML =
    `<details class="nw-callout"${open ? " open" : ""}><summary>` +
    `<audio class="nw-asset" controls="" preload="none" aria-label="talk.mp3"></audio> Talk</summary>` +
    `<p>body <audio class="nw-asset" controls="" preload="none" aria-label="inner.mp3"></audio></p></details>`;
  document.body.append(container);
  const undo = titleMedia(container, context);
  const details = container.querySelector("details") as HTMLDetailsElement;
  const [title, inner] = [...container.querySelectorAll("audio")];
  return { details, summary: container.querySelector("summary") as HTMLElement, title, inner, undo };
}

/** activate dispatches DOMActivate at target, as the browser does on a click it takes. */
const activate = (target: Element | undefined) =>
  target?.dispatchEvent(new UIEvent("DOMActivate", { bubbles: true, cancelable: true }));

test("a toggle that a control of a medium in the title caused is undone", () => {
  const { details, title } = callout();

  activate(title);
  details.open = true;
  vi.advanceTimersToNextFrame();

  expect(details.open).toBe(false);
});

test("the reader's toggle stays: the title's text", () => {
  const { details, summary } = callout();

  activate(summary);
  details.open = true;
  vi.advanceTimersToNextFrame();

  expect(details.open).toBe(true);
});

test("a medium out of a callout's title is not the callout's", () => {
  const { details, inner } = callout();

  activate(inner);
  details.open = true;
  vi.advanceTimersToNextFrame();

  expect(details.open).toBe(true);
});

test("a callout opened by the app after a click on the control stays open", () => {
  const { details, title } = callout();
  activate(title);
  vi.advanceTimersToNextFrame();

  details.open = true;
  vi.advanceTimersToNextFrame();

  expect(details.open).toBe(true);
});

test("a control's click that toggled nothing leaves the callout alone", async () => {
  const { details, title } = callout(true);
  const records: MutationRecord[] = [];
  const observer = new MutationObserver((list) => records.push(...list));
  observer.observe(details, { attributes: true });

  activate(title);
  vi.advanceTimersToNextFrame();
  // Taken as they are: the observer's callback has not run, the frame advanced at once.
  records.push(...observer.takeRecords());
  await Promise.resolve();

  expect(records).toEqual([]);
  observer.disconnect();
});

test("undone, the enhancement puts nothing back", () => {
  const { details, title, undo } = callout();
  undo?.();

  activate(title);
  details.open = true;
  vi.advanceTimersToNextFrame();

  expect(details.open).toBe(true);
});
