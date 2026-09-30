import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { renderApp } from "../test/render";

test("choosing a theme in the top bar applies it to the page", async () => {
  const user = userEvent.setup();
  const { store } = renderApp("/");
  await screen.findByRole("heading", { name: "Nerve Wiki" });
  expect(document.documentElement.classList.contains("dark")).toBe(false);

  await user.click(screen.getByRole("button", { name: "Theme" }));
  await user.click(await screen.findByRole("menuitemradio", { name: "Dark" }));

  expect(store.preferences.theme).toBe("dark");
  expect(document.documentElement.classList.contains("dark")).toBe(true);
});
