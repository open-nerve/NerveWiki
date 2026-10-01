import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { AUTH_KEY } from "../../session/token-manager";
import {
  byRoute,
  instanceJSON,
  json,
  problem,
  storedSession,
  testApp,
  tokensJSON,
  userJSON,
  type Answer,
} from "../../test/fakes";
import { renderApp } from "../../test/render";

/**
 * A tab signed in from its stored session (stored, which the test reads)
 * on the security page; what it sent is in sent, as "METHOD /path" and
 * the body when there is one.
 */
function securityPage(routes: Record<string, Answer> = {}) {
  const sent: string[] = [];
  const stored = storedSession("login-0");
  const answers: Record<string, Answer> = {
    "POST /api/v0/auth/refresh": () => json(tokensJSON),
    "GET /api/v0/me": () => json(userJSON),
    "GET /api/v0/instance": () => json(instanceJSON),
    "POST /api/v0/me/change-password": () => new Response(null, { status: 204 }),
    "POST /api/v0/me/deactivate": () => new Response(null, { status: 204 }),
    ...routes,
  };
  const app = testApp(async (request) => {
    const route = `${request.method} ${new URL(request.url).pathname}`;
    const body = request.method === "POST" ? await request.clone().text() : "";
    if (!route.includes("/auth/refresh") && !route.startsWith("GET")) {
      sent.push(body === "" ? route : `${route} ${body}`);
    }
    return byRoute(answers)(request);
  }, stored);
  const rendered = renderApp("/settings/security", app);
  return { ...rendered, app, sent, stored };
}

async function changePassword(current: string, next: string) {
  const user = userEvent.setup();
  const currentField = await screen.findByLabelText("Current password");
  await user.clear(currentField);
  if (current !== "") {
    await user.type(currentField, current);
  }
  await user.clear(screen.getByLabelText("New password"));
  if (next !== "") {
    await user.type(screen.getByLabelText("New password"), next);
  }
  await user.click(screen.getByRole("button", { name: "Change password" }));
}

function noteOf(label: string): string | null | undefined {
  const field = screen.getByLabelText(label);
  return document.getElementById(field.getAttribute("aria-describedby") ?? "")?.textContent;
}

test("the password is checked here first: nothing goes out while a field is wrong", async () => {
  const { sent } = securityPage();

  await changePassword("", "short");

  expect(noteOf("Current password")).toBe("Required.");
  expect(noteOf("New password")).toBe("At least 8 characters.");
  expect(document.activeElement).toBe(screen.getByLabelText("Current password"));
  expect(sent).toEqual([]);
});

test("a wrong current password shows under it, with nothing above the form", async () => {
  securityPage({
    "POST /api/v0/me/change-password": () => problem(422, "identity.current_password_incorrect"),
  });

  await changePassword("Wr0ng-password", "new horse battery");

  await waitFor(() => expect(noteOf("Current password")).toBe("The current password is incorrect."));
  expect(document.activeElement).toBe(screen.getByLabelText("Current password"));
  expect(screen.queryByRole("alert")).toBeNull();
});

test("the server's problems with the new password show under it; a 429 says how long to wait", async () => {
  let answer = problem(422, "validation_failed", { errors: [{ field: "new_password", code: "common_password" }] });
  securityPage({ "POST /api/v0/me/change-password": () => answer });

  await changePassword("correct horse battery", "password1");
  await waitFor(() => expect(noteOf("New password")).toBe("Too common, or too close to the e-mail address."));

  answer = problem(429, "rate_limited", {}, { "Retry-After": "42" });
  await changePassword("correct horse battery", "new horse battery");
  expect((await screen.findByRole("alert")).textContent).toBe("Too many attempts. Try again in 42 s.");
});

test("a changed password empties the fields and says what became of the sessions", async () => {
  const { sent, app } = securityPage();

  await changePassword("correct horse battery", "new horse battery");

  expect((await screen.findByRole("status")).textContent).toContain("Password changed.");
  expect(screen.getByLabelText("Current password")).toHaveProperty("value", "");
  expect(screen.getByLabelText("New password")).toHaveProperty("value", "");
  expect(sent).toEqual([
    'POST /api/v0/me/change-password {"current_password":"correct horse battery","new_password":"new horse battery"}',
  ]);
  expect(app.session.tokens.state.status).toBe("signed-in");
});

test("a confirmed deactivation forgets the session here, without a logout, and goes to sign in", async () => {
  const user = userEvent.setup();
  const { app, sent, stored, router } = securityPage();

  await user.click(await screen.findByRole("button", { name: "Deactivate account" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Deactivate your account?" });
  await user.click(screen.getByRole("button", { name: "Deactivate" }));

  expect(await screen.findByRole("heading", { name: "Sign in" })).toBeTruthy();
  expect(router.state.location.search).toBe("?next=%2Fsettings%2Fsecurity");
  expect(app.session.tokens.state.status).toBe("signed-out");
  expect(stored).not.toHaveProperty(AUTH_KEY);
  expect(sent).toEqual(["POST /api/v0/me/deactivate"]);
  expect(dialog.isConnected).toBe(false);
});

test("a refused deactivation keeps the dialog, its reason and the session", async () => {
  const user = userEvent.setup();
  const { app, stored } = securityPage({ "POST /api/v0/me/deactivate": () => problem(503, "server_busy") });

  await user.click(await screen.findByRole("button", { name: "Deactivate account" }));
  await user.click(await screen.findByRole("button", { name: "Deactivate" }));

  const dialog = await screen.findByRole("alertdialog");
  expect((await screen.findByRole("alert")).textContent).toBe("The server is busy. Try again in a moment.");
  expect(dialog.contains(screen.getByRole("alert"))).toBe(true);
  expect(screen.getByRole("button", { name: "Deactivate" })).toHaveProperty("disabled", false);
  expect(app.session.tokens.state.status).toBe("signed-in");
  expect(stored).toHaveProperty(AUTH_KEY);
});

test.each([
  [
    "workspace.sole_admin",
    "You are the only admin of a workspace that has other members. Make another member an admin there first (workspace settings, Members), then deactivate.",
  ],
  [
    "notebook.sole_admin",
    "You are the only admin of notebooks that others are in. In each one's settings, make another member an admin, or delete it; then deactivate.",
  ],
])(
  "the only admin of a workspace or a notebook with other members is told what to do first, and stays signed in: %s",
  async (code, why) => {
    const user = userEvent.setup();
    const { app } = securityPage({ "POST /api/v0/me/deactivate": () => problem(409, code) });

    await user.click(await screen.findByRole("button", { name: "Deactivate account" }));
    await user.click(await screen.findByRole("button", { name: "Deactivate" }));

    const dialog = await screen.findByRole("alertdialog");
    expect((await within(dialog).findByRole("alert")).textContent).toBe(why);
    expect(app.session.tokens.state.status).toBe("signed-in");
  }
);

test("a deactivation goes out once, however often it is pressed; cancel sends nothing", async () => {
  const user = userEvent.setup();
  let release: ((response: Response) => void) | undefined;
  const { sent } = securityPage({
    "POST /api/v0/me/deactivate": () => new Promise<Response>((resolve) => (release = resolve)),
  });

  await user.click(await screen.findByRole("button", { name: "Deactivate account" }));
  await user.click(await screen.findByRole("button", { name: "Cancel" }));
  expect(screen.queryByRole("alertdialog")).toBeNull();
  expect(sent).toEqual([]);

  await user.click(screen.getByRole("button", { name: "Deactivate account" }));
  await user.click(await screen.findByRole("button", { name: "Deactivate" }));
  const sending = await screen.findByRole("button", { name: "Deactivating…" });
  expect(sending).toHaveProperty("disabled", true);
  await user.click(sending);
  // Nothing closes the dialog while it is out.
  await user.keyboard("{Escape}");
  expect(screen.getByRole("alertdialog")).toBeTruthy();

  release?.(new Response(null, { status: 204 }));
  expect(await screen.findByRole("heading", { name: "Sign in" })).toBeTruthy();
  expect(sent).toEqual(["POST /api/v0/me/deactivate"]);
});
