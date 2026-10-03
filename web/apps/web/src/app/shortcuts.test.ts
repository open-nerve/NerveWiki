import { expect, test } from "vitest";

import { isMod, onMac } from "./shortcuts";

const press = (
  key: string,
  modifiers: Partial<Record<"metaKey" | "ctrlKey" | "altKey" | "shiftKey", boolean>> = {}
) => ({
  key,
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
