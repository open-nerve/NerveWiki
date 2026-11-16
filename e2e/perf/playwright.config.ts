import path from "node:path";

import { defineConfig, devices } from "@playwright/test";

// perf's own run (M12/P4 design 4.1): pnpm perf, from e2e, through heavy.sh. Not a story: make e2e runs the
// stories' config, whose testDir is stories. knip reads this file from the perf script: the testMatch is a string
// glob (its Playwright plugin drops a regular expression) and the globalSetup an absolute path (it resolves a
// relative one from the workspace's directory).
export default defineConfig({
  testDir: ".",
  testMatch: "**/*.perf.ts",
  globalSetup: path.join(import.meta.dirname, "setup.ts"),
  // One measure at a time, on one serve.
  workers: 1,
  fullyParallel: false,
  timeout: 30 * 60_000,
  outputDir: "../test-results/perf",
  reporter: [["list"]],
  use: {
    locale: "en-US",
    trace: "off",
    screenshot: "off",
  },
  // Chromium, its default viewport of 1280 × 720 (00-M12 4.7.2).
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
