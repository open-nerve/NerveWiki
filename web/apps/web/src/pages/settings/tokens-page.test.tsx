import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import type { ApiToken } from "../../services/api-token.service";
import { json, problem, signedInApp, type Answer } from "../../test/fakes";
import { renderApp } from "../../test/render";

const secret = "nwk_pat_0123456789abcdefghijklmnopqrstuvwxyzABCDEFG";

const listed = (name: string, more: Partial<ApiToken> = {}): ApiToken => ({
  id: `id-${name}`,
  name,
  expires_at: null,
  last_used_at: null,
  created_at: "2026-09-01T08:00:00Z",
  ...more,
});

/**
 * A signed-in tab on the tokens page, over a server that keeps tokens:
 * create answers a creation (by default the new token, "id-new", with
 * secret), revoke a revocation (by default 204). What the tab changed is
 * in sent.
 */
function tokensPage(tokens: ApiToken[] = [], answers: { create?: Answer; revoke?: Answer } = {}) {
  const sent: string[] = [];
  let kept = tokens;
  const revokeRoutes = Object.fromEntries(
    [...tokens.map((t) => t.id), "id-new"].map((id): [string, Answer] => [
      `DELETE /api/v0/api-tokens/${id}`,
      (request) => {
        sent.push(`revoke ${id}`);
        if (answers.revoke) {
          return answers.revoke(request);
        }
        kept = kept.filter((t) => t.id !== id);
        return new Response(null, { status: 204 });
      },
    ])
  );
  const app = signedInApp({
    ...revokeRoutes,
    "GET /api/v0/me/api-tokens": () => json({ data: kept }),
    "POST /api/v0/me/api-tokens": async (request) => {
      const body = (await request.clone().json()) as { name: string; expires_at?: string };
      sent.push(`create ${body.name} ${body.expires_at === undefined ? "never" : "expires"}`);
      if (answers.create) {
        return answers.create(request);
      }
      const token = listed(body.name, { id: "id-new", expires_at: body.expires_at ?? null });
      kept = [token, ...kept];
      return json({ ...token, token: secret }, 201);
    },
  });
  renderApp("/settings/tokens", app);
  return { app, sent };
}

async function fillCreateForm(name: string, password: string, expiry?: string) {
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Create token" }));
  const dialog = await screen.findByRole("dialog", { name: "Create an access token" });
  if (name !== "") {
    await user.type(within(dialog).getByLabelText("Name"), name);
  }
  if (expiry !== undefined) {
    await user.selectOptions(within(dialog).getByLabelText("Expires"), expiry);
  }
  if (password !== "") {
    await user.type(within(dialog).getByLabelText("Current password"), password);
  }
  return { user, dialog };
}

/** Opens the creating dialog and expects its empty form, and the token nowhere. */
async function expectEmptyForm(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "Create token" }));
  const dialog = await screen.findByRole("dialog", { name: "Create an access token" });
  expect(within(dialog).getByLabelText("Name")).toHaveProperty("value", "");
  expect(within(dialog).getByLabelText("Current password")).toHaveProperty("value", "");
  expect(document.body.innerHTML).not.toContain(secret);
}

function noteOf(field: HTMLElement): string | null | undefined {
  return document.getElementById(field.getAttribute("aria-describedby") ?? "")?.textContent;
}

test("each token shows when it was created, when it expires and when it was last used", async () => {
  tokensPage([
    listed("deploy", { last_used_at: "2026-09-30T12:34:00Z", expires_at: "2099-01-01T00:00:00Z" }),
    listed("old", { expires_at: "2026-01-01T00:00:00Z" }),
    listed("agent"),
  ]);

  const rows = await screen.findAllByRole("listitem");
  expect(rows.map((row) => row.querySelector("p")?.textContent)).toEqual(["deploy", "oldExpired", "agent"]);
  expect(rows[0]?.textContent).toMatch(/Created Sep 1, 2026 · Expires Jan 1, 2099Last used Sep 30, 2026/);
  expect(rows[1]?.textContent).toContain("Expired Jan 1, 2026");
  expect(rows[2]?.textContent).toContain("Never expires");
  expect(rows[2]?.textContent).toContain("Never used");
});

test("without tokens the page says so", async () => {
  tokensPage();

  expect(await screen.findByText("You have no access tokens.")).toBeTruthy();
});

test("a new token needs a name and the password, checked here first", async () => {
  const { sent } = tokensPage();
  const { user, dialog } = await fillCreateForm("", "");

  await user.click(within(dialog).getByRole("button", { name: "Create" }));

  expect(noteOf(within(dialog).getByLabelText("Name"))).toBe("Required.");
  expect(noteOf(within(dialog).getByLabelText("Current password"))).toBe("Required.");
  expect(document.activeElement).toBe(within(dialog).getByLabelText("Name"));
  expect(sent).toEqual([]);
});

test("a wrong password shows under it, and nothing is created", async () => {
  tokensPage([], { create: () => problem(422, "identity.current_password_incorrect") });
  const { user, dialog } = await fillCreateForm("CI", "Wr0ng-password");

  await user.click(within(dialog).getByRole("button", { name: "Create" }));

  await waitFor(() =>
    expect(noteOf(within(dialog).getByLabelText("Current password"))).toBe("The current password is incorrect.")
  );
  expect(within(dialog).queryByRole("alert")).toBeNull();
  expect(screen.getByText("You have no access tokens.")).toBeTruthy();
});

test("a new token is shown once, in the dialog; after Done it is nowhere, and the list has it", async () => {
  const { sent } = tokensPage();
  const { user, dialog } = await fillCreateForm(" CI deploy ", "correct horse battery", "never");

  await user.click(within(dialog).getByRole("button", { name: "Create" }));

  const shown = await screen.findByRole("dialog", { name: "Your new token" });
  expect(within(shown).getByLabelText("Token")).toHaveProperty("value", secret);
  await user.click(within(shown).getByRole("button", { name: "Copy" }));
  expect(await navigator.clipboard.readText()).toBe(secret);
  expect(within(shown).getByRole("status").textContent).toBe("Copied.");
  // Only Done closes it: not Escape.
  await user.keyboard("{Escape}");
  expect(screen.getByRole("dialog", { name: "Your new token" })).toBeTruthy();

  await user.click(within(shown).getByRole("button", { name: "Done" }));

  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(document.body.innerHTML).not.toContain(secret);
  expect(screen.getByRole("listitem").textContent).toContain("CI deploy");
  expect(sent).toEqual(["create CI deploy never"]);
  // Opened again, the dialog starts from an empty form.
  await expectEmptyForm(user);
});

test("a token lasts 90 days unless another lifetime is chosen", async () => {
  let expiresAt = "";
  tokensPage([], {
    create: async (request) => {
      expiresAt = ((await request.json()) as { expires_at: string }).expires_at;
      return json({ ...listed("CI", { expires_at: expiresAt }), token: secret }, 201);
    },
  });
  const { user, dialog } = await fillCreateForm("CI", "correct horse battery");

  await user.click(within(dialog).getByRole("button", { name: "Create" }));

  await screen.findByRole("dialog", { name: "Your new token" });
  const days = (Date.parse(expiresAt) - Date.now()) / 86_400_000;
  expect(days).toBeGreaterThan(89.9);
  expect(days).toBeLessThanOrEqual(90);
});

// The dialog is cancelled while the creation is out: the token it answers
// is shown nowhere, and the list has the token, to revoke if unwanted.
test("a creation answered after the dialog was cancelled shows its token nowhere", async () => {
  let release: (() => void) | undefined;
  tokensPage([], {
    create: () =>
      new Promise<Response>((resolve) => {
        release = () => resolve(json({ ...listed("late", { id: "id-new" }), token: secret }, 201));
      }),
  });
  const { user, dialog } = await fillCreateForm("late", "correct horse battery");
  await user.click(within(dialog).getByRole("button", { name: "Create" }));
  await waitFor(() => expect(release).toBeDefined());

  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  release?.();

  expect(await screen.findByRole("listitem")).toBeTruthy();
  expect(screen.getByRole("listitem").textContent).toContain("late");
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.body.innerHTML).not.toContain(secret);
  await expectEmptyForm(user);
});

test("a revoked token leaves the list, and so does one another tab revoked already", async () => {
  const user = userEvent.setup();
  let gone = false;
  const { sent } = tokensPage([listed("deploy"), listed("agent")], {
    revoke: () => (gone ? problem(404, "identity.api_token_not_found") : new Response(null, { status: 204 })),
  });

  await user.click(await screen.findByRole("button", { name: "Revoke deploy" }));
  const confirm = await screen.findByRole("alertdialog", { name: "Revoke deploy?" });
  await user.click(within(confirm).getByRole("button", { name: "Revoke" }));
  await waitFor(() => expect(screen.getAllByRole("listitem")).toHaveLength(1));

  gone = true;
  await user.click(screen.getByRole("button", { name: "Revoke agent" }));
  await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Revoke" }));

  expect(await screen.findByText("You have no access tokens.")).toBeTruthy();
  expect(sent).toEqual(["revoke id-deploy", "revoke id-agent"]);
});

test("a refused revocation stays in the dialog, and the token in the list", async () => {
  const user = userEvent.setup();
  tokensPage([listed("deploy")], { revoke: () => problem(503, "server_busy") });

  await user.click(await screen.findByRole("button", { name: "Revoke deploy" }));
  const confirm = await screen.findByRole("alertdialog");
  await user.click(within(confirm).getByRole("button", { name: "Revoke" }));

  expect((await within(confirm).findByRole("alert")).textContent).toBe("The server is busy. Try again in a moment.");
  // The page behind the dialog is hidden from assistive technology while it is open.
  expect(screen.getAllByRole("listitem", { hidden: true })).toHaveLength(1);
});
