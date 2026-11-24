import { afterEach, expect, test, vi } from "vitest";

import { translator } from "../i18n/i18n";
import type { Locale } from "../i18n/locale";
import { codeBlocks } from "./code-blocks";
import type { ReadingContext } from "./enhancement";

// A code block's head (M13/P4/S2, decisions 869, 890): its language, and Copy where the page may use the clipboard,
// which copies the code's text alone.

afterEach(() => {
  vi.useRealTimers();
  Reflect.deleteProperty(navigator, "clipboard");
  document.body.replaceChildren();
});

/** contextIn is a view's context in locale, as the enhancements read it. */
function contextIn(locale: Locale = "en"): ReadingContext {
  return {
    workspace: "lab",
    notebook: "n",
    page: "p",
    revision: 1,
    t: translator(locale),
    locale,
    theme: () => "light",
    onThemeChange: () => () => undefined,
    reload: () => undefined,
    assetsExpire: null,
    assetAddress: () => Promise.reject(new Error("no attachments")),
    navigate: () => undefined,
    report: () => undefined,
    unresolved: () => undefined,
  };
}

/** clipboard gives the page a clipboard whose writeText is written: jsdom has none. */
function clipboard(writeText: (text: string) => Promise<void>) {
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
}

/** view is an article of html in the page. */
function view(html: string): HTMLElement {
  const article = document.createElement("article");
  article.innerHTML = html;
  document.body.append(article);
  return article;
}

const goBlock = '<pre><code class="language-go">func main() {}\n</code></pre>';

/** parts are a code block's head's, of the pre that holds it: its language, what it said and Copy. */
function parts(pre: Element) {
  const head = pre.firstElementChild;
  return {
    head,
    said: head?.querySelector<HTMLElement>(".nw-code-said") ?? null,
    copy: head?.querySelector<HTMLButtonElement>("button.nw-code-copy") ?? null,
  };
}

/** answered lets a press's clipboard answer be heard: its promise settles in a microtask. */
const answered = () => vi.advanceTimersByTimeAsync(0);

test("a code block's head is its pre's first child: its language, and Copy where the page has a clipboard; a diagram's has none", () => {
  clipboard(() => Promise.resolve());
  const article = view(
    `${goBlock}<pre><code>plain</code></pre><pre><code class="language-mermaid">graph TD; a-->b</code></pre>` +
      '<pre><code class="hljs language-rust">fn x()</code></pre>'
  );

  codeBlocks(article, contextIn());

  const [go, plain, mermaid, rust] = [...article.querySelectorAll("pre")] as HTMLElement[];
  const named = parts(go as HTMLElement);
  expect(named.head?.className).toBe("nw-code-head");
  expect(named.head?.textContent).toBe("goCopy");
  expect(named.copy?.type).toBe("button");
  expect(named.copy?.textContent).toBe("Copy");
  // No language, no name: Copy alone.
  expect(parts(plain as HTMLElement).head?.textContent).toBe("Copy");
  expect(mermaid?.querySelector(".nw-code-head")).toBeNull();
  // The language is the class that names it, whatever else the code's classes are.
  expect(parts(rust as HTMLElement).head?.firstChild?.textContent).toBe("rust");
  // The code is as it was: the head is before it, in the pre.
  expect(go?.lastElementChild?.outerHTML).toBe('<code class="language-go">func main() {}\n</code>');
});

test("without a clipboard a code block's head is its language alone, and a block without a language has none", () => {
  const article = view(`${goBlock}<pre><code>plain</code></pre>`);

  codeBlocks(article, contextIn());

  const [go, plain] = [...article.querySelectorAll("pre")];
  expect(go?.firstElementChild?.className).toBe("nw-code-head");
  expect(go?.firstElementChild?.textContent).toBe("go");
  expect(article.querySelector("button")).toBeNull();
  expect(plain?.querySelector(".nw-code-head")).toBeNull();
});

test("Copy copies the code's text alone, turns to a check a moment and says Copied to a screen reader, its name the same", async () => {
  vi.useFakeTimers();
  const written: string[] = [];
  clipboard(async (text) => void written.push(text));
  const article = view(goBlock);
  codeBlocks(article, contextIn());
  const { said, copy } = parts(article.querySelector("pre") as HTMLElement);

  copy?.click();
  await answered();

  expect(written).toEqual(["func main() {}\n"]);
  expect([said?.textContent, said?.dataset.state, copy?.dataset.state]).toEqual(["Copied", "copied", "copied"]);
  expect(said?.getAttribute("aria-live")).toBe("polite");
  // No status: the page's reads of the one status (the edit's) find no other.
  expect(document.querySelector('[role="status"], output')).toBeNull();
  expect(copy?.textContent).toBe("Copy");

  await vi.advanceTimersByTimeAsync(1499);
  expect(said?.textContent).toBe("Copied");
  await vi.advanceTimersByTimeAsync(1);
  expect([said?.textContent, said?.dataset.state, copy?.dataset.state]).toEqual(["", undefined, undefined]);
  expect(copy?.textContent).toBe("Copy");
});

test("a copy refused says it couldn't, Copy as it was", async () => {
  vi.useFakeTimers();
  clipboard(() => Promise.reject(new Error("denied")));
  const article = view(goBlock);
  codeBlocks(article, contextIn());
  const { said, copy } = parts(article.querySelector("pre") as HTMLElement);

  copy?.click();
  await answered();

  expect([said?.textContent, said?.dataset.state, copy?.dataset.state]).toEqual(["Couldn't copy", "failed", undefined]);
  await vi.advanceTimersByTimeAsync(1500);
  expect([said?.textContent, said?.dataset.state]).toEqual(["", undefined]);
});

/** pending is a clipboard's answer that comes as it is told: resolved or refused. */
function pending() {
  const settle: { yes?: () => void; no?: (error: Error) => void } = {};
  const answer = new Promise<void>((yes, no) => Object.assign(settle, { yes, no }));
  return { answer, resolve: () => settle.yes?.(), reject: () => settle.no?.(new Error("denied")) };
}

test("pressed twice, only the last press tells: the first's answer, come later, neither says nor clears", async () => {
  vi.useFakeTimers();
  const first = pending();
  const third = pending();
  const answers = [first.answer, Promise.resolve(), third.answer, Promise.resolve()];
  clipboard(() => answers.shift() ?? Promise.resolve());
  const article = view(goBlock);
  codeBlocks(article, contextIn());
  const { said, copy } = parts(article.querySelector("pre") as HTMLElement);

  copy?.click();
  await vi.advanceTimersByTimeAsync(500);
  copy?.click();
  await answered();
  expect(said?.textContent).toBe("Copied");

  // The first press refused after the second copied: Copied stays, 1.5 s from the second's answer.
  await vi.advanceTimersByTimeAsync(1000);
  first.reject();
  await answered();
  expect([said?.textContent, said?.dataset.state, copy?.dataset.state]).toEqual(["Copied", "copied", "copied"]);
  await vi.advanceTimersByTimeAsync(499);
  expect(said?.textContent).toBe("Copied");
  await vi.advanceTimersByTimeAsync(1);
  expect([said?.textContent, said?.dataset.state, copy?.dataset.state]).toEqual(["", undefined, undefined]);

  // Again, the earlier press copying once the later one's word is gone: nothing comes back.
  copy?.click();
  copy?.click();
  await answered();
  await vi.advanceTimersByTimeAsync(1500);
  expect(said?.textContent).toBe("");
  third.resolve();
  await answered();
  expect([said?.textContent, said?.dataset.state, copy?.dataset.state]).toEqual(["", undefined, undefined]);
});

test("pressed again while its word shows, the word goes at once, and the last press's shows 1.5 s from its answer", async () => {
  vi.useFakeTimers();
  const second = pending();
  const answers = [Promise.reject(new Error("denied")), second.answer];
  clipboard(() => answers.shift() ?? Promise.resolve());
  const article = view(goBlock);
  codeBlocks(article, contextIn());
  const { said, copy } = parts(article.querySelector("pre") as HTMLElement);

  copy?.click();
  await answered();
  expect(said?.textContent).toBe("Couldn't copy");
  await vi.advanceTimersByTimeAsync(1000);
  copy?.click();
  expect([said?.textContent, said?.dataset.state, copy?.dataset.state]).toEqual(["", undefined, undefined]);

  await vi.advanceTimersByTimeAsync(400);
  second.resolve();
  await answered();
  expect(said?.textContent).toBe("Copied");
  // The first word's timer, due 1.5 s from its answer, was cleared by the press: it clears nothing.
  await vi.advanceTimersByTimeAsync(1499);
  expect(said?.textContent).toBe("Copied");
  await vi.advanceTimersByTimeAsync(1);
  expect(said?.textContent).toBe("");
});

test("in Chinese: 复制, 已复制, 没能复制", async () => {
  vi.useFakeTimers();
  let refuse = false;
  clipboard(() => (refuse ? Promise.reject(new Error("denied")) : Promise.resolve()));
  const article = view(goBlock);
  codeBlocks(article, contextIn("zh-CN"));
  const { said, copy } = parts(article.querySelector("pre") as HTMLElement);
  expect(copy?.textContent).toBe("复制");

  copy?.click();
  await answered();
  expect(said?.textContent).toBe("已复制");
  refuse = true;
  copy?.click();
  await answered();
  expect(said?.textContent).toBe("没能复制");
  expect(copy?.textContent).toBe("复制");
});

test("cleaned up, the heads are gone and a word's timer with them", async () => {
  vi.useFakeTimers();
  clipboard(() => Promise.resolve());
  const article = view(`${goBlock}<p>end</p>`);
  const html = article.innerHTML;
  const undo = codeBlocks(article, contextIn());
  parts(article.querySelector("pre") as HTMLElement).copy?.click();
  await answered();
  expect(vi.getTimerCount()).toBe(1);

  undo?.();

  expect(article.innerHTML).toBe(html);
  expect(vi.getTimerCount()).toBe(0);
});
