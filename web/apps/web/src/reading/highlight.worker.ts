import { highlightBlock } from "./highlight-block";
import type { HighlightAnswer, HighlightRequest } from "./highlight";

// The highlighting worker (M4/P5 design 3.9): it answers each block of a
// request in turn, so that a page's first blocks are coloured though its
// last are not answered in time.

/** What the worker uses of its global scope: the app's TypeScript has the DOM's types, not a worker's. */
type Scope = {
  addEventListener: (type: "message", listener: (event: MessageEvent<HighlightRequest[]>) => void) => void;
  postMessage: (answer: HighlightAnswer) => void;
};

const scope = globalThis as unknown as Scope;

scope.addEventListener("message", ({ data }) => {
  for (const request of data) {
    // oxlint-disable-next-line unicorn/require-post-message-target-origin -- a worker's postMessage has no target origin
    scope.postMessage(highlightBlock(request));
  }
});
