import { describe, expect, test } from "vitest";

import { sourceTitleProblem } from "./source-title";

describe("sourceTitleProblem", () => {
  test.each([
    ["", "required"],
    [" \t\n ", "required"],
    ["a".repeat(256), "too_long"],
    ["界".repeat(86), "too_long"],
    [".hidden", "invalid_format"],
    ["tail.", "invalid_format"],
    ["path/name", "invalid_format"],
    ["a\\b", "invalid_format"],
    ["why?", "invalid_format"],
    ["a:b*c", "invalid_format"],
    ['a"b<c>d|e#f^g[h]', "invalid_format"],
    ["line\nbreak", "invalid_format"],
    ["line\u2028break", "invalid_format"],
    ["para\u2029break", "invalid_format"],
    ["bi\u202Edirection", "invalid_format"],
    ["bi\u2066direction", "invalid_format"],
    ["CON", "not_allowed"],
    ["con.txt", "not_allowed"],
    ["PRN", "not_allowed"],
    ["aux", "not_allowed"],
    ["NUL", "not_allowed"],
    ["COM1.tar.gz", "not_allowed"],
    ["lpt9", "not_allowed"],
    ["LPT¹.txt", "not_allowed"],
    ["COM²", "not_allowed"],
    ["COM³", "not_allowed"],
  ])("%j is refused as %s", (title, problem) => {
    expect(sourceTitleProblem(title)).toBe(problem);
  });

  test.each([
    "  Valid title  ",
    "a".repeat(255),
    "界".repeat(85),
    ` ${"e\u0301".repeat(127)}a `,
    "COM10",
    "LPT0",
    "ＣＯＮ",
    "console.txt",
    "part.one",
    "👨\u200D👩",
    "AGENTS",
  ])("%j is a valid explicit source title after trimming and NFC", (title) => {
    expect(sourceTitleProblem(title)).toBeUndefined();
  });
});
