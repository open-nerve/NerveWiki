import { act, screen, waitFor } from "@testing-library/react";
import { expect, test } from "vitest";

import { byRoute, instanceJSON, json, notebookJSON, testApp } from "../test/fakes";
import { install, pagePath, pageServer } from "../test/page-server";
import { renderApp } from "../test/render";

// The tab is named after the page shown (WCAG 2.4.2): its heading, the
// places it is in, then the app.

test("the tab is named after the page shown, then the places it is in", async () => {
  const { router } = renderApp(pagePath(install.id), pageServer().app);
  await screen.findByRole("heading", { level: 1, name: "Install" });
  expect(document.title).toBe("Install · Plans · Nerve Wiki");

  const titles: [string, string][] = [
    [`/lab/notebooks/${notebookJSON.id}`, "Plans · Lab · Nerve Wiki"],
    [`/lab/notebooks/${notebookJSON.id}/settings/members`, "Members · Plans settings · Nerve Wiki"],
    ["/lab", "Lab · Nerve Wiki"],
    ["/lab/settings/general", "General · Workspace settings · Lab · Nerve Wiki"],
    ["/settings/tokens", "Access tokens · Settings · Nerve Wiki"],
    ["/lab/nothing-here", "Page not found · Nerve Wiki"],
  ];
  await titles.reduce(async (before, [path, title]) => {
    await before;
    await act(() => router.navigate(path));
    await waitFor(() => expect(document.title).toBe(title));
  }, Promise.resolve());
});

test("a signed-out tab is named after its form", async () => {
  const { unmount } = renderApp("/sign-in", testApp(byRoute({ "GET /api/v0/instance": () => json(instanceJSON) })));

  await screen.findByRole("heading", { level: 1, name: "Sign in" });
  expect(document.title).toBe("Sign in · Nerve Wiki");
  // Gone, it names the tab no more: a page that names none shows the app's name.
  unmount();
  expect(document.title).toBe("Nerve Wiki");
});
