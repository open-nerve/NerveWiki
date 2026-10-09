import { afterEach, expect, onTestFinished, test, vi } from "vitest";

import { translator } from "../i18n/i18n";
import { assets } from "./assets";
import type { ReadingContext } from "./enhancement";

// The attachments of the reading view (M7/P4 design 4.4–4.6).

afterEach(() => {
  vi.useRealTimers();
  document.body.replaceChildren();
});

/** address is the address of the attachment id's content as signed at sig. */
const address = (id: string, sig = "1") => `/api/v0/assets/${id}/content?b=b1&amp;e=${sig}&amp;s=s`;

const audio = (id: string, label = id) =>
  `<audio class="nw-asset" src="${address(id)}" controls="" preload="none" aria-label="${label}"></audio>`;
/** focusable is audio's, taking the focus. */
const focusable = (id: string) => audio(id).replace("<audio ", '<audio tabindex="0" ');
const video = (id: string, label = id, width = "300") =>
  `<video class="nw-asset" src="${address(id)}" controls="" preload="none" aria-label="${label}" width="${width}"></video>`;

/** expired and later are when a view's addresses expire: gone by, and to come. */
const expired = "2026-10-09T10:00:00Z";
const later = "2026-10-09T13:00:00Z";
const now = new Date("2026-10-09T12:00:00Z");

/**
 * setUp is a view of html, an enhancement's own (each keeps what it kept),
 * and a context of page whose addresses expire at expires, that records
 * its reloads and signs an attachment's address anew (signed) unless it
 * is gone.
 */
function setUp(html: string, { page = "p", expires = expired as string | null } = {}) {
  vi.useFakeTimers({ now, toFake: ["Date"] });
  const container = document.createElement("article");
  container.innerHTML = html;
  document.body.append(container);
  const reloads: string[] = [];
  const signed: string[] = [];
  const context = (at = page, expiring = expires): ReadingContext => ({
    workspace: "lab",
    notebook: "n",
    page: at,
    revision: 1,
    role: "reader",
    t: translator("en"),
    locale: "en",
    theme: () => "light",
    onThemeChange: () => () => undefined,
    reload: () => reloads.push(at),
    navigate: () => undefined,
    report: () => undefined,
    unresolved: () => undefined,
    assetsExpire: expiring,
    assetAddress: async (id) => {
      signed.push(id);
      if (id === "gone") {
        throw new Error("404");
      }
      return `/api/v0/assets/${id}/content?anew=1`;
    },
  });
  return { container, context, reloads, signed, enhancement: assets() };
}

/**
 * play has element be as one started, playing (or paused) at time, as a
 * browser has it: a new address loads it anew, paused, at the start, at
 * the default rate, without its error; play plays it.
 */
function play(element: Element | null | undefined, time: number, paused = false) {
  if (!(element instanceof HTMLMediaElement)) {
    throw new Error("no media");
  }
  const state: Record<string, unknown> = { paused, currentTime: time, playbackRate: 1, error: null };
  for (const name of Object.keys(state)) {
    Object.defineProperty(element, name, {
      get: () => state[name],
      set: (value: unknown) => {
        state[name] = value;
      },
      configurable: true,
    });
  }
  const setAttribute = element.setAttribute.bind(element);
  element.setAttribute = (name: string, value: string) => {
    if (name === "src") {
      Object.assign(state, { paused: true, currentTime: 0, playbackRate: 1, error: null });
    }
    setAttribute(name, value);
  };
  element.play = vi.fn(async () => {
    state.paused = false;
  });
  return element;
}

/** broken has element, as play has it, fail for good: its error set, as a browser's after a failed load. */
function broken(element: HTMLMediaElement | undefined) {
  Object.assign(element ?? {}, { error: { code: 2 } });
}

/** fail has element fail to load, as the browser tells it: an error event, which does not bubble. */
function fail(element: Element | null | undefined) {
  element?.dispatchEvent(new Event("error"));
}

/** settle lets the promises out settle. */
async function settle() {
  for (let i = 0; i < 5; i++) {
    // oxlint-disable-next-line no-await-in-loop -- a turn at a time
    await Promise.resolve();
  }
}

test("a link to an attachment the browser shows opens in a tab of its own, as it says unseen; a download is as it was", () => {
  const html =
    `<p><a class="nw-wikilink nw-asset" href="${address("a1")}" data-nw-size="8">doc.pdf</a> ` +
    `<a class="nw-asset" href="${address("a2")}" data-nw-size="9" download="">a.zip</a> ` +
    `<a class="nw-asset">gone.png</a> <a href="https://x.example/">x</a></p>`;
  const { container, context, enhancement } = setUp(html);
  const before = container.innerHTML;
  const undo = enhancement(container, context());
  const [pdf, zip, gone, other] = container.querySelectorAll("a");
  expect(pdf?.getAttribute("target")).toBe("_blank");
  expect(pdf?.getAttribute("rel")).toBe("noopener noreferrer");
  expect(pdf?.querySelector("span.sr-only")?.textContent).toBe("(opens in a new tab)");
  expect(pdf?.textContent).toBe("doc.pdf (opens in a new tab)");
  for (const link of [zip, gone, other]) {
    expect(link?.hasAttribute("target")).toBe(false);
    expect(link?.querySelector("span")).toBeNull();
  }
  // Each says its size after it, one without an address none.
  expect(container.textContent).toBe("doc.pdf (opens in a new tab) (8 B) a.zip (9 B) gone.png x");
  expect([...container.querySelectorAll(".nw-size")].map((size) => size.previousElementSibling)).toEqual([pdf, zip]);
  undo?.();
  expect(container.innerHTML).toBe(before);
});

test("the hint and the size are in the reader's language", () => {
  const { container, context, enhancement } = setUp(
    `<a class="nw-asset" href="${address("a1")}" data-nw-size="1536">x.png</a>`
  );
  enhancement(container, { ...context(), t: translator("zh-CN"), locale: "zh-CN" });
  expect(container.textContent).toBe("x.png （在新标签页打开）（1.5 KB）");
});

test("an attachment that fails to load once the addresses expired reads the view again, once for the page's expiry", () => {
  const html =
    `<p><img class="nw-asset" src="${address("a1")}" alt="x"> <img class="nw-asset" src="${address("a2")}" alt="y"> ` +
    `${audio("a3")} <img src="https://x.example/y.png" alt="z"></p>`;
  const { container, context, reloads, enhancement } = setUp(html);
  let undo = enhancement(container, context());
  const [x, y] = container.querySelectorAll("img.nw-asset");
  // Another site's image is not the view's to read again.
  fail(container.querySelector("img:not(.nw-asset)"));
  expect(reloads).toEqual([]);
  fail(x);
  fail(y);
  fail(container.querySelector("audio"));
  expect(reloads).toEqual(["p"]);
  // The view read again of the same expiry: its failures are not the addresses'.
  undo?.();
  container.innerHTML = html;
  undo = enhancement(container, context());
  fail(container.querySelector("img.nw-asset"));
  expect(reloads).toEqual(["p"]);
  // Another page's, and one of a later expiry, once each.
  undo?.();
  container.innerHTML = html;
  undo = enhancement(container, context("q"));
  fail(container.querySelector("img.nw-asset"));
  undo?.();
  container.innerHTML = html;
  undo = enhancement(container, context("p", "2026-10-09T11:00:00Z"));
  fail(container.querySelector("img.nw-asset"));
  fail(container.querySelector("img.nw-asset"));
  expect(reloads).toEqual(["p", "q", "p"]);
  undo?.();
  fail(x);
  expect(reloads).toEqual(["p", "q", "p"]);
});

test("one that fails before the addresses expire, or in a view without them, reads nothing", () => {
  const html = `<p><img class="nw-asset" src="${address("a1")}" alt="x"> ${video("a2")}</p>`;
  const { container, context, reloads, enhancement } = setUp(html, { expires: later });
  enhancement(container, context());
  fail(container.querySelector("img"));
  fail(container.querySelector("video"));
  enhancement(container, context("q", null));
  fail(container.querySelector("img"));
  expect(reloads).toEqual([]);
});

test("an audio or a video started is kept as the HTML is replaced, in place of the same of its attachment's", () => {
  const html =
    `<p>${audio("a1", "first")} ${audio("a1", "second")} ${video("a2")} ${audio("a3")} ${audio("a4")}</p>`.replace(
      'aria-label="second"',
      'aria-label="second" loop=""'
    );
  const { container, context, enhancement } = setUp(html, { expires: later });
  let undo = enhancement(container, context());
  const [first, second, , third, fourth] = container.querySelectorAll<HTMLMediaElement>("audio, video");
  play(second, 12);
  const clip = play(container.querySelector("video"), 40, true);
  play(third, 0, true);
  // Ended, it is not kept: what replaces it starts again.
  play(fourth, 9);
  Object.defineProperty(fourth, "ended", { value: true });
  undo?.();
  // Signed anew, the second of a1 captioned anew; a2 now first of two, a3 there, a4 a link.
  const next =
    `<p>${audio("a1", "first").replace("e=1", "e=2")} ${audio("a1", "again").replace("e=1", "e=2")} ` +
    `${video("a2", "a2", "200").replace("e=1", "e=2")} ${video("a2")} ${audio("a3")} ${audio("a4")}</p>`;
  container.innerHTML = next;
  undo = enhancement(container, context());
  const shown = [...container.querySelectorAll<HTMLMediaElement>("audio, video")];
  expect(shown[0]).not.toBe(first);
  expect(shown[0]?.getAttribute("src")).toContain("e=2");
  expect(shown[1]).toBe(second);
  expect(second?.currentTime).toBe(12);
  // Its attributes the new HTML's, its address its own.
  expect(second?.getAttribute("aria-label")).toBe("again");
  expect(second?.hasAttribute("loop")).toBe(false);
  expect(second?.getAttribute("src")).toContain("e=1");
  expect(shown[2]).toBe(clip);
  expect(clip.getAttribute("width")).toBe("200");
  expect(shown[3]).not.toBe(clip);
  expect(shown[4]).not.toBe(third);
  expect(shown[5]).not.toBe(fourth);
  expect(shown[5]?.getAttribute("src")).toContain("a4");
  undo?.();
});

test("another page's view keeps none; at most twenty are kept", () => {
  const ids = Array.from({ length: 21 }, (_, at) => `m${at.toString()}`);
  const html = `<p>${ids.map((id) => audio(id)).join(" ")}</p>`;
  const { container, context, enhancement } = setUp(html, { expires: later });
  let undo = enhancement(container, context());
  const started = new Set([...container.querySelectorAll("audio")].map((element) => play(element, 1)));
  undo?.();
  container.innerHTML = html;
  undo = enhancement(container, context("q"));
  expect([...container.querySelectorAll("audio")].filter((element) => started.has(element))).toEqual([]);
  const again = new Set([...container.querySelectorAll("audio")].map((element) => play(element, 1)));
  undo?.();
  container.innerHTML = html;
  undo = enhancement(container, context("q"));
  const kept = [...container.querySelectorAll("audio")].map((element) => again.has(element));
  expect(kept).toEqual([...Array.from({ length: 20 }, () => true), false]);
  undo?.();
});

test("one kept that fails has its address signed anew and goes on where it was, once for the HTML", async () => {
  const html = `<p>${audio("a1")} ${video("gone")}</p>`;
  const { container, context, signed, reloads, enhancement } = setUp(html);
  let undo = enhancement(container, context());
  const sound = play(container.querySelector("audio"), 30);
  const clip = play(container.querySelector("video"), 5, true);
  undo?.();
  container.innerHTML = html;
  undo = enhancement(container, context());
  sound.playbackRate = 1.5;
  fail(sound);
  await settle();
  expect(signed).toEqual(["a1"]);
  // Loaded anew, it goes on where it was, at its rate, playing.
  expect(sound.getAttribute("src")).toBe("/api/v0/assets/a1/content?anew=1");
  expect([sound.currentTime, sound.playbackRate, sound.paused, sound.preload]).toEqual([30, 1.5, false, "none"]);
  expect(sound.play).toHaveBeenCalledTimes(1);
  // Not again for this HTML: its failure is the view's, read again once.
  fail(sound);
  await settle();
  expect(signed).toEqual(["a1"]);
  expect(reloads).toEqual(["p"]);
  // Gone, it stays as it is; paused, it does not play.
  fail(clip);
  await settle();
  expect(signed).toEqual(["a1", "gone"]);
  expect(clip.getAttribute("src")).toContain("/gone/");
  expect(clip.play).not.toHaveBeenCalled();
  undo?.();
});

test("one kept goes on from where it failed, playing; one gone from the page meanwhile is left be", async () => {
  const html = `<p>${audio("a1")} ${audio("a2")} ${audio("a3")}</p>`;
  const { container, context, enhancement } = setUp(html, { expires: later });
  let undo = enhancement(container, context());
  const [first, second] = [...container.querySelectorAll("audio")].map((element) => play(element, 30));
  const paused = play(container.querySelectorAll("audio")[2], 12, true);
  undo?.();
  container.innerHTML = html;
  undo = enhancement(container, context());
  fail(first);
  fail(second);
  fail(paused);
  second?.remove();
  await settle();
  expect(first?.currentTime).toBe(30);
  expect(first?.play).toHaveBeenCalledTimes(1);
  expect(second?.getAttribute("src")).toContain("e=1");
  expect(second?.play).not.toHaveBeenCalled();
  // Paused, it is signed anew where it was, its frame loaded, and stays paused.
  expect(paused.getAttribute("src")).toBe("/api/v0/assets/a3/content?anew=1");
  expect([paused.currentTime, paused.preload, paused.paused]).toEqual([12, "metadata", true]);
  expect(paused.play).not.toHaveBeenCalled();
  // An engine that took no position before the metadata takes it then; one that took it is not moved again.
  paused.currentTime = 0;
  paused.dispatchEvent(new Event("loadedmetadata"));
  expect(paused.currentTime).toBe(12);
  if (first !== undefined) {
    first.currentTime = 30.2;
    first.dispatchEvent(new Event("loadedmetadata"));
  }
  expect(first?.currentTime).toBe(30.2);
  undo?.();
});

test("an engine that takes no position before the metadata, and throws, has it as they come; one playing plays", async () => {
  const html = `<p>${audio("a1")}</p>`;
  const { container, context, enhancement } = setUp(html, { expires: later });
  let undo = enhancement(container, context());
  const sound = play(container.querySelector("audio"), 30);
  let time = 30;
  let loaded = true;
  Object.defineProperty(sound, "currentTime", {
    get: () => time,
    set: (value: number) => {
      if (!loaded) {
        throw new DOMException("no metadata", "InvalidStateError");
      }
      time = value;
    },
    configurable: true,
  });
  const setAttribute = sound.setAttribute.bind(sound);
  sound.setAttribute = (name: string, value: string) => {
    if (name === "src") {
      [time, loaded] = [0, false];
    }
    setAttribute(name, value);
  };
  sound.addEventListener("loadedmetadata", () => {
    loaded = true;
  });
  undo?.();
  container.innerHTML = html;
  undo = enhancement(container, context());
  fail(sound);
  await settle();
  expect(sound.getAttribute("src")).toBe("/api/v0/assets/a1/content?anew=1");
  expect(sound.play).toHaveBeenCalledTimes(1);
  expect(time).toBe(0);
  sound.dispatchEvent(new Event("loadedmetadata"));
  expect(time).toBe(30);
  undo?.();
});

test("one kept that had the focus has it back, a folded callout it is in opening", () => {
  const html = `<p>${focusable("a1")}</p><details><summary>More</summary><p>${focusable("a2")}</p></details>`;
  const { container, context, enhancement } = setUp(html, { expires: later });
  let undo = enhancement(container, context());
  const [first, second] = [...container.querySelectorAll("audio")].map((element) => play(element, 20));
  container.querySelector("details")?.setAttribute("open", "");
  second?.focus();
  expect(document.activeElement).toBe(second);
  undo?.();
  container.innerHTML = html;
  undo = enhancement(container, context());
  expect([...container.querySelectorAll("audio")]).toEqual([first, second]);
  expect(document.activeElement).toBe(second);
  expect(container.querySelector("details")?.open).toBe(true);
  undo?.();
});

test("one that failed is not kept: the new one goes on where it was, at its rate, playing if it was, the focus given it", async () => {
  const played = vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue();
  onTestFinished(() => played.mockRestore());
  const html = `<p>${focusable("a1")}</p><details><summary>More</summary><p>${focusable("a2")}</p></details>`;
  const { container, context, enhancement } = setUp(html, { expires: later });
  let undo = enhancement(container, context());
  const [sounding, still] = container.querySelectorAll("audio");
  play(sounding, 20).playbackRate = 1.5;
  play(still, 7, true);
  container.querySelector("details")?.setAttribute("open", "");
  still?.focus();
  broken(sounding);
  broken(still);
  undo?.();
  container.innerHTML = html;
  undo = enhancement(container, context());
  const [first, second] = container.querySelectorAll("audio");
  expect([first === sounding, second === still]).toEqual([false, false]);
  expect([first?.currentTime, first?.playbackRate, first?.preload]).toEqual([20, 1.5, "none"]);
  expect([second?.currentTime, second?.preload]).toEqual([7, "metadata"]);
  expect(document.activeElement).toBe(second);
  expect(container.querySelector("details")?.open).toBe(true);
  await settle();
  expect(played.mock.contexts).toEqual([first]);
  undo?.();
});

test("one that failed, its new one's position refused before the metadata, has it as they come", () => {
  const refused = vi.spyOn(HTMLMediaElement.prototype, "currentTime", "set").mockImplementation(() => {
    throw new DOMException("no metadata", "InvalidStateError");
  });
  onTestFinished(() => refused.mockRestore());
  const html = `<p>${audio("a1")} <a class="nw-asset" href="${address("a2")}" data-nw-size="8">doc.pdf</a></p>`;
  const { container, context, enhancement } = setUp(html, { expires: later });
  let undo = enhancement(container, context());
  broken(play(container.querySelector("audio"), 20, true));
  undo?.();
  container.innerHTML = html;
  undo = enhancement(container, context());
  // The enhancement goes on: the link's hint and size are there.
  expect(container.textContent).toBe(" doc.pdf (opens in a new tab) (8 B)");
  refused.mockRestore();
  const fresh = container.querySelector("audio");
  fresh?.dispatchEvent(new Event("loadedmetadata"));
  expect(fresh?.currentTime).toBe(20);
  undo?.();
});
