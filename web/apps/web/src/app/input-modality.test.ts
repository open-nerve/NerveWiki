import { afterEach, expect, test } from "vitest";

import { trackInputModality } from "./input-modality";

// How the reader last gave input, which the menus' rings follow (M13/P1 design 4.2.6, decision 760).

const root = document.documentElement;
let stop: (() => void) | undefined;

afterEach(() => {
  stop?.();
  stop = undefined;
  delete root.dataset.modality;
});

/** key presses a key on the body, as KeyboardEventInit says. */
function key(init: KeyboardEventInit): void {
  document.body.dispatchEvent(new KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init }));
}

/** point moves or presses the pointer on the body. */
function point(type: "pointermove" | "pointerdown"): void {
  document.body.dispatchEvent(new PointerEvent(type, { bubbles: true }));
}

test("nothing typed nor pointed with, there is no modality; a key is the keyboard's, the pointer's move and press the pointer's", () => {
  stop = trackInputModality(root);
  expect(root.dataset.modality).toBeUndefined();
  key({ key: "Tab" });
  expect(root.dataset.modality).toBe("keyboard");
  point("pointermove");
  expect(root.dataset.modality).toBe("pointer");
  key({ key: "ArrowDown" });
  expect(root.dataset.modality).toBe("keyboard");
  point("pointerdown");
  expect(root.dataset.modality).toBe("pointer");
});

test("a modifier alone, or a key with Meta, Ctrl or Alt, is no keyboard's; Shift and Tab is", () => {
  stop = trackInputModality(root);
  point("pointermove");
  // A copy, a screenshot's keys, Shift held to click: the item the pointer rests on gets no ring.
  for (const init of [
    { key: "Shift", shiftKey: true },
    { key: "Meta", metaKey: true },
    { key: "Control", ctrlKey: true },
    { key: "Alt", altKey: true },
    { key: "CapsLock" },
    { key: "c", metaKey: true },
    { key: "c", ctrlKey: true },
    { key: "4", metaKey: true, shiftKey: true },
    { key: "ø", altKey: true },
  ]) {
    key(init);
    expect(root.dataset.modality, JSON.stringify(init)).toBe("pointer");
  }
  key({ key: "Tab", shiftKey: true });
  expect(root.dataset.modality).toBe("keyboard");
});

test("a key a control keeps to itself is the keyboard's too: the document hears it first", () => {
  stop = trackInputModality(root);
  point("pointerdown");
  const button = document.createElement("button");
  button.addEventListener("keydown", (event) => event.stopPropagation());
  document.body.append(button);
  button.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
  button.remove();
  expect(root.dataset.modality).toBe("keyboard");
});

test("it writes the modality only as it changes", () => {
  stop = trackInputModality(root);
  const observer = new MutationObserver(() => {});
  observer.observe(root, { attributes: true, attributeFilter: ["data-modality"] });
  key({ key: "a" });
  key({ key: "b" });
  point("pointermove");
  point("pointermove");
  point("pointerdown");
  key({ key: "Tab" });
  // Each write is a record, the same value written again too: keyboard, pointer, keyboard.
  const written = observer.takeRecords().length;
  observer.disconnect();
  expect(written).toBe(3);
});

test("stopped, it changes the modality no more", () => {
  stop = trackInputModality(root);
  key({ key: "Tab" });
  stop();
  stop = undefined;
  point("pointermove");
  point("pointerdown");
  expect(root.dataset.modality).toBe("keyboard");
  // Nor a key: each listener goes, in the capture it was added in.
  stop = trackInputModality(root);
  point("pointerdown");
  stop();
  stop = undefined;
  key({ key: "Tab" });
  expect(root.dataset.modality).toBe("pointer");
});
