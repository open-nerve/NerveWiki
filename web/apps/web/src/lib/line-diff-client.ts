import { lineDiff, type LineDiffResult } from "./line-diff";

/** How long a comparison may take in its worker: then the worker is ended, and the texts are too far apart. */
const diffTimeout = 2000;

/** The most bytes of UTF-8, both texts together, compared line by line (M8 design 4.3). */
export const maxDiffBytes = 5 * 1024 * 1024;

/**
 * Texts this small, both together, are compared on the page's own thread
 * at once: at most a million steps and a little refining, which a worker's
 * start would cost more than.
 */
export const smallDiff = { lines: 1000, length: 32 * 1024 };

/** A comparison sent to the worker: the two texts, split as the editor holds them. */
export type DiffRequest = { a: string; b: string };

/** What comparing uses of a Worker; the tests give a fake one. */
export type DiffWorker = {
  addEventListener: (type: "message", listener: (event: MessageEvent<LineDiffResult>) => void) => void;
  postMessage: (request: DiffRequest) => void;
  terminate: () => void;
};

/**
 * lineDiffWorker starts line-diff.worker.ts: a file of the app's own
 * origin, which the CSP's script-src 'self' allows, as the highlighting's.
 */
function lineDiffWorker(): DiffWorker {
  return new Worker(new URL("./line-diff.worker.ts", import.meta.url), { type: "module" });
}

/** compareNow is lineDiff of two small texts, on the page's thread; undefined for larger ones. */
export function compareNow(a: string, b: string): LineDiffResult | undefined {
  if (a.length + b.length > smallDiff.length || lineCount(a) + lineCount(b) > smallDiff.lines) {
    return undefined;
  }
  return lineDiff(a, b);
}

const encoder = new TextEncoder();
const tooLarge: LineDiffResult = { kind: "tooLarge", added: null, removed: null };

/**
 * compareInWorker is lineDiff of two texts in a worker that start starts
 * (M8/P1 design 4.2): tooLarge for texts of more than maxDiffBytes
 * together, without a worker, and once timeout passes, the worker ended.
 * When signal aborts, the worker is ended and the answer is the signal's
 * reason, rejected: a newer comparison of the same view replaced it.
 */
export function compareInWorker(
  a: string,
  b: string,
  signal: AbortSignal,
  start: () => DiffWorker = lineDiffWorker,
  timeout = diffTimeout
): Promise<LineDiffResult> {
  if (signal.aborted) {
    return Promise.reject(signal.reason);
  }
  if (encoder.encode(a).length + encoder.encode(b).length > maxDiffBytes) {
    return Promise.resolve(tooLarge);
  }
  return new Promise((resolve, reject) => {
    const running = start();
    const end = () => {
      clearTimeout(timer);
      signal.removeEventListener("abort", abort);
      running.terminate();
    };
    const abort = () => {
      end();
      reject(signal.reason);
    };
    const timer = setTimeout(() => {
      end();
      resolve(tooLarge);
    }, timeout);
    signal.addEventListener("abort", abort);
    running.addEventListener("message", ({ data }) => {
      end();
      resolve(data);
    });
    // oxlint-disable-next-line unicorn/require-post-message-target-origin -- a worker's postMessage has no target origin
    running.postMessage({ a, b });
  });
}

function lineCount(text: string): number {
  let count = 1;
  for (let at = text.indexOf("\n"); at !== -1; at = text.indexOf("\n", at + 1)) {
    count++;
  }
  return count;
}
