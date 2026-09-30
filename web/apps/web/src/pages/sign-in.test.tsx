import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { byRoute, instanceJSON, json, problem, testApp, tokensJSON, userJSON, type Answer } from "../test/fakes";
import { renderApp } from "../test/render";

/** A signed-out tab whose sign-in answers login; what it sent is in sent. */
function signInWith(login: Answer, instance = instanceJSON) {
  const sent: unknown[] = [];
  const app = testApp(
    byRoute({
      "GET /api/v0/instance": () => json(instance),
      "POST /api/v0/auth/login": async (request) => {
        sent.push(await request.json());
        return login(request);
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
  await user.click(screen.getByRole("button", { name: "Sign in" }));
  return user;
}

test("signing in goes to next", async () => {
  const { app, sent } = signInWith(() => json(tokensJSON));
  const { router } = renderApp("/sign-in?next=%2Facme", app);

  await fillIn(" ada@example.com ", "correct horse");

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/acme");
  expect(sent).toEqual([{ email: "ada@example.com", password: "correct horse" }]);
});

test.each([
  [problem(401, "identity.invalid_credentials"), "The e-mail address or the password is incorrect."],
  [
    problem(403, "identity.account_deactivated"),
    "This account is deactivated. Ask the server's administrator to activate it.",
  ],
  [problem(429, "rate_limited", {}, { "Retry-After": "30" }), "Too many attempts. Try again in 30 s."],
])("a refused sign-in says why above the form and keeps what was typed (%#)", async (answer, text) => {
  renderApp("/sign-in", signInWith(() => answer.clone()).app);

  await fillIn("ada@example.com", "wrong");

  expect((await screen.findByRole("alert")).textContent).toBe(text);
  expect(screen.getByLabelText<HTMLInputElement>("E-mail address").value).toBe("ada@example.com");
  expect(screen.getByLabelText<HTMLInputElement>("Password").value).toBe("wrong");
  expect(screen.getByRole("button", { name: "Sign in" })).toHaveProperty("disabled", false);
});

test("an empty field is marked and focused, and nothing is sent", async () => {
  const { app, sent } = signInWith(() => json(tokensJSON));
  renderApp("/sign-in", app);
  const user = userEvent.setup();

  await user.click(await screen.findByRole("button", { name: "Sign in" }));

  const email = screen.getByLabelText("E-mail address");
  expect(email.getAttribute("aria-invalid")).toBe("true");
  expect(document.getElementById(email.getAttribute("aria-describedby") ?? "")?.textContent).toBe("Required.");
  expect(screen.getByLabelText("Password").getAttribute("aria-invalid")).toBe("true");
  expect(document.activeElement).toBe(email);
  expect(sent).toEqual([]);
});

test("the form cannot be sent again while it is being sent", async () => {
  let answer: ((response: Response) => void) | undefined;
  const { app, sent } = signInWith(() => new Promise<Response>((resolve) => (answer = resolve)));
  renderApp("/sign-in", app);

  const user = await fillIn("ada@example.com", "correct horse");
  await waitFor(() => expect(screen.getByRole("button", { name: "Sign in" })).toHaveProperty("disabled", true));
  await user.click(screen.getByRole("button", { name: "Sign in" }));

  answer?.(problem(401, "identity.invalid_credentials"));
  await screen.findByRole("alert");
  expect(sent).toHaveLength(1);
});

test("the password can be shown and hidden again", async () => {
  const user = userEvent.setup();
  renderApp("/sign-in", signInWith(() => json(tokensJSON)).app);
  const password = await screen.findByLabelText("Password");

  const toggle = screen.getByRole("button", { name: "Show password" });

  await user.click(toggle);
  expect(password.getAttribute("type")).toBe("text");
  expect(toggle.getAttribute("aria-pressed")).toBe("true");
  await user.click(toggle);
  expect(password.getAttribute("type")).toBe("password");
  expect(toggle.getAttribute("aria-pressed")).toBe("false");
});

test("the sign-up link keeps next, and shows only while sign-up is open", async () => {
  const { unmount } = renderApp("/sign-in?next=%2Facme", signInWith(() => json(tokensJSON)).app);
  expect((await screen.findByRole("link", { name: "Sign up" })).getAttribute("href")).toBe("/sign-up?next=%2Facme");
  unmount();

  renderApp("/sign-in", signInWith(() => json(tokensJSON), { ...instanceJSON, signup_enabled: false }).app);
  await screen.findByRole("heading", { name: "Sign in" });
  await waitFor(() => expect(screen.queryByText("No account yet?")).toBeNull());
  expect(screen.queryByRole("link", { name: "Sign up" })).toBeNull();
});
