import { afterEach, expect, onTestFinished, test, vi } from "vitest";

import { translator } from "../i18n/i18n";
import type { ReadingContext } from "./enhancement";
import { expiredDownloads } from "./expired-downloads";

// A download clicked once the view's addresses have expired (M12/P2/S5, 17.4).

afterEach(() => {
  vi.useRealTimers();
  document.body.replaceChildren();
});

/** address is the address of the attachment id's content as signed at sig. */
const address = (id: string, sig = "1") => `/api/v0/assets/${id}/content?b=b1&e=${sig}&s=s`;

/** expired and later are when a view's addresses expire: gone by, and to come. */
const expired = "2026-10-09T10:00:00Z";
const later = "2026-10-09T13:00:00Z";
const now = new Date("2026-10-09T12:00:00Z");

/**
 * setUp is a view of html enhanced, its addresses expiring at expires (as
 * it is now: expiry changes it), that records its reloads and signs an
 * attachment's address anew (signed) unless it is gone, answering in a
 * task of its own as the network does; and each click that reached the
 * document, its address and whether it was held, which jsdom does not
 * follow.
 */
function setUp(html: string, expires: string | null = expired) {
  vi.useFakeTimers({ now, toFake: ["Date"] });
  const container = document.createElement("article");
  container.innerHTML = html;
  document.body.append(container);
  const reloads: string[] = [];
  const signed: string[] = [];
  const clicks: { href: string | null; prevented: boolean }[] = [];
  let expiry = expires;
  const context: ReadingContext = {
    workspace: "lab",
    notebook: "n",
    page: "p",
    revision: 1,
    t: translator("en"),
    locale: "en",
    theme: () => "light",
    onThemeChange: () => () => undefined,
    reload: () => reloads.push("p"),
    navigate: () => undefined,
    report: () => undefined,
    unresolved: () => undefined,
    // The view's as it is now, as the reading view has it: its addresses signed anew in place (M12/P2/S4, 17.1).
    get assetsExpire() {
      return expiry;
    },
    assetAddress: async (id) => {
      signed.push(id);
      // A task, not a microtask: one that clicked it again and again would leave the test no turn (review C's check 2).
      await new Promise((resolve) => void setTimeout(resolve, 0));
      if (id === "gone") {
        throw new Error("404");
      }
      return address(id, "2");
    },
  };
  const record = (event: MouseEvent) => {
    const link = event.target instanceof Element ? event.target.closest("a") : null;
    clicks.push({ href: link?.getAttribute("href") ?? null, prevented: event.defaultPrevented });
    event.preventDefault();
  };
  document.addEventListener("click", record);
  onTestFinished(() => document.removeEventListener("click", record));
  const undo = expiredDownloads(container, context);
  onTestFinished(() => undo?.());
  const link = (name: string) => {
    const found = [...container.querySelectorAll("a")].find((each) => each.textContent === name);
    if (found === undefined) {
      throw new Error(`no link ${name}`);
    }
    return found;
  };
  return {
    container,
    reloads,
    signed,
    clicks,
    link,
    expiry: (to: string | null) => {
      expiry = to;
    },
  };
}

/** settled lets what the clicks began settle (the dates alone are fake): the address signed, the link clicked again. */
const settled = () => new Promise<void>((resolve) => void setTimeout(resolve, 0));

const download = (id: string, name: string) =>
  `<p><a class="nw-asset" href="${address(id).replace(/&/g, "&amp;")}" download="">${name}</a></p>`;

test("a download clicked once the addresses expired is signed anew, goes from the new address, and the view is read again", async () => {
  const { link, signed, clicks, reloads } = setUp(download("f1", "f.zip"));

  link("f.zip").click();
  await settled();

  expect(signed).toEqual(["f1"]);
  expect(clicks).toEqual([
    { href: address("f1"), prevented: true },
    { href: address("f1", "2"), prevented: false },
  ]);
  expect(reloads).toEqual(["p"]);
});

test("a download signed anew, its address changed in place since, is signed anew again as it expires again", async () => {
  const { link, signed, clicks, reloads, expiry } = setUp(download("f1", "f.zip"));
  link("f.zip").click();
  await settled();
  expect(signed).toEqual(["f1"]);

  // The view read again: its addresses signed for the next hour, given to the link in place, the enhancement kept.
  link("f.zip").setAttribute("href", address("f1", "3"));
  expiry(later);
  link("f.zip").click();
  await settled();
  expect(signed).toEqual(["f1"]);
  expect(clicks.at(-1)).toEqual({ href: address("f1", "3"), prevented: false });

  // An hour on, they have expired again.
  vi.setSystemTime(new Date("2026-10-09T13:30:00Z"));
  link("f.zip").click();
  await settled();

  expect(signed).toEqual(["f1", "f1"]);
  expect(clicks.slice(-2)).toEqual([
    { href: address("f1", "3"), prevented: true },
    { href: address("f1", "2"), prevented: false },
  ]);
  expect(reloads).toEqual(["p", "p"]);
});

test.each([
  ["a download clicked before they expire", download("f1", "f.zip"), later, "f.zip", address("f1")],
  [
    "a link that opens in a tab of its own",
    `<p><a class="nw-asset" href="${address("d1").replace(/&/g, "&amp;")}">d.pdf</a></p>`,
    expired,
    "d.pdf",
    address("d1"),
  ],
])("%s goes as it is", async (_, html, expires, name, href) => {
  const { link, signed, clicks, reloads } = setUp(html, expires);

  link(name).click();
  await settled();

  expect(signed).toEqual([]);
  expect(clicks).toEqual([{ href, prevented: false }]);
  expect(reloads).toEqual([]);
});

test("one whose address cannot be signed goes as it is: the browser says it failed", async () => {
  const { link, signed, clicks } = setUp(download("gone", "g.zip"));

  link("g.zip").click();
  await settled();

  expect(signed).toEqual(["gone"]);
  expect(clicks).toEqual([
    { href: address("gone"), prevented: true },
    { href: address("gone"), prevented: false },
  ]);
});

test("a click with a key held goes as the browser has it", async () => {
  const { link, signed, clicks } = setUp(download("f1", "f.zip"));

  link("f.zip").dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true, ctrlKey: true }));
  await settled();

  expect(signed).toEqual([]);
  expect(clicks).toEqual([{ href: address("f1"), prevented: false }]);
});
