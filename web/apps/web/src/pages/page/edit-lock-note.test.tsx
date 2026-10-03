import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import type { NotebookRole } from "../../services/notebook.service";
import type { EditLock } from "../../services/page.service";
import { problem, userJSON } from "../../test/fakes";
import { guide, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// Who is editing the page, above its reading view, and the admins' release
// (M5/P3 design 3.10).

const bob = { user_id: "0199a2b4-0000-7000-8000-000000000002", display_name: "Bob" };
const held = (holder: EditLock["holder"], expiresIn = 60): EditLock => ({ holder, expires_in: expiresIn });

async function shown(lock: EditLock, role: NotebookRole = "admin", answers = {}) {
  const server = pageServer({ role, answers });
  server.lock = lock;
  renderApp(pagePath(guide.id), server.app);
  await screen.findByRole("article", { name: "Guide" });
  return server;
}

test("a page another member edits says who", async () => {
  await shown(held(bob));

  expect((await screen.findByRole("status")).textContent).toContain("Bob is editing this page.");
});

test("a page the account edits elsewhere says so", async () => {
  await shown(held({ user_id: userJSON.id, display_name: userJSON.display_name }));

  expect((await screen.findByRole("status")).textContent).toContain("You are editing this page elsewhere.");
});

test("a page no one edits says nothing", async () => {
  await shown({ holder: null, expires_in: null });
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));

  expect(screen.queryByRole("status")).toBeNull();
});

test("an admin releases the lock once confirmed, and the lock is read again", async () => {
  const user = userEvent.setup();
  const server = await shown(held(bob));

  await user.click(await screen.findByRole("button", { name: "Release lock" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Release the edit lock?" });
  expect(dialog.textContent).toContain("Bob's edit ends");
  await user.click(within(dialog).getByRole("button", { name: "Release lock" }));

  await waitFor(() => expect(screen.queryByRole("status")).toBeNull());
  expect(server.sent).toContain("RELEASE Guide");
});

test("a release refused stays in the dialog with the reason", async () => {
  const user = userEvent.setup();
  await shown(held(bob), "admin", { "DELETE /api/v0/pages/*/edit-lock": () => problem(403, "forbidden") });

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
    await shown(held(bob), role);

    await screen.findByRole("status");
    expect(screen.queryByRole("button", { name: "Release lock" })).toBeNull();
  }
);

test("a held lock is read again once its lease should have ended", async () => {
  const server = await shown(held(bob, 1));
  await screen.findByRole("status");

  server.lock = { holder: null, expires_in: null };

  await waitFor(() => expect(screen.queryByRole("status")).toBeNull(), { timeout: 3000 });
});
