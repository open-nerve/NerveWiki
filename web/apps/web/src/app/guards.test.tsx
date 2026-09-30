import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import {
  byRoute,
  instanceJSON,
  json,
  problem,
  signedInApp,
  storedSession,
  testApp,
  tokensJSON,
  userJSON,
} from "../test/fakes";
import { renderApp } from "../test/render";

// The guards alone decide where the tab goes as its session changes
// (M1/P5 design 3.5).

function where(router: { state: { location: { pathname: string; search: string; hash: string } } }): string {
  const { pathname, search, hash } = router.state.location;
  return pathname + search + hash;
}

test("a signed-out tab goes to sign in, and comes back to the page with its query and fragment", async () => {
  const { router } = renderApp("/acme/page?view=list#part");

  expect(await screen.findByRole("heading", { name: "Sign in" })).toBeTruthy();
  expect(where(router)).toBe(`/sign-in?next=${encodeURIComponent("/acme/page?view=list#part")}`);
});

test("a signed-out tab on the home page goes to sign in without next", async () => {
  const { router } = renderApp("/");

  expect(await screen.findByRole("heading", { name: "Sign in" })).toBeTruthy();
  expect(where(router)).toBe("/sign-in");
});

test("a starting session shows loading, on the page's address", async () => {
  const { router } = renderApp(
    "/acme",
    testApp(() => new Promise<Response>(() => {}), storedSession("login-0"))
  );

  expect(await screen.findByText("Loading…")).toBeTruthy();
  expect(where(router)).toBe("/acme");
});

test("an unavailable session says so on the page's address; Try again signs in", async () => {
  const user = userEvent.setup();
  let busy = true;
  const app = signedInApp({
    "POST /api/v0/auth/refresh": () => (busy ? problem(503, "server_busy") : json(tokensJSON)),
  });
  const { router } = renderApp("/", app);
  expect((await screen.findByRole("alert")).textContent).toContain("Cannot reach the server");

  busy = false;
  await user.click(screen.getByRole("button", { name: "Try again" }));

  expect(await screen.findByRole("heading", { name: "Nerve Wiki" })).toBeTruthy();
  expect(where(router)).toBe("/");
});

test("an account that cannot be loaded says so; Try again loads it", async () => {
  const user = userEvent.setup();
  let down = true;
  const { router } = renderApp(
    "/acme",
    signedInApp({ "GET /api/v0/me": () => (down ? Promise.reject(new TypeError("offline")) : json(userJSON)) })
  );
  expect((await screen.findByRole("alert")).textContent).toContain("Cannot reach the server");

  down = false;
  await user.click(screen.getByRole("button", { name: "Try again" }));

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
  expect(where(router)).toBe("/acme");
});

test.each([
  ["/acme/page?view=list", "/acme/page?view=list"],
  ["//evil.example", "/"],
  ["https://evil.example/", "/"],
])("a signed-in tab on sign-in with next=%s goes to %s", async (next, to) => {
  const { router } = renderApp(`/sign-in?next=${encodeURIComponent(next)}`, signedInApp());

  await waitFor(() => expect(where(router)).toBe(to));
});

// Signing in in another tab changes this tab's session without its page
// doing anything: the guard alone takes it on.
test("a sign-in page leaves for next when the session starts from elsewhere", async () => {
  const app = testApp(
    byRoute({ "GET /api/v0/instance": () => json(instanceJSON), "GET /api/v0/me": () => json(userJSON) })
  );
  const { router } = renderApp("/sign-in?next=%2Facme", app);
  await screen.findByRole("heading", { name: "Sign in" });

  await act(() => app.session.tokens.signIn(tokensJSON));

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
  expect(where(router)).toBe("/acme");
});

test("an account with a step left goes to onboarding, which comes back to the page", async () => {
  const { router } = renderApp(
    "/acme?view=list",
    signedInApp({ "GET /api/v0/me": () => json({ ...userJSON, onboarding_steps: [] }) })
  );

  expect(await screen.findByText("Step 1 of 1")).toBeTruthy();
  expect(where(router)).toBe(`/onboarding?next=${encodeURIComponent("/acme?view=list")}`);
});

test("an account done with onboarding leaves the onboarding page for next", async () => {
  const { router } = renderApp("/onboarding?next=%2Facme", signedInApp());

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
  expect(where(router)).toBe("/acme");
});
