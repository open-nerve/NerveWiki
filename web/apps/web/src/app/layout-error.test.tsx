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
  expect(screen.queryByRole("banner")).toBeNull();
});
