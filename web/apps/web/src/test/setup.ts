import { cleanup } from "@testing-library/react";
import { afterEach, beforeEach, expect, vi } from "vitest";

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
