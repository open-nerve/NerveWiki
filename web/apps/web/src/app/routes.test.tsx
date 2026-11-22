import { fireEvent, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { signedInApp } from "../test/fakes";
import { renderApp } from "../test/render";

test("a path that is no page shows the app's 404, with a way home", async () => {
  const user = userEvent.setup();
  const { router } = renderApp("/acme/nothing-here", signedInApp());

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
  await user.click(screen.getByRole("link", { name: "Go to the home page" }));
  expect(await screen.findByRole("heading", { name: "Lab" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/lab");
});

test("choosing a language in the account's Appearance changes the text and <html lang>", async () => {
  const user = userEvent.setup();
  const { app } = renderApp("/lab", signedInApp());
  await screen.findByRole("heading", { name: "Lab" });

  await user.click(screen.getByRole("button", { name: "Ada" }));
  await user.click(await screen.findByRole("menuitem", { name: "Appearance" }));
  // Clicked, not moved to: without a layout Radix takes the pointer leaving Appearance for one going elsewhere.
  fireEvent.click(await screen.findByRole("menuitemradio", { name: "简体中文" }));

  expect(app.preferences.locale).toBe("zh-CN");
  // The menu stays open (M13/P2/S1, decision 770), in Chinese already; what it hides from the rest of the page shows
  // once Escape closes all of it (review C-M3).
  expect(screen.getByRole("menuitem", { name: "外观" })).toBeTruthy();
  await user.keyboard("{Escape}");
  expect(await screen.findByRole("link", { name: "首页" })).toBeTruthy();
  expect(document.documentElement.lang).toBe("zh-CN");
});
