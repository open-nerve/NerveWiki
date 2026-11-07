import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, onTestFinished, test, vi } from "vitest";

import { pageEditor } from "./page-editor";
import { guide, pagePath, pageServer } from "./page-server";
import { renderApp } from "./render";

// A test that leaves an editor open owns its deferred session end too:
// global cleanup must finish it before the test's jsdom page is removed.
test.each(["real", "fake"] as const)("cleanup ends an active editor with %s timers", async (timers) => {
  const server = pageServer();
  const user = userEvent.setup();
  renderApp(pagePath(guide.id), server.app);
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  await pageEditor();
  expect(server.sessions.has("session-1")).toBe(true);
  expect(server.sent.filter((line) => line.startsWith("END "))).toEqual([]);

  if (timers === "fake") {
    // Freeze only teardown, after the real app has opened its editor.
    // Existing heartbeat/editor timers still need their native clear calls.
    vi.useFakeTimers({ shouldClearNativeTimers: true });
  }
  onTestFinished(async () => {
    try {
      // onTestFinished runs after the shared afterEach. Do not advance time
      // before checking: a synchronous cleanup must fail the fake variant.
      expect(screen.queryByRole("textbox", { name: "Page content" })).toBeNull();
      expect(server.sent.filter((line) => line.startsWith("END "))).toEqual(["END session-1"]);
      expect(server.sessions.has("session-1")).toBe(false);
    } finally {
      // An intentional RED must not leave the edit alive after this check.
      if (vi.isFakeTimers()) {
        try {
          await vi.advanceTimersByTimeAsync(0);
        } finally {
          vi.useRealTimers();
        }
      } else {
        await new Promise<void>((resolve) => setTimeout(resolve, 0));
      }
    }
  });
});
