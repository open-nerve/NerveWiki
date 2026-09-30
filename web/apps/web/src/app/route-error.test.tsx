import { render, screen, waitFor } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { expect, test, vi } from "vitest";

import { ApiError } from "../services/api";
import { RootStore } from "../stores/root.store";
import { fakeApi, json, preferences } from "../test/fakes";
import { renderApp } from "../test/render";
import { AppProviders } from "./providers";
import { RouteError } from "./route-error";

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
  // The boundary logs from an effect, which may run after the alert shows.
  await waitFor(() => expect(logged).toHaveBeenCalled());
});

function Refused(): never {
  throw new ApiError(503, { status: 503, code: "server_busy", title: "Service Unavailable", detail: "busy" });
}

// An exception while a page renders, here the API's refusal, shows the error
// page with the API's code and nothing else of the exception.
test("an ApiError thrown while rendering shows its code", async () => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  const router = createMemoryRouter([{ path: "/", Component: Refused, ErrorBoundary: RouteError }]);
  render(
    <AppProviders
      store={
        new RootStore(
          preferences(),
          fakeApi(() => json({}))
        )
      }
    >
      <RouterProvider router={router} />
    </AppProviders>
  );

  const alert = await screen.findByRole("alert");

  expect(alert.textContent).toContain("Error code: server_busy");
  expect(alert.textContent).not.toContain("busy.");
});
