import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { signedInApp } from "../test/fakes";
import { renderApp } from "../test/render";

test("choosing a theme in the top bar applies it to the page", async () => {
  const user = userEvent.setup();
  const { app } = renderApp("/", signedInApp());
  await screen.findByRole("heading", { name: "Nerve Wiki" });
  expect(document.documentElement.classList.contains("dark")).toBe(false);

  await user.click(screen.getByRole("button", { name: "Theme" }));
  await user.click(await screen.findByRole("menuitemradio", { name: "Dark" }));

  expect(app.preferences.theme).toBe("dark");
  expect(document.documentElement.classList.contains("dark")).toBe(true);
});
