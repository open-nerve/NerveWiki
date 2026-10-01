import { screen } from "@testing-library/react";
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

test("choosing a language changes the text and <html lang>", async () => {
  const user = userEvent.setup();
  const { app } = renderApp("/lab", signedInApp());
  await screen.findByRole("heading", { name: "Lab" });

  await user.click(screen.getByRole("button", { name: "Language" }));
  await user.click(await screen.findByRole("menuitemradio", { name: "简体中文" }));

  expect(app.preferences.locale).toBe("zh-CN");
  expect(screen.getByRole("link", { name: "首页" })).toBeTruthy();
  expect(screen.getByRole("button", { name: "主题" })).toBeTruthy();
  expect(document.documentElement.lang).toBe("zh-CN");
});
