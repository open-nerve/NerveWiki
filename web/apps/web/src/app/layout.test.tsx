import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { signedInApp } from "../test/fakes";
import { renderApp } from "../test/render";

test("choosing a theme in the top bar applies it to the page", async () => {
  const user = userEvent.setup();
  const { app } = renderApp("/", signedInApp());
  await screen.findByRole("heading", { name: "Lab" });
  expect(document.documentElement.classList.contains("dark")).toBe(false);

  await user.click(screen.getByRole("button", { name: "Theme" }));
  await user.click(await screen.findByRole("menuitemradio", { name: "Dark" }));

  expect(app.preferences.theme).toBe("dark");
  expect(document.documentElement.classList.contains("dark")).toBe(true);
});

test("a workspace's left column is beside the one main, which holds its page (M3 handoff 1)", async () => {
  renderApp("/lab", signedInApp());
  const heading = await screen.findByRole("heading", { name: "Lab" });
  // getByRole finds one or fails.
  const main = screen.getByRole("main");
  expect(main.contains(heading)).toBe(true);
  const nav = screen.getByRole("navigation", { name: "Lab" });
  expect(main.contains(nav)).toBe(false);
  expect(nav.compareDocumentPosition(main) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
});

test("a page without a shell is the one main", async () => {
  renderApp("/settings/profile", signedInApp());
  const heading = await screen.findByRole("heading", { level: 1 });
  expect(screen.getByRole("main").contains(heading)).toBe(true);
});

test("a workspace's 404 is in the main, without the left column", async () => {
  renderApp("/nowhere", signedInApp());
  const heading = await screen.findByRole("heading", { name: "Page not found" });
  expect(screen.getByRole("main").contains(heading)).toBe(true);
  expect(screen.queryByRole("navigation", { name: "Lab" })).toBeNull();
});
