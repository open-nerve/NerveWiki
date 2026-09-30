import { screen } from "@testing-library/react";
import { expect, test } from "vitest";

import { renderApp } from "../test/render";

test("the home page is the index route, inside the layout", async () => {
  renderApp("/");

  expect(await screen.findByRole("heading", { name: "Nerve Wiki" })).toBeTruthy();
  expect(screen.getByRole("banner").textContent).toContain("Nerve Wiki");
});
