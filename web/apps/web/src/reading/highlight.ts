import type { Enhancement } from "./enhancement";
import { highlightMarkup } from "./highlight-markup";

/** How long a page's highlighting may take in all: then its worker is ended, the blocks not answered left plain. */
export const highlightTimeout = 2000;

/** The largest block sent to be highlighted, in bytes of UTF-8. */
export const blockLimit = 100 * 1024;

/** A code block sent to the worker: its index among the page's, its language, its text. */
export type HighlightRequest = { id: number; language: string; text: string };

/** The worker's answer for a block: highlight.js's HTML, or null for a language it does not have. */
export type HighlightAnswer = { id: number; html: string | null };

/** What highlighting uses of a Worker; the tests give a fake one. */
export type HighlightWorker = {
  addEventListener: (type: "message", listener: (event: MessageEvent<HighlightAnswer>) => void) => void;
  postMessage: (requests: HighlightRequest[]) => void;
  terminate: () => void;
};

/**
 * highlightWorker starts highlight.worker.ts: a file of the app's own
 * origin, which the CSP's script-src 'self' allows (M4/P5 design 3.9).
 */
export function highlightWorker(): HighlightWorker {
  return new Worker(new URL("./highlight.worker.ts", import.meta.url), { type: "module" });
}

const encoder = new TextEncoder();

/**
 * codeHighlight colours the reading view's code blocks that name a
 * language (M4/P5 design 3.9), in a worker that worker starts: one per
 * reading view, and none for a view without such a block. A block over
 * blockLimit is not sent. An answer is taken only as text in
 * span.hljs-* that is the block's text (highlight-markup.ts); otherwise
 * the block stays as it was. Once every block is answered, after
 * highlightTimeout, or when the view is cleaned up, the worker is ended.
 */
export function codeHighlight(worker: () => HighlightWorker): Enhancement {
  return (container) => {
    const blocks = new Map<number, { code: Element; text: string }>();
    const requests: HighlightRequest[] = [];
    // A diagram's source is drawn, not coloured (diagrams.ts).
    for (const [id, code] of container
      .querySelectorAll('pre > code[class^="language-"]:not(.language-mermaid)')
      .entries()) {
      const text = code.textContent;
      if (encoder.encode(text).length <= blockLimit) {
        blocks.set(id, { code, text });
        requests.push({ id, language: code.className.slice("language-".length), text });
      }
    }
    if (requests.length === 0) {
      return undefined;
    }
    const running = worker();
    const timer = setTimeout(() => running.terminate(), highlightTimeout);
    const end = () => {
      clearTimeout(timer);
      running.terminate();
    };
    running.addEventListener("message", ({ data }) => {
      const block = blocks.get(data.id);
      blocks.delete(data.id);
      const markup = block === undefined || data.html === null ? undefined : highlightMarkup(data.html, block.text);
      if (block !== undefined && markup !== undefined) {
        block.code.replaceChildren(markup);
      }
      if (blocks.size === 0) {
        end();
      }
    });
    // oxlint-disable-next-line unicorn/require-post-message-target-origin -- a worker's postMessage has no target origin
    running.postMessage(requests);
    return end;
  };
}
