import { fireEvent } from "@testing-library/react";
import { afterEach, expect, test } from "vitest";

import { watchReader } from "./readers-input";

// What a reader does, watched (readers-input.ts): within an area, what they do there alone (M13/P2/S2, decision 791).

afterEach(() => {
  document.body.replaceChildren();
});

/** A column's area that scrolls with a link in it, and a main with a paragraph, in the document. */
function shell(): { link: HTMLElement; paragraph: HTMLElement } {
  const area = document.createElement("div");
  area.setAttribute("data-sidebar-scroll", "");
  const link = document.createElement("a");
  link.href = "#a";
  area.append(link);
  const main = document.createElement("main");
  const paragraph = document.createElement("p");
  main.append(paragraph);
  document.body.append(area, main);
  return { link, paragraph };
}

test.each([
  ["a key", (on: Element) => fireEvent.keyDown(on, { key: "ArrowDown" })],
  ["the main button's press", (on: Element) => fireEvent.pointerDown(on, { button: 0 })],
  ["a wheel", (on: Element) => fireEvent.wheel(on)],
  ["a touch's move", (on: Element) => fireEvent.touchMove(on)],
])("a watch within an area does not count %s outside it, and counts it in it", (_what, act) => {
  const { link, paragraph } = shell();
  const reader = watchReader({ within: "[data-sidebar-scroll]" });
  try {
    act(paragraph);
    act(document.body);
    expect(reader.acted()).toBe(false);
    act(link);
    expect(reader.acted()).toBe(true);
  } finally {
    reader.end();
  }
});

test("a watch without an area counts what is done anywhere", () => {
  const { paragraph } = shell();
  const reader = watchReader();
  try {
    fireEvent.wheel(paragraph);
    expect(reader.acted()).toBe(true);
  } finally {
    reader.end();
  }
});
