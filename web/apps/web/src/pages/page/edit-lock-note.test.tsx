import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import type { NotebookRole } from "../../services/notebook.service";
import { problem } from "../../test/fakes";
import { ada, bob, guide, pagePath, pageServer, type Person } from "../../test/page-server";
import { renderApp } from "../../test/render";

// Who is editing the page, above its reading view, and the admins' release
// (M5/P3 design 3.10).

/** Guide shown to Ada with role, holder holding its lock for expiresIn seconds (none: no one). */
async function shown(holder: Person | null, role: NotebookRole = "admin", answers = {}, expiresIn = 60) {
  const server = pageServer({ role, answers });
  if (holder !== null) {
    server.hold(guide.id, holder, expiresIn);
  }
  renderApp(pagePath(guide.id), server.app);
  await screen.findByRole("article", { name: "Guide" });
  return server;
}

test("a page another member edits says who", async () => {
  await shown(bob);

  expect((await screen.findByRole("status")).textContent).toContain("Bob is editing this page.");
});

test("a page the account edits elsewhere says so", async () => {
  await shown(ada);

  expect((await screen.findByRole("status")).textContent).toContain("You are editing this page elsewhere.");
});

test("a page no one edits says nothing", async () => {
  await shown(null);
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));

  expect(screen.queryByRole("status")).toBeNull();
});

test("an admin releases the lock once confirmed, and the lock is read again, the focus on Edit", async () => {
  const user = userEvent.setup();
  const server = await shown(bob);

  await user.click(await screen.findByRole("button", { name: "Release lock" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Release the edit lock?" });
  expect(dialog.textContent).toContain("Bob's edit ends");
  await user.click(within(dialog).getByRole("button", { name: "Release lock" }));

  await waitFor(() => expect(screen.queryByRole("status")).toBeNull());
  expect(server.sent).toContain("RELEASE Guide");
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("button", { name: "Edit" })));
});

test("a release refused stays in the dialog with the reason", async () => {
  const user = userEvent.setup();
  await shown(bob, "admin", { "DELETE /api/v0/pages/*/edit-lock": () => problem(403, "forbidden") });

  await user.click(await screen.findByRole("button", { name: "Release lock" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Release the edit lock?" });
  await user.click(within(dialog).getByRole("button", { name: "Release lock" }));

  expect(await within(dialog).findByRole("alert")).toBeTruthy();
  // The page behind the dialog is hidden from the accessibility tree while it is open.
  expect(screen.getByRole("status", { hidden: true }).textContent).toContain("Bob is editing this page.");
});

test.each(["editor", "reader"] as const)(
  "in the notebook as %s, one sees who edits, with nothing to release",
  async (role) => {
    await shown(bob, role);

    await screen.findByRole("status");
    expect(screen.queryByRole("button", { name: "Release lock" })).toBeNull();
  }
);

test("a held lock is read again once its lease should have ended", async () => {
  const server = await shown(bob, "admin", {}, 1);
  await screen.findByRole("status");

  server.sessions.clear();

  await waitFor(() => expect(screen.queryByRole("status")).toBeNull(), { timeout: 3000 });
});
