import { describe, expect, test } from "vitest";

import { extensionOf, fixedName, freeName, isPageName } from "./upload-name";

const bytes = (s: string) => new TextEncoder().encode(s).length;

describe("fixedName", () => {
  test.each([
    ["a.png", "a.png"],
    ["  a.png  ", "a.png"],
    ["..a.png..", "a.png"],
    [" . a.png . ", "a.png"],
    ['a/b\\c:d*e?f"g<h>i|j#k^l[m]n.png', "a_b_c_d_e_f_g_h_i_j_k_l_m_n.png"],
    ["tab\there.png", "tab_here.png"],
    ["line\u2028sep\u2029.png", "line_sep_.png"],
    ["evil\u202Egnp.exe", "evil_gnp.exe"],
    ["\u061C\u200E\u200F\u2066\u2069x.png", "_____x.png"],
    // NFC: an e and a combining acute make one character.
    ["cafe\u0301.png", "caf\u00E9.png"],
    // Joiners stay: they make up emoji.
    ["\u{1F468}\u200D\u{1F469}.png", "\u{1F468}\u200D\u{1F469}.png"],
    ["CON", "CON_"],
    ["con.txt", "con_.txt"],
    ["Com1.tar.gz", "Com1_.tar.gz"],
    ["LPT\u00B9.txt", "LPT\u00B9_.txt"],
    ["console.txt", "console.txt"],
    ["", "Untitled"],
    ["   ", "Untitled"],
    ["...", "Untitled"],
  ])("%j is %j", (raw, fixed) => {
    expect(fixedName(raw, "Untitled")).toBe(fixed);
  });

  test("a name over 255 bytes is cut at a character, its extension kept", () => {
    const fixed = fixedName(`${"界".repeat(100)}.png`, "Untitled");

    expect(bytes(fixed)).toBeLessThanOrEqual(255);
    expect(fixed).toBe(`${"界".repeat(83)}.png`);
  });

  test("a cut that would end with blanks or dots ends before them", () => {
    const fixed = fixedName(`${"a".repeat(248)} . x.png`, "Untitled");

    expect(fixed).toBe(`${"a".repeat(248)}.png`);
  });

  test("an extension longer than a name is cut as well, the stem's first character kept", () => {
    const fixed = fixedName(`a.${"b".repeat(300)}`, "Untitled");
    expect(bytes(fixed)).toBeLessThanOrEqual(255);
    expect(fixed.startsWith("a.")).toBe(true);

    for (const stem of ["\u4E2D", "\u{1F600}"]) {
      const wide = fixedName(`${stem}.${"b".repeat(300)}`, "Untitled");
      expect(bytes(wide)).toBeLessThanOrEqual(255);
      expect(wide.startsWith(`${stem}.`)).toBe(true);
      const numbered = freeName(wide, [wide]);
      expect(bytes(numbered)).toBeLessThanOrEqual(255);
      expect(numbered.startsWith(`${stem} 2.`)).toBe(true);
    }
  });

  test("a long run of blanks and dots takes no time to the square of its length", () => {
    const started = performance.now();
    fixedName(`a${" .".repeat(100_000)}b`, "Untitled");
    fixedName(`${" .".repeat(100_000)}`, "Untitled");

    expect(performance.now() - started).toBeLessThan(1000);
  });
});

describe("freeName", () => {
  test.each([
    ["a.png", [], "a.png"],
    ["a.png", ["b.png"], "a.png"],
    ["a.png", ["a.png"], "a 2.png"],
    ["a.png", ["A.PNG", "a 2.png"], "a 3.png"],
    ["caf\u00E9.png", ["cafe\u0301.png"], "caf\u00E9 2.png"],
    ["STRASSE.png", ["Stra\u00DFe.png"], "STRASSE 2.png"],
    ["README", ["readme"], "README 2"],
    [".env", [".env"], ".env 2"],
    ["a.tar.gz", ["a.tar.gz"], "a.tar 2.gz"],
  ])("%j beside %j is %j", (name, taken, free) => {
    expect(freeName(name, taken)).toBe(free);
  });

  test("a numbered name of a long one stays within 255 bytes", () => {
    const long = `${"a".repeat(251)}.png`;

    const free = freeName(long, [long]);

    expect(bytes(free)).toBeLessThanOrEqual(255);
    expect(free).toBe(`${"a".repeat(249)} 2.png`);
  });
});

describe("the names' helpers", () => {
  test("isPageName: a name ending with .md, in any case, is a page's file", () => {
    expect(["a.md", "A.MD", "b.Md"].map(isPageName)).toEqual([true, true, true]);
    expect(["a.mdx", "md", "a.md.png"].map(isPageName)).toEqual([false, false, false]);
  });

  test("extensionOf is from the last dot, none for a dot that starts the name", () => {
    expect(["a.png", "a.tar.gz", ".env", "README", "a."].map(extensionOf)).toEqual([".png", ".gz", "", "", "."]);
  });
});
