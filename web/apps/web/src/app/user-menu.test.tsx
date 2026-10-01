import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { signedInApp } from "../test/fakes";
import { renderApp } from "../test/render";

test("signing out from the user menu ends the session and goes to sign in", async () => {
  const user = userEvent.setup();
  const sent: string[] = [];
  const app = signedInApp({
    "POST /api/v0/auth/logout": (request) => {
      sent.push(new URL(request.url).pathname);
      return new Response(null, { status: 204 });
    },
  });
  const { router } = renderApp("/acme", app);

  await user.click(await screen.findByRole("button", { name: "Ada" }));
  await user.click(await screen.findByRole("menuitem", { name: "Sign out" }));

  expect(await screen.findByRole("heading", { name: "Sign in" })).toBeTruthy();
  expect(app.session.tokens.state.status).toBe("signed-out");
  expect(router.state.location.search).toBe("?next=%2Facme");
  expect(sent).toEqual(["/api/v0/auth/logout"]);
  expect(screen.queryByRole("button", { name: "Ada" })).toBeNull();
});

test("the user menu says which version the server runs", async () => {
  const user = userEvent.setup();
  renderApp("/lab", signedInApp());

  await user.click(await screen.findByRole("button", { name: "Ada" }));

  expect(await screen.findByText("Nerve Wiki 1.2.3 (4f2a9c1)")).toBeTruthy();
});
