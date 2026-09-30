import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

// Without vitest's globals Testing Library cannot unmount by itself. The app
// also writes to <html> and to localStorage: every test starts from none of it.
afterEach(() => {
  cleanup();
  document.documentElement.className = "";
  document.documentElement.lang = "en";
  localStorage.clear();
});
