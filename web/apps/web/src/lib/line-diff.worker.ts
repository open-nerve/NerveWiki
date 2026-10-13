import { lineDiff, type LineDiffResult } from "./line-diff";
import type { DiffRequest } from "./line-diff-client";

// The comparing worker (M8/P1 design 4.2): it answers each comparison with
// lineDiff's result. The page ends it when the answer is late.

/** What the worker uses of its global scope: the app's TypeScript has the DOM's types, not a worker's. */
type Scope = {
  addEventListener: (type: "message", listener: (event: MessageEvent<DiffRequest>) => void) => void;
  postMessage: (answer: LineDiffResult) => void;
};

const scope = globalThis as unknown as Scope;

scope.addEventListener("message", ({ data }) => {
  // oxlint-disable-next-line unicorn/require-post-message-target-origin -- a worker's postMessage has no target origin
  scope.postMessage(lineDiff(data.a, data.b));
});
