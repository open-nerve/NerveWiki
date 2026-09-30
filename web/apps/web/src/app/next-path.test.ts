import { describe, expect, test } from "vitest";

import { keepNext, safeNextPath, withNext } from "./next-path";

// The cases of Nerve's isValidNextPath, with this app's parameter (M1/P5
// design 3.5).
describe("safeNextPath", () => {
  test.each([
    ["a path", "/dashboard", "/dashboard"],
    ["several segments", "/workspace/123", "/workspace/123"],
    ["a query and a fragment", "/settings/profile?tab=x#y", "/settings/profile?tab=x#y"],
    ["spaces around it, trimmed", "  /dashboard  ", "/dashboard"],
    ["the characters around the control ranges", "/a b~/\u0080é", "/a b~/\u0080é"],
  ])("takes %s", (_name, next, want) => {
    expect(safeNextPath(next)).toBe(want);
  });

  test.each([
    ["none", null],
    ["an empty value", ""],
    ["a tab before the path", "\t/dashboard"],
    ["a newline before the path", "\n/dashboard"],
    ["a tab between the slashes", "/\t/evil.example"],
    ["a NUL", "/dash\u0000board"],
    ["a U+001F", "/dashboard\u001f"],
    ["a DEL", "/dash\u007fboard"],
    ["an absolute address", "https://evil.example"],
    ["a protocol-relative address", "//evil.example"],
    ["a slash and a backslash", "/\\evil.example"],
    ["a backslash first", "\\evil.example"],
    ["a javascript: address", "javascript:alert(1)"],
    ["a path without its slash", "dashboard"],
  ])("refuses %s", (_name, next) => {
    expect(safeNextPath(next)).toBeUndefined();
  });
});

test("withNext comes back to the path, its query and its fragment, as one value", () => {
  expect(withNext("/sign-in", "/settings/profile?tab=x#y")).toBe("/sign-in?next=%2Fsettings%2Fprofile%3Ftab%3Dx%23y");
  expect(withNext("/sign-in", "/")).toBe("/sign-in");
});

test("keepNext passes next on as it came, and nothing else", () => {
  expect(keepNext("/sign-up", new URLSearchParams("next=%2Facme%3Fview%3Dlist&x=1"))).toBe(
    "/sign-up?next=%2Facme%3Fview%3Dlist"
  );
  expect(keepNext("/sign-up", new URLSearchParams("x=1"))).toBe("/sign-up");
});
