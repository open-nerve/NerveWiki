import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { expect, test, vi } from "vitest";

import { ApiError } from "../services/api";
import { RootStore } from "../stores/root.store";
import { json, signedInApp, testApp, userJSON } from "../test/fakes";
import { renderApp } from "../test/render";
import { arrived } from "./arrival";
import { AppProviders } from "./providers";
import { RouteError } from "./route-error";

// The landing page's chunk cannot be loaded, as when an older build is gone; a workspace's home's neither.
vi.mock("../pages/landing", () => {
  throw new Error("Failed to fetch dynamically imported module");
});
vi.mock("../pages/workspace/workspace-home", () => {
  throw new Error("Failed to fetch dynamically imported module");
});

test("a page that fails to load shows the error page inside the layout", async () => {
  const logged = vi.spyOn(console, "error").mockImplementation(() => {});
  renderApp("/");

  const alert = await screen.findByRole("alert");

  expect(alert.textContent).toContain("Something went wrong");
  expect(screen.getByRole("button", { name: "Reload" })).toBeTruthy();
  // Inside the layout, with its top right: signed out, Appearance (M13/P2/S1).
  expect(screen.getByRole("button", { name: "Appearance" })).toBeTruthy();
  // The boundary logs from an effect, which may run after the alert shows.
  await waitFor(() => expect(logged).toHaveBeenCalled());
});

test("a workspace's page that fails to load on the tab's first load has the account's button at the top right", async () => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  let reads = 0;
  renderApp(
    "/lab",
    signedInApp({
      "GET /api/v0/me": () => {
        reads += 1;
        return json(userJSON);
      },
    })
  );

  expect((await screen.findByRole("alert")).textContent).toContain("Something went wrong");
  // The boundary is above the guards, which never loaded the account: the top right loads it itself (M13/P2/S1,
  // decision 792; review C-M1), once (review C, S1C-N3).
  expect(await screen.findByRole("button", { name: "Ada" })).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Appearance" })).toBeNull();
  expect(reads).toBe(1);
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
    <AppProviders store={new RootStore(testApp(), undefined)}>
      <RouterProvider router={router} />
    </AppProviders>
  );

  const alert = await screen.findByRole("alert");

  expect(alert.textContent).toContain("Error code: server_busy");
  expect(alert.textContent).not.toContain("busy.");
});

// M13/P5/S1 (decisions 921, 924): the way home, a link drawn as a button, arrived at.
test("the error page's Go to the home page leads to /, arrived at; a pill, as Reload is", async () => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  const user = userEvent.setup();
  const { router } = renderApp("/lab", signedInApp());

  const alert = await screen.findByRole("alert");
  const home = within(alert).getByRole("link", { name: "Go to the home page" });
  expect(home.getAttribute("href")).toBe("/");
  const looks = home.className.split(" ");
  expect(looks).toContain("rounded-full");
  expect(looks).not.toContain("rounded-lg");
  expect(within(alert).getByRole("button", { name: "Reload" }).className.split(" ")).toContain("rounded-full");
  await user.click(home);

  await waitFor(() => expect(router.state.location.pathname).toBe("/"));
  expect(router.state.location.state).toEqual(arrived);
});
