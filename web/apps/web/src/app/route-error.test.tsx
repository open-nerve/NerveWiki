import { screen } from "@testing-library/react";
import { expect, test, vi } from "vitest";

import { renderApp } from "../test/render";

// The home page's chunk cannot be loaded, as when an older build is gone.
vi.mock("../pages/home", () => {
  throw new Error("Failed to fetch dynamically imported module");
});

test("a page that fails to load shows the error page inside the layout", async () => {
  const logged = vi.spyOn(console, "error").mockImplementation(() => {});
  renderApp("/");

  const alert = await screen.findByRole("alert");

  expect(alert.textContent).toContain("Something went wrong");
  expect(screen.getByRole("button", { name: "Reload" })).toBeTruthy();
  expect(screen.getByRole("banner")).toBeTruthy();
  expect(logged).toHaveBeenCalled();
});
