import katex, { renderToString, type KatexOptions } from "katex";
import { afterEach, expect, test, vi } from "vitest";

import { translator } from "../i18n/i18n";
import type { ReadingContext } from "./enhancement";
import { formulaLimit, guardLabels, loadKatex, math, taskTime, type LabelTypesetter, type Typesetter } from "./math";

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
  theme: () => "light",
  onThemeChange: () => () => undefined,
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

test("a formula that defines a macro, or names one of KaTeX's own, shows its TeX: KaTeX is not given it", async () => {
  const { calls, typeset } = typesetter();
  const refused = [
    String.raw`\def\a{x}\a`,
    String.raw`\gdef\a{x}`,
    String.raw`\edef\a{x}`,
    String.raw`\xdef\a{x}`,
    String.raw`\let\a=x`,
    String.raw`\futurelet\a\b`,
    String.raw`\global\a`,
    String.raw`\long\a`,
    String.raw`\newcommand{\a}{x}`,
    String.raw`\renewcommand*{\a}{x}`,
    String.raw`\providecommand\a{x}`,
    String.raw`\tag{1}\df@tag\df@tag`,
    String.raw`a\@b`,
    String.raw`\message{x}`,
    String.raw`\errmessage{x}`,
    String.raw`\show\x`,
  ];
  // Commands whose names begin with one refused are not refused; nor is a row's end before "@" (\\@, a CD arrow).
  const typesetAll = [
    String.raw`\deg x \longrightarrow y`,
    String.raw`a \newline b \leftarrow c`,
    String.raw`\text{me@host} \tag{1}`,
    String.raw`\begin{CD}A @>a>> B \\@VbVV @AAcA \\ C @= D\end{CD}`,
    String.raw`a \\def`,
  ];
  const article = view([...refused, ...typesetAll].map((tex) => `<span class="nw-math">${tex}</span>`).join(""));

  math(async () => typeset)(article, context);
  await settled();

  expect(calls.map((call) => call.tex)).toEqual(typesetAll);
  expect([...article.querySelectorAll(".nw-math")].slice(0, refused.length).map((each) => each.textContent)).toEqual(
    refused
  );
});

test("KaTeX as the app loads it refuses an alignment of more columns than it can make, and one nested past formulaDepth", async () => {
  const columns = String.raw`\begin{alignedat}{1000000}a\end{alignedat}`;
  // Forty levels nest some 270 elements deep: past formulaDepth, though short of where Chromium crashes.
  const deep = `${"x^{".repeat(40)}x${"}".repeat(40)}`;
  const article = view(
    `<span class="nw-math">${columns}</span><span class="nw-math">${deep}</span>` +
      String.raw`<span class="nw-math">\begin{alignedat}{2}a&=b&c&=d\end{alignedat} x^{x^{x}}</span>`
  );

  math(loadKatex)(article, context);
  await vi.waitFor(() => expect(article.querySelectorAll(".katex")).toHaveLength(1), { timeout: 10_000 });

  const [first, second] = article.querySelectorAll(".nw-math");
  expect([first?.textContent, second?.textContent]).toEqual([columns, deep]);
});

test("guarded, KaTeX's renderToString, as mermaid calls it for a label, refuses what math refuses and takes math's options", () => {
  const labels: LabelTypesetter = { renderToString };
  guardLabels(labels);
  guardLabels(labels);

  expect(() => labels.renderToString(String.raw`\d` + String.raw`ef\a{x}\a`, { displayMode: true })).toThrow();
  // As HTML alone: MathML's annotation holds the TeX.
  const html = labels.renderToString(String.raw`\rule{100em}{1em}\href{https://x.example}{x}`, {
    displayMode: true,
    throwOnError: true,
    output: "html",
  });
  expect(html).toContain("katex-display");
  expect(html).toContain("50em");
  expect(html).not.toContain("100em");
  expect(html).not.toContain("<a");
  // Longer than formulaLimit, or nesting past the bound once typeset.
  expect(() => labels.renderToString("x".repeat(formulaLimit + 1), { displayMode: true })).toThrow();
  expect(labels.renderToString("x".repeat(formulaLimit), { displayMode: true, output: "mathml" })).toContain("<math");
  expect(() => labels.renderToString(`${"x^{".repeat(40)}x${"}".repeat(40)}`, { displayMode: true })).toThrow();
  expect(labels.renderToString(`${"x^{".repeat(5)}x${"}".repeat(5)}`, { displayMode: true })).toContain("katex");
});

test("KaTeX as the app loads it typesets a formula, and is not given one that would expand without end", async () => {
  const a = String.raw`\def\a{xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx}`;
  const b = String.raw`\def\b{${String.raw`\a`.repeat(31)}}`;
  const bomb = a + b + String.raw`\b`.repeat(31);
  const article = view(
    `<span class="nw-math">${bomb}</span><span class="nw-math nw-math-block">x = \\frac{1}{2} \\tag{1}</span>`
  );

  math(loadKatex)(article, context);
  await vi.waitFor(() => expect(article.querySelector(".nw-math-block .katex-display")).not.toBeNull());

  expect(article.querySelector(".nw-math")?.textContent).toBe(bomb);
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

/** slow is a typesetter that takes 20 ms of its clock (now) for each formula. */
function slow() {
  const { calls, typeset } = typesetter();
  let time = 0;
  const timed: Typesetter = {
    render: (tex, element, options) => {
      time += 20;
      typeset.render(tex, element, options);
    },
  };
  return { calls, typeset: timed, now: () => time };
}

test("the formulas are typeset for a while at a time, the page's thread given back between", async () => {
  vi.useFakeTimers();
  const { calls, typeset, now } = slow();
  const count = 20;
  const article = view(Array.from({ length: count }, (_, i) => `<span class="nw-math">${i}</span>`).join(""));

  math(async () => typeset, now)(article, context);
  await vi.advanceTimersByTimeAsync(0);
  // The first task, at least, is its own: as many as fit in taskTime.
  expect(calls.length).toBeGreaterThanOrEqual(Math.ceil(taskTime / 20));
  expect(calls.length).toBeLessThan(count);
  await vi.runAllTimersAsync();
  expect(calls).toHaveLength(count);
});

test("each formula is laid out as it is put in, the clock counting the layout, which can take far longer than KaTeX", async () => {
  vi.useFakeTimers();
  const { calls, typeset } = typesetter();
  let time = 0;
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(() => {
    time += 20;
    return new DOMRect();
  });
  const count = 20;
  const article = view(Array.from({ length: count }, (_, i) => `<span class="nw-math">${i}</span>`).join(""));

  math(
    async () => typeset,
    () => time
  )(article, context);
  await vi.advanceTimersByTimeAsync(0);
  expect(calls.length).toBeGreaterThanOrEqual(Math.ceil(taskTime / 20));
  expect(calls.length).toBeLessThan(count);
  await vi.runAllTimersAsync();
  expect(calls).toHaveLength(count);
});

test("undone, the typeset formulas show their TeX again, and those not reached stay", async () => {
  vi.useFakeTimers();
  const { calls, typeset, now } = slow();
  const count = 30;
  const article = view(Array.from({ length: count }, (_, i) => `<span class="nw-math">${i}</span>`).join(""));

  const undo = math(async () => typeset, now)(article, context);
  await vi.advanceTimersByTimeAsync(0);
  const reached = calls.length;
  expect(reached).toBeGreaterThanOrEqual(Math.ceil(taskTime / 20));
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
