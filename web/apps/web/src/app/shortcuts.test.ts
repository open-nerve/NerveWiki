import { expect, test } from "vitest";

import { isMod, isModShift, onMac } from "./shortcuts";

const press = (
  key: string,
  modifiers: Partial<Record<"metaKey" | "ctrlKey" | "altKey" | "shiftKey", boolean>> = {},
  code = `Key${key.toUpperCase()}`
) => ({
  key,
  code,
  metaKey: false,
  ctrlKey: false,
  altKey: false,
  shiftKey: false,
  ...modifiers,
});

test("Mod is Cmd on macOS and Ctrl elsewhere, alone", () => {
  expect(isMod(press("o", { metaKey: true }), "o", true)).toBe(true);
  expect(isMod(press("O", { metaKey: true }), "o", true)).toBe(true);
  expect(isMod(press("o", { ctrlKey: true }), "o", true)).toBe(false);
  expect(isMod(press("o", { ctrlKey: true }), "o", false)).toBe(true);
  expect(isMod(press("o", { metaKey: true }), "o", false)).toBe(false);
  expect(isMod(press("o", { ctrlKey: true, metaKey: true }), "o", false)).toBe(false);
  expect(isMod(press("o", { ctrlKey: true, shiftKey: true }), "o", false)).toBe(false);
  expect(isMod(press("o", { ctrlKey: true, altKey: true }), "o", false)).toBe(false);
  expect(isMod(press("p", { ctrlKey: true }), "o", false)).toBe(false);
  expect(isMod(press("o"), "o", false)).toBe(false);
});

test("macOS is told by the platform", () => {
  expect(onMac("MacIntel")).toBe(true);
  expect(onMac("Win32")).toBe(false);
  expect(onMac("Linux x86_64")).toBe(false);
});

test("the letter is the key's, or on a layout of other letters, the Latin one at its place", () => {
  // Dvorak: O where QWERTY has S.
  expect(isMod(press("o", { ctrlKey: true }, "KeyS"), "o", false)).toBe(true);
  // Russian: щ where QWERTY has O.
  expect(isMod(press("щ", { ctrlKey: true }, "KeyO"), "o", false)).toBe(true);
  expect(isMod(press("щ", { ctrlKey: true }, "KeyP"), "o", false)).toBe(false);
  expect(isMod(press("s", { ctrlKey: true }, "KeyO"), "o", false)).toBe(false);
});

// Mod+Shift+F focuses the search box (M8/P5 design 4.2); isMod still refuses Shift.
test("Mod+Shift is Cmd on macOS and Ctrl elsewhere, with Shift and without Alt", () => {
  expect(isModShift(press("F", { metaKey: true, shiftKey: true }), "f", true)).toBe(true);
  expect(isModShift(press("f", { metaKey: true, shiftKey: true }), "f", true)).toBe(true);
  expect(isModShift(press("F", { ctrlKey: true, shiftKey: true }), "f", true)).toBe(false);
  expect(isModShift(press("F", { ctrlKey: true, shiftKey: true }), "f", false)).toBe(true);
  expect(isModShift(press("F", { metaKey: true, shiftKey: true }), "f", false)).toBe(false);
  expect(isModShift(press("F", { ctrlKey: true, metaKey: true, shiftKey: true }), "f", false)).toBe(false);
  expect(isModShift(press("f", { ctrlKey: true }), "f", false)).toBe(false);
  expect(isModShift(press("F", { ctrlKey: true, shiftKey: true, altKey: true }), "f", false)).toBe(false);
  expect(isModShift(press("G", { ctrlKey: true, shiftKey: true }), "f", false)).toBe(false);
  expect(isModShift(press("F", { shiftKey: true }), "f", false)).toBe(false);
  expect(isMod(press("F", { ctrlKey: true, shiftKey: true }), "f", false)).toBe(false);
});

test("Mod+Shift's letter is the key's, or on a layout of other letters, the Latin one at its place", () => {
  // Russian: А where QWERTY has F.
  expect(isModShift(press("А", { ctrlKey: true, shiftKey: true }, "KeyF"), "f", false)).toBe(true);
  expect(isModShift(press("А", { ctrlKey: true, shiftKey: true }, "KeyA"), "f", false)).toBe(false);
  // Dvorak: F where QWERTY has Y.
  expect(isModShift(press("F", { ctrlKey: true, shiftKey: true }, "KeyY"), "f", false)).toBe(true);
});
