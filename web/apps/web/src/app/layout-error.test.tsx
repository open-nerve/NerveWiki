import { screen } from "@testing-library/react";
import { expect, test, vi } from "vitest";

import { renderApp } from "../test/render";

// The layout itself fails: the root route's error boundary replaces it.
vi.mock("./layout", () => ({
  Layout: () => {
    throw new Error("the layout broke");
  },
}));

test("an error of the layout shows the error page in its place", async () => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  renderApp("/");

  expect((await screen.findByRole("alert")).textContent).toContain("Something went wrong");
  // Outside the layout, no top right (M13/P2/S1, decision 769; review A-M6).
  expect(screen.queryByRole("button", { name: "Appearance" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Ada" })).toBeNull();
});
