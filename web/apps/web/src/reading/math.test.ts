import katex, { type KatexOptions } from "katex";
import { afterEach, expect, test, vi } from "vitest";

import { translator } from "../i18n/i18n";
import type { ReadingContext } from "./enhancement";
import { formulaLimit, formulasAtOnce, math, type Typesetter } from "./math";

// The formulas, typeset by KaTeX (M6/P6 design 10).

afterEach(() => {
  vi.useRealTimers();
  document.body.replaceChildren();
});

const context: ReadingContext = {
  workspace: "lab",
  notebook: "n",
  page: "p",
  revision: 1,
  role: "reader",
  t: translator("en"),
  theme: "light",
  reload: () => undefined,
  navigate: () => undefined,
  report: () => undefined,
  unresolved: () => undefined,
};

/** typesetter is a KaTeX that writes [tex] (displayed: [[tex]]) and records each call; it throws for TeX with "bad". */
function typesetter() {
  const calls: { tex: string; options: KatexOptions }[] = [];
  const typeset: Typesetter = {
    render: (tex, element, options) => {
      calls.push({ tex, options });
      if (tex.includes("bad")) {
        throw new Error("KaTeX parse error");
      }
      element.textContent = options.displayMode === true ? `[[${tex}]]` : `[${tex}]`;
    },
  };
  return { calls, typeset };
}

/** view is an article of html, in the page. */
function view(html: string) {
  const article = document.createElement("article");
  article.innerHTML = html;
  document.body.append(article);
  return article;
}

/** settled lets the loading and the tasks of typesetting finish. */
const settled = () => new Promise((resolve) => setTimeout(resolve, 0));

test("each formula is typeset, a block's displayed, with the options that trust nothing in the TeX", async () => {
  const { calls, typeset } = typesetter();
  const article = view(
    '<p>a <span class="nw-math">x^2</span> and <span class="nw-math nw-math-block">\\sum</span></p>' +
      '<div class="nw-scroll"><div class="nw-math nw-math-block">y</div></div>'
  );

  math(async () => typeset)(article, context);
  await settled();

  expect([...article.querySelectorAll(".nw-math")].map((each) => each.textContent)).toEqual([
    "[x^2]",
    "[[\\sum]]",
    "[[y]]",
  ]);
  expect(calls[0]?.options).toEqual({
    trust: false,
    maxSize: 50,
    maxExpand: 1000,
    throwOnError: true,
    strict: false,
    displayMode: false,
  });
});

test("a formula KaTeX cannot read, or longer than the limit, shows its TeX", async () => {
  const { calls, typeset } = typesetter();
  // Each 中 is three bytes: the limit is of bytes, and one of the limit's fits.
  const long = "中".repeat(Math.floor(formulaLimit / 3) + 1);
  const fits = "a".repeat(formulaLimit);
  const article = view(
    `<span class="nw-math">bad</span><span class="nw-math">${long}</span><span class="nw-math">${fits}</span>`
  );

  math(async () => typeset)(article, context);
  await settled();

  expect([...article.querySelectorAll(".nw-math")].map((each) => each.textContent)).toEqual(["bad", long, `[${fits}]`]);
  expect(calls.map((call) => call.tex)).toEqual(["bad", fits]);
});

test("KaTeX itself makes no link, nor loads anything, of the TeX, and throws on what it cannot read", async () => {
  const article = view(
    '<span class="nw-math">\\href{javascript:alert(1)}{x}</span><span class="nw-math">\\url{https://x.example}</span>' +
      '<span class="nw-math">\\includegraphics{https://x.example/a.png}</span><span class="nw-math">\\frac{1}</span>' +
      '<span class="nw-math">\\frac{1}{2}</span>'
  );

  math(async () => katex)(article, context);
  await settled();

  expect(article.querySelectorAll("a, img")).toHaveLength(0);
  const [, , , unread, half] = article.querySelectorAll(".nw-math");
  expect(unread?.textContent).toBe("\\frac{1}");
  expect(half?.querySelector(".katex")).not.toBeNull();
});

test("the formulas are typeset some at a time, the page's thread given back between", async () => {
  vi.useFakeTimers();
  const { calls, typeset } = typesetter();
  const article = view(
    Array.from({ length: formulasAtOnce * 2 + 1 }, (_, i) => `<span class="nw-math">${i}</span>`).join("")
  );

  math(async () => typeset)(article, context);
  await vi.advanceTimersByTimeAsync(0);
  // The first task, at least, is its own.
  expect(calls.length).toBeGreaterThanOrEqual(formulasAtOnce);
  expect(calls.length).toBeLessThan(formulasAtOnce * 2 + 1);
  await vi.runAllTimersAsync();
  expect(calls).toHaveLength(formulasAtOnce * 2 + 1);
});

test("undone, the typeset formulas show their TeX again, and those not reached stay", async () => {
  vi.useFakeTimers();
  const { calls, typeset } = typesetter();
  const count = formulasAtOnce * 3;
  const article = view(Array.from({ length: count }, (_, i) => `<span class="nw-math">${i}</span>`).join(""));

  const undo = math(async () => typeset)(article, context);
  await vi.advanceTimersByTimeAsync(0);
  const reached = calls.length;
  expect(reached).toBeGreaterThanOrEqual(formulasAtOnce);
  expect(reached).toBeLessThan(count);
  undo?.();
  await vi.runAllTimersAsync();

  expect(calls).toHaveLength(reached);
  expect(article.textContent).toBe(Array.from({ length: count }, (_, i) => i).join(""));
});

test("undone before KaTeX has loaded, nothing is typeset", async () => {
  const { calls, typeset } = typesetter();
  let loaded!: (katex: Typesetter) => void;
  const article = view('<span class="nw-math">x</span>');

  const undo = math(() => new Promise((resolve) => (loaded = resolve)))(article, context);
  undo?.();
  loaded(typeset);
  await settled();

  expect(calls).toEqual([]);
  expect(article.textContent).toBe("x");
});

test("KaTeX that cannot be loaded leaves the TeX, and says so on the console", async () => {
  const error = vi.spyOn(console, "error").mockImplementation(() => undefined);
  const article = view('<span class="nw-math">x</span>');

  math(() => Promise.reject(new Error("offline")))(article, context);
  await settled();

  expect(article.textContent).toBe("x");
  expect(error).toHaveBeenCalledTimes(1);
});

test("a view without formulas loads nothing", () => {
  const load = vi.fn(async () => typesetter().typeset);
  expect(math(load)(view("<p>no formula</p>"), context)).toBeUndefined();
  expect(load).not.toHaveBeenCalled();
});
