import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { signedInApp } from "../../test/fakes";
import { renderApp } from "../../test/render";

// The appearance and the language, a page of the account's settings (M13/P3/S1, decision 824).

test("the theme and the language are the account menu's: chosen here, they apply at once", async () => {
  const user = userEvent.setup();
  const app = signedInApp();
  renderApp("/settings/appearance", app);
  await screen.findByRole("heading", { level: 1, name: "Appearance and language" });

  await user.click(screen.getByRole("radio", { name: "Dark" }));
  expect(app.preferences.theme).toBe("dark");
  expect(document.documentElement.classList.contains("dark")).toBe(true);
  // The choice shows as it is made, nothing else drawn again (the rows' controls are drawn by a function SettingsRow
  // calls: what it reads, the page's observer would not follow).
  expect(screen.getByRole("radio", { name: "Dark" })).toHaveProperty("checked", true);
  expect(screen.getByRole("radio", { name: "System" })).toHaveProperty("checked", false);

  await user.click(screen.getByRole("radio", { name: "简体中文" }));
  expect(app.preferences.locale).toBe("zh-CN");
  expect(await screen.findByRole("heading", { level: 1, name: "外观与语言" })).toBeTruthy();
  expect(screen.getByRole("radio", { name: "深色" })).toHaveProperty("checked", true);

  // The account menu's Appearance shows the same choice (M13/P2/S1).
  await user.click(screen.getByRole("button", { name: "Ada" }));
  await user.click(await screen.findByRole("menuitem", { name: "外观" }));
  await waitFor(() =>
    expect(screen.getByRole("menuitemradio", { name: "深色" }).getAttribute("aria-checked")).toBe("true")
  );
});

test("its two groups of choices are named by their rows: Theme, described as kept in this browser, and Language", async () => {
  renderApp("/settings/appearance", signedInApp());

  const theme = await screen.findByRole("radiogroup", { name: "Theme" });
  expect(
    within(theme)
      .getAllByRole("radio")
      .map((radio) => radio.getAttribute("value"))
  ).toEqual(["system", "light", "dark"]);
  const described = theme.getAttribute("aria-describedby");
  expect(described === null ? undefined : document.getElementById(described)?.textContent).toBe(
    "Kept in this browser."
  );
  const language = screen.getByRole("radiogroup", { name: "Language" });
  expect(
    within(language)
      .getAllByRole("radio")
      .map((radio) => radio.getAttribute("value"))
  ).toEqual(["zh-CN", "en"]);
});
