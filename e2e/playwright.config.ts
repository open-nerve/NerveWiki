import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "stories",
  // Starts PostgreSQL and migrates the template database once per run.
  globalSetup: "./global-setup.ts",
  forbidOnly: !!process.env.CI,
  // The html report keeps the traces and screenshots of failed tests; CI uploads it. In CI each failure is also an
  // annotation of the run, which reads without the job's log.
  reporter: [["list"], ["html", { open: "never" }], ...(process.env.CI ? [["github"] as const] : [])],
  use: {
    // The stories read the page's English text.
    locale: "en-US",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
