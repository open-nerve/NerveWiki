import { act, fireEvent, render } from "@testing-library/react";
import { createElement, useLayoutEffect } from "react";
import { afterEach, beforeEach, expect, test, vi } from "vitest";

import { useCurrentHeading } from "./current-heading";

// The section read of the page's outline (M13/P4/S1, decisions 861, 887, 893): the last of the headings listed whose
// top is above a third of the window, the window's bottom once a page that scrolls is at its end; a heading clicked
// marked until the reader moves; measured at most once a frame, only while active. jsdom lays nothing out: the
// headings' boxes, the window's height, its scroll and the page's height are stubbed (design 7.1).

/** Where each heading's top is, by its id: what its stubbed box answers. */
let tops: Record<string, number> = {};
/** The page's height: what document.documentElement.scrollHeight answers (jsdom's is 0). */
let height = 3000;
/** The ids of the headings measured, a call of getBoundingClientRect each. */
let measured: string[] = [];
/** What a test put in the page, taken out after it. */
let added: Element[] = [];

const ids = ["a", "b", "c", "d"] as const;

beforeEach(() => {
  tops = {};
  height = 3000;
  measured = [];
  added = [];
  vi.stubGlobal("innerHeight", 900);
  Object.defineProperty(document.documentElement, "scrollHeight", { configurable: true, get: () => height });
  vi.spyOn(window, "scrollY", "get").mockReturnValue(0);
});

afterEach(() => {
  Reflect.deleteProperty(document.documentElement, "scrollHeight");
  for (const element of added) {
    element.remove();
  }
});

/** put adds element to the page's body, taken out after the test. */
function put<T extends Element>(element: T): T {
  document.body.append(element);
  added.push(element);
  return element;
}

/** headings puts an h2 for each id of placed in the page, its box's top as placed has it; closed, in a closed details. */
function headings(placed: Record<string, number>, closed?: string): void {
  tops = { ...placed };
  for (const id of Object.keys(placed)) {
    const heading = document.createElement("h2");
    heading.id = id;
    vi.spyOn(heading, "getBoundingClientRect").mockImplementation(() => {
      measured.push(id);
      return new DOMRect(0, tops[id] ?? 0, 600, 28);
    });
    if (id === closed) {
      const details = document.createElement("details");
      details.append(document.createElement("summary"), heading);
      put(details);
    } else {
      put(heading);
    }
  }
}

/** A link of the outline, which pin is given. */
function link(): HTMLAnchorElement {
  const anchor = document.createElement("a");
  anchor.href = "#c";
  anchor.textContent = "C";
  return put(anchor);
}

/** frame settles once the window has drawn its next frame. */
function frame(): Promise<void> {
  return new Promise((resolve) => {
    requestAnimationFrame(() => resolve());
  });
}

/** scrolled has the window scroll, and waits for a frame: the hook's measure is in it. */
async function scrolled(): Promise<void> {
  await act(async () => {
    fireEvent.scroll(window);
    await frame();
  });
}

/** probe renders the hook over ids, active or not, keeping what each render answered. */
function probe(active = true) {
  const renders: (string | undefined)[] = [];
  const held: { pin?: (id: string, link: Element) => void } = {};
  function Probe({ on }: { on: boolean }) {
    const heading = useCurrentHeading(ids, on);
    renders.push(heading.current);
    useLayoutEffect(() => {
      held.pin = heading.pin;
    });
    return null;
  }
  const view = render(createElement(Probe, { on: active }));
  return {
    renders,
    current: () => renders.at(-1),
    pin: (id: string, to: Element) => act(() => held.pin?.(id, to)),
    rerender: (on: boolean) => view.rerender(createElement(Probe, { on })),
    unmount: view.unmount,
  };
}

test("the current heading is the last of those above a third of the window, from the commit it mounts in; with all below it, none", async () => {
  headings({ a: -400, b: 100, c: 500, d: 700 });
  const read = probe();
  // No frame waited for (section 1 (k)).
  expect(read.current()).toBe("b");

  tops = { a: 400, b: 500, c: 600, d: 700 };
  await scrolled();
  expect(read.current()).toBeUndefined();
});

test("a heading in a closed details is not read, whatever its box; opened, it is", async () => {
  headings({ a: -400, b: 100, c: 500, d: 700 }, "b");
  const read = probe();
  expect(read.current()).toBe("a");

  const details = document.getElementById("b")?.closest("details");
  if (!details) {
    throw new Error("b is in no details");
  }
  details.open = true;
  await scrolled();
  expect(read.current()).toBe("b");
});

test("a page that scrolls, at its end: the line is the window's bottom", () => {
  vi.spyOn(window, "scrollY", "get").mockReturnValue(2100);
  headings({ a: -400, b: 100, c: 500, d: 700 });
  expect(probe().current()).toBe("d");
});

test("a page no taller than the window is never at its end: the line stays at a third", () => {
  height = 900;
  headings({ a: -400, b: 100, c: 500, d: 700 });
  expect(probe().current()).toBe("b");
});

test("a heading pinned in view is current until the reader moves; a press on its link is no move", async () => {
  headings({ a: -400, b: 100, c: 500, d: 700 });
  const read = probe();
  const to = link();

  read.pin("c", to);
  expect(read.current()).toBe("c");
  fireEvent.pointerDown(to, { button: 0 });
  await scrolled();
  expect(read.current()).toBe("c");

  fireEvent.wheel(window);
  await scrolled();
  expect(read.current()).toBe("b");
});

test("a heading pinned out of view stays current until the view has scrolled it in, and while it is in, until the reader moves", async () => {
  headings({ a: -400, b: 100, c: 2000, d: 700 });
  const read = probe();
  const to = link();

  read.pin("c", to);
  expect(read.current()).toBe("c");
  // The router's commit may come a frame late (review A's R1-M1).
  await scrolled();
  expect(read.current()).toBe("c");
  tops = { ...tops, c: 16 };
  await scrolled();
  expect(read.current()).toBe("c");
  // By the line it would be d now.
  tops = { ...tops, d: 200 };
  await scrolled();
  expect(read.current()).toBe("c");

  fireEvent.wheel(window);
  await scrolled();
  expect(read.current()).toBe("d");
});

test("folded away and shown again, the render that shows it answers none before it measures, not the one read before", () => {
  headings({ a: -400, b: 100, c: 500, d: 700 });
  const read = probe();
  expect(read.current()).toBe("b");

  act(() => read.rerender(false));
  expect(read.current()).toBeUndefined();
  const before = read.renders.length;
  act(() => read.rerender(true));
  expect(read.renders.slice(before)).toEqual([undefined, "b"]);
});

test("a heading pinned while not active marks nothing: active, it reads the line", () => {
  headings({ a: -400, b: 100, c: 500, d: 700 });
  const read = probe(false);

  read.pin("c", link());
  expect(read.current()).toBeUndefined();
  act(() => read.rerender(true));
  expect(read.current()).toBe("b");
});

test("a heading pinned and not yet seen is let go as the outline is folded away: shown again, it reads the line", () => {
  headings({ a: -400, b: 100, c: 2000, d: 700 });
  const read = probe();

  read.pin("c", link());
  expect(read.current()).toBe("c");
  act(() => read.rerender(false));
  act(() => read.rerender(true));
  expect(read.current()).toBe("b");
});

test("a frame that reads the same heading renders nothing", async () => {
  headings({ a: -400, b: 100, c: 500, d: 700 });
  const read = probe();
  const renders = read.renders.length;

  await scrolled();
  await scrolled();
  expect(read.renders.length).toBe(renders);
  expect(read.current()).toBe("b");
});

test("two scrolls in a frame measure once", () => {
  const queued: FrameRequestCallback[] = [];
  vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => queued.push(callback));
  vi.stubGlobal("cancelAnimationFrame", () => {});
  headings({ a: -400, b: 100, c: 500, d: 700 });
  probe();
  measured = [];

  fireEvent.scroll(window);
  fireEvent.scroll(window);
  expect(queued).toHaveLength(1);
  act(() => {
    for (const callback of queued.splice(0)) {
      callback(0);
    }
  });
  // From the last listed back to the first above the line.
  expect(measured).toEqual(["d", "c", "b"]);
});

test("not active, it measures nothing and answers none", async () => {
  headings({ a: -400, b: 100, c: 500, d: 700 });
  const read = probe(false);

  await scrolled();
  expect([measured, read.current()]).toEqual([[], undefined]);
});

test("without a ResizeObserver it reads all the same; with one, it watches the main and measures again as it resizes", async () => {
  expect(typeof ResizeObserver).toBe("undefined");
  headings({ a: -400, b: 100, c: 500, d: 700 });
  const without = probe();
  expect(without.current()).toBe("b");
  without.unmount();

  const observers: { resized: ResizeObserverCallback; observed: Element[]; disconnected: boolean }[] = [];
  vi.stubGlobal(
    "ResizeObserver",
    class {
      readonly watched: { resized: ResizeObserverCallback; observed: Element[]; disconnected: boolean };
      constructor(resized: ResizeObserverCallback) {
        this.watched = { resized, observed: [], disconnected: false };
        observers.push(this.watched);
      }
      observe(element: Element) {
        this.watched.observed.push(element);
      }
      disconnect() {
        this.watched.disconnected = true;
      }
    }
  );
  const main = put(document.createElement("main"));
  const read = probe();
  expect(observers.map(({ observed }) => observed)).toEqual([[main]]);

  tops = { ...tops, c: 200 };
  await act(async () => {
    observers[0]?.resized([], {} as ResizeObserver);
    await frame();
  });
  expect(read.current()).toBe("c");

  read.unmount();
  expect(observers[0]?.disconnected).toBe(true);
});

test("unmounted, it listens no more", async () => {
  headings({ a: -400, b: 100, c: 500, d: 700 });
  const read = probe();
  read.unmount();
  measured = [];

  await act(async () => {
    fireEvent.scroll(window);
    fireEvent(window, new Event("resize"));
    await frame();
  });
  expect(measured).toEqual([]);
});
