import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { byRoute, instanceJSON, json, problem, testApp, tokensJSON, userJSON, type Answer } from "../test/fakes";
import { renderApp } from "../test/render";

/** A signed-out tab whose sign-up answers register; what it sent is in sent. */
function signUpWith(register: Answer, instance = instanceJSON) {
  const sent: unknown[] = [];
  const app = testApp(
    byRoute({
      "GET /api/v0/instance": () => json(instance),
      "POST /api/v0/auth/register": async (request) => {
        sent.push(await request.json());
        return register(request);
      },
      "GET /api/v0/me": () => json(userJSON),
    })
  );
  return { app, sent };
}

async function fillIn(email: string, password: string) {
  const user = userEvent.setup();
  await user.type(await screen.findByLabelText("E-mail address"), email);
  await user.type(screen.getByLabelText("Password"), password);
  await user.click(screen.getByRole("button", { name: "Sign up" }));
}

function noteOf(label: string): string | null | undefined {
  const field = screen.getByLabelText(label);
  return document.getElementById(field.getAttribute("aria-describedby") ?? "")?.textContent;
}

test("signing up signs in to the new account", async () => {
  const { app, sent } = signUpWith(() => json(tokensJSON, 201));
  const { router } = renderApp("/sign-up", app);

  await fillIn("ada@example.com", "correct horse");

  expect(await screen.findByRole("button", { name: "Ada" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/");
  expect(sent).toEqual([{ email: "ada@example.com", password: "correct horse" }]);
});

test("a password of the wrong length is marked under it, and nothing is sent", async () => {
  const { app, sent } = signUpWith(() => json(tokensJSON, 201));
  renderApp("/sign-up", app);
  await screen.findByLabelText("Password");
  expect(noteOf("Password")).toBe("8 to 128 characters.");

  await fillIn("ada@example.com", "short");

  expect(noteOf("Password")).toBe("At least 8 characters.");
  expect(sent).toEqual([]);
});

test("the server's problems with a field show under it, with nothing above the form", async () => {
  const refused = problem(422, "validation_failed", { errors: [{ field: "password", code: "common_password" }] });
  renderApp("/sign-up", signUpWith(() => refused).app);

  await fillIn("ada@example.com", "password1");

  await screen.findByText("Too common, or too close to the e-mail address.");
  expect(noteOf("Password")).toBe("Too common, or too close to the e-mail address.");
  expect(screen.queryByRole("alert")).toBeNull();
});

test.each([
  [problem(409, "identity.email_taken"), "An account with this e-mail address already exists."],
  [problem(403, "identity.signup_disabled"), "Sign-up is disabled on this server."],
])("a refused sign-up says why above the form (%#)", async (answer, text) => {
  renderApp("/sign-up", signUpWith(() => answer).app);

  await fillIn("ada@example.com", "correct horse");

  expect((await screen.findByRole("alert")).textContent).toBe(text);
});

test("while sign-up is closed the page says so, with the way back to sign in", async () => {
  renderApp(
    "/sign-up?next=%2Facme",
    signUpWith(() => json(tokensJSON, 201), { ...instanceJSON, signup_enabled: false }).app
  );

  expect(
    await screen.findByText("This server does not take new accounts. Ask its administrator for one.")
  ).toBeTruthy();
  expect(screen.getByRole("link", { name: "Sign in" }).getAttribute("href")).toBe("/sign-in?next=%2Facme");
  expect(screen.queryByLabelText("Password")).toBeNull();
});
