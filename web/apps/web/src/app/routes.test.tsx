import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { problem, signedInApp } from "../test/fakes";
import { renderApp } from "../test/render";

test("the home page shows what the instance runs, inside the layout", async () => {
  renderApp("/", signedInApp());

  expect(await screen.findByRole("heading", { name: "Nerve Wiki" })).toBeTruthy();
  expect(screen.getByText("Version 1.2.3 (4f2a9c1)")).toBeTruthy();
  expect(screen.getByText("API v0")).toBeTruthy();
  expect(screen.getByRole("banner").textContent).toContain("Nerve Wiki");
});

test("the home page says so when the instance cannot be loaded", async () => {
  renderApp("/", signedInApp({ "GET /api/v0/instance": () => problem(404, "not_found") }));

  expect((await screen.findByRole("alert")).textContent).toBe("The instance information could not be loaded.");
});

test("a path that is no page shows the app's 404, with a way home", async () => {
  const user = userEvent.setup();
  const { router } = renderApp("/acme/nothing-here", signedInApp());

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
  await user.click(screen.getByRole("link", { name: "Go to the home page" }));
  expect(router.state.location.pathname).toBe("/");
});

test("choosing a language changes the text and <html lang>", async () => {
  const user = userEvent.setup();
  const { app } = renderApp("/", signedInApp());
  await screen.findByText("Version 1.2.3 (4f2a9c1)");

  await user.click(screen.getByRole("button", { name: "Language" }));
  await user.click(await screen.findByRole("menuitemradio", { name: "简体中文" }));

  expect(app.preferences.locale).toBe("zh-CN");
  expect(screen.getByText("版本 1.2.3（4f2a9c1）")).toBeTruthy();
  expect(screen.getByRole("button", { name: "主题" })).toBeTruthy();
  expect(document.documentElement.lang).toBe("zh-CN");
});
