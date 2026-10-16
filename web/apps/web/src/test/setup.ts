import { cleanup, configure } from "@testing-library/react";
import { afterEach, beforeEach, expect, vi } from "vitest";

// findBy and waitFor give up after 3 seconds, not Testing Library's 1: a
// file's first test waits for its imports and its first render, which a
// loaded machine (a load of 30, another suite's race build beside it) held
// past a second (M8 closeout B-N1). A test whose expectation holds waits no
// longer for it.
configure({ asyncUtilTimeout: 3000 });

// jsdom lays nothing out, and has no rectangles of a range: CodeMirror
// measures one to place its tooltips (the completion's, M6/P7).
Range.prototype.getClientRects ??= () => [] as unknown as DOMRectList;
Range.prototype.getBoundingClientRect ??= () => new DOMRect();

// A warning or an error on the console fails the test that caused it: React
// and React Router report misuse there (a missing HydrateFallback, a missing
// key) and nothing else would notice. A test that expects output silences
// it with mockImplementation, which also tells this check to leave it be.
beforeEach(() => {
  vi.spyOn(console, "warn");
  vi.spyOn(console, "error");
});

// Without vitest's globals Testing Library cannot unmount by itself. The app
// also writes to <html> and to localStorage: every test starts from none of it.
afterEach(() => {
  cleanup();
  for (const method of ["warn", "error"] as const) {
    const spy = vi.mocked(console[method]);
    if (!spy.getMockImplementation()) {
      expect(spy, `console.${method}`).not.toHaveBeenCalled();
    }
  }
  document.documentElement.className = "";
  document.documentElement.lang = "en";
  localStorage.clear();
});
