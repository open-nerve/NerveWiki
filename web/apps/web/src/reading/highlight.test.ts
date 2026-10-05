import { afterEach, expect, test, vi } from "vitest";

import type { ReadingContext } from "./enhancement";
import {
  blockLimit,
  codeHighlight,
  highlightTimeout,
  type HighlightAnswer,
  type HighlightRequest,
  type HighlightWorker,
} from "./highlight";
import { highlightBlock } from "./highlight-block";
import { translator } from "../i18n/i18n";

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

afterEach(() => {
  vi.useRealTimers();
});

/** A fake worker: it answers each block at once as highlight.worker.ts does, or with answer, or not at all. */
class FakeWorker implements HighlightWorker {
  private listener: ((event: MessageEvent<HighlightAnswer>) => void) | undefined;
  sent: HighlightRequest[] = [];
  terminated = false;

  constructor(private readonly answer: ((request: HighlightRequest) => HighlightAnswer) | "silent" = highlightBlock) {}

  addEventListener(_type: "message", listener: (event: MessageEvent<HighlightAnswer>) => void) {
    this.listener = listener;
  }

  postMessage(requests: HighlightRequest[]) {
    this.sent.push(...requests);
    for (const request of requests) {
      if (this.answer !== "silent") {
        this.listener?.(new MessageEvent("message", { data: this.answer(request) }));
      }
    }
  }

  terminate() {
    this.terminated = true;
  }
}

/** view is a reading view's container holding html. */
function view(html: string): HTMLElement {
  const container = document.createElement("div");
  container.innerHTML = html;
  return container;
}

/** run runs code highlighting on container with worker, and tells how often it started one. */
function run(container: HTMLElement, worker: FakeWorker) {
  const start = vi.fn(() => worker);
  const undo = codeHighlight(start)(container, context);
  return { start, undo };
}

test("the blocks that name a language are coloured in one worker, ended once all are answered", () => {
  const container = view(
    '<pre><code class="language-go">func main() {}</code></pre>' +
      '<pre><code class="language-js">let a = "&lt;b&gt;";</code></pre>' +
      "<pre><code>plain()</code></pre><p><code>inline()</code></p>"
  );
  const worker = new FakeWorker();

  const { start } = run(container, worker);

  expect(start).toHaveBeenCalledTimes(1);
  expect(worker.sent.map((request) => [request.id, request.language])).toEqual([
    [0, "go"],
    [1, "js"],
  ]);
  const [go, js, plain] = container.querySelectorAll("pre > code");
  expect(go?.querySelector("span.hljs-keyword")?.textContent).toBe("func");
  expect(go?.textContent).toBe("func main() {}");
  expect(js?.querySelector("span.hljs-string")?.textContent).toBe('"<b>"');
  expect(plain?.innerHTML).toBe("plain()");
  expect(worker.terminated).toBe(true);
});

test("a view without a block that names a language starts no worker", () => {
  const { start, undo } = run(view("<pre><code>plain()</code></pre>"), new FakeWorker());

  expect(start).not.toHaveBeenCalled();
  expect(undo).toBeUndefined();
});

test("a diagram's source is not coloured: it is drawn", () => {
  const worker = new FakeWorker();
  const container = view(
    '<pre><code class="language-mermaid">graph TD; a-->b</code></pre><pre><code class="language-go">x</code></pre>'
  );
  run(container, worker);
  expect(worker.sent.map((request) => request.language)).toEqual(["go"]);
  expect(container.querySelector("code.language-mermaid")?.innerHTML).toBe("graph TD; a--&gt;b");
  const start = vi.fn(() => new FakeWorker());
  expect(codeHighlight(start)(view('<pre><code class="language-mermaid">a</code></pre>'), context)).toBeUndefined();
  expect(start).not.toHaveBeenCalled();
});

test("a language highlight.js does not have stays plain", () => {
  const container = view('<pre><code class="language-cobol">DISPLAY "A".</code></pre>');
  const worker = new FakeWorker();

  run(container, worker);

  expect(worker.sent).toHaveLength(1);
  expect(container.querySelector("code")?.innerHTML).toBe('DISPLAY "A".');
  expect(worker.terminated).toBe(true);
});

test("a block over 100 KB of UTF-8 is not sent, one of 100 KB is", () => {
  // Two bytes a character: half the limit and one more is over it.
  const over = "é".repeat(blockLimit / 2 + 1);
  const container = view(
    `<pre><code class="language-go">${over}</code></pre><pre><code class="language-go">${"a".repeat(blockLimit)}</code></pre>`
  );
  const worker = new FakeWorker("silent");

  run(container, worker);

  expect(worker.sent.map((request) => request.id)).toEqual([1]);
});

test("an answer that is not only highlight.js's spans leaves its block plain", () => {
  const container = view('<pre><code class="language-go">func</code></pre>');

  run(container, new FakeWorker(({ id }) => ({ id, html: '<a href="https://example.com">func</a>' })));

  expect(container.querySelector("code")?.innerHTML).toBe("func");
});

test("a worker that does not answer is ended after two seconds; its blocks stay plain", () => {
  vi.useFakeTimers();
  const container = view('<pre><code class="language-go">func</code></pre>');
  const worker = new FakeWorker("silent");

  run(container, worker);
  vi.advanceTimersByTime(highlightTimeout - 1);
  expect(worker.terminated).toBe(false);
  vi.advanceTimersByTime(1);

  expect(highlightTimeout).toBe(2000);
  expect(worker.terminated).toBe(true);
  expect(container.querySelector("code")?.innerHTML).toBe("func");
});

test("undoing the highlighting ends its worker and its timer", () => {
  vi.useFakeTimers();
  const worker = new FakeWorker("silent");

  const { undo } = run(view('<pre><code class="language-go">func</code></pre>'), worker);
  undo?.();

  expect(worker.terminated).toBe(true);
  expect(vi.getTimerCount()).toBe(0);
});
