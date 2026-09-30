import { act, screen } from "@testing-library/react";
import useSWR from "swr";
import { expect, test, vi } from "vitest";

import { useStore } from "../stores/context";
import type { RootStore } from "../stores/root.store";
import { json, storedSession, testApp } from "../test/fakes";
import { renderApp } from "../test/render";

const tokens = {
  token_type: "Bearer" as const,
  access_token: "at-1",
  access_token_expires_in: 900,
  refresh_token: "rt-1",
  refresh_token_expires_at: "2026-10-31T00:00:00Z",
};

/** A page that loads through SWR, and the generations of stores it was rendered with. */
function probe() {
  let loads = 0;
  const generations = new Set<RootStore>();
  function Probe() {
    const store = useStore();
    generations.add(store);
    const { data } = useSWR("probe", () => ++loads);
    return <p>{`load ${data ?? "…"} for ${store.loginId ?? "nobody"}`}</p>;
  }
  return { routes: [{ path: "/", Component: Probe }], generations };
}

// Each login mounts the app anew with its generation (M1/P5 design 3.3):
// SWR starts with nothing cached, so the next session never sees an answer
// of the session before.
test("a sign-in starts a generation with its own SWR cache", async () => {
  const app = testApp(() => json(tokens));
  const { routes } = probe();
  renderApp("/", app, routes);
  expect(await screen.findByText("load 1 for nobody")).toBeTruthy();

  await act(() => app.session.tokens.signIn(tokens));

  expect(await screen.findByText("load 2 for login-1")).toBeTruthy();
});

// The session's state changes without its login changing (starting with
// the stored session, a first refresh that fails, then one that passes):
// the generation stays, with what it loaded.
test("the same login keeps its generation from starting through unavailable to signed in", async () => {
  let busy = true;
  const app = testApp(
    () =>
      busy ? json({ status: 503, code: "server_busy", title: "" }, 503, "application/problem+json") : json(tokens),
    storedSession("login-0")
  );
  const { routes, generations } = probe();
  renderApp("/", app, routes);
  expect(await screen.findByText("load 1 for login-0")).toBeTruthy();
  await act(async () => {
    await vi.waitFor(() => expect(app.session.tokens.state.status).toBe("unavailable"));
  });

  busy = false;
  await act(() => app.session.tokens.retry());

  expect(app.session.tokens.state).toEqual({ status: "signed-in", loginId: "login-0" });
  expect(screen.getByText("load 1 for login-0")).toBeTruthy();
  expect([...generations].map((g) => g.loginId)).toEqual(["login-0"]);
});
