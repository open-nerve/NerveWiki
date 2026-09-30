import { render, screen } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { expect, test } from "vitest";

import { routes } from "./routes";

test("the home page is the index route", async () => {
  render(<RouterProvider router={createMemoryRouter(routes, { initialEntries: ["/"] })} />);

  expect(await screen.findByRole("heading", { name: "Nerve Wiki" })).toBeTruthy();
});
