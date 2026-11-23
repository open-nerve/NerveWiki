import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";

import type { User } from "../../services/account.service";
import { json, problem, signedInApp, userJSON, type Answer } from "../../test/fakes";
import { renderApp } from "../../test/render";

/** A signed-in tab whose account the fake server keeps; each change it was sent is in sent. */
function profileServer(patch?: Answer) {
  const sent: unknown[] = [];
  let me: User = userJSON;
  const app = signedInApp({
    "GET /api/v0/me": () => json(me),
    "PATCH /api/v0/me": async (request) => {
      const changes = (await request.clone().json()) as Partial<User>;
      sent.push(changes);
      if (patch) {
        return patch(request);
      }
      me = { ...me, ...changes };
      return json(me);
    },
  });
  return { app, sent };
}

test("the settings start at the profile, its name the title, its link current in the directory", async () => {
  const { router } = renderApp("/settings", signedInApp());

  expect(await screen.findByRole("heading", { level: 1, name: "Profile" })).toBeTruthy();
  expect(router.state.location.pathname).toBe("/settings/profile");
  const nav = screen.getByRole("navigation", { name: "Settings" });
  expect(nav.closest("[data-shell]")).not.toBeNull();
  expect(nav.querySelector('[aria-current="page"]')?.textContent).toBe("Profile");
});

test("an account with a step left does its onboarding before the settings", async () => {
  const { router } = renderApp(
    "/settings/profile",
    signedInApp({ "GET /api/v0/me": () => json({ ...userJSON, onboarding_steps: [] }) })
  );

  expect(await screen.findByText("Step 1 of 3")).toBeTruthy();
  expect(router.state.location.search).toBe("?next=%2Fsettings%2Fprofile");
});

test("the account's menu leads to the settings, arrived at: the profile's title takes the focus; chosen there again in the column's foot, the focus goes back to its button", async () => {
  const user = userEvent.setup();
  const { router } = renderApp("/lab", signedInApp());
  await screen.findByRole("heading", { name: "Lab" });

  // The left column's, which leaves with the workspace's shell (M13/P2/S1, decision 771).
  await user.click(screen.getByRole("button", { name: "Ada" }));
  await user.click(await screen.findByRole("menuitem", { name: "Settings" }));

  const title = await screen.findByRole("heading", { level: 1, name: "Profile" });
  expect(router.state.location.pathname).toBe("/settings/profile");
  await waitFor(() => expect(document.activeElement).toBe(title));

  // The same address: no arrival (M13/P3/S1, decision 816), the menu gives the focus back to its button, in the
  // settings' column (review A, P2A1-N1).
  const account = screen.getByRole("button", { name: "Ada" });
  expect(account.closest("[data-shell]")).not.toBeNull();
  await user.click(account);
  await user.click(await screen.findByRole("menuitem", { name: "Settings" }));
  await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());
  await waitFor(() => expect(document.activeElement).toBe(account));
  expect(router.state.location.pathname).toBe("/settings/profile");
  expect(router.state.location.state).toBeNull();
});

/** The left column's root, [data-shell]: the drawer below md. */
const shell = () => document.querySelector("[data-shell]") as HTMLElement;

// jsdom has no inert: an element under [inert] takes no focus here, as in the browsers, or Radix's setTimeout(0)
// would give it back to the account's button in the drawer closed (review A's R1-M2, C's R1-I1). vi.spyOn: the
// configuration's restoreMocks puts focus back after the cell.
test("at a phone's width, Settings chosen again on the profile from the drawer's account menu: the drawer closes, the focus on its menu, no arrival", async () => {
  vi.stubGlobal("matchMedia", (query: string) => ({
    matches: query === "(width < 48rem)",
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
  }));
  const focus = HTMLElement.prototype.focus;
  vi.spyOn(HTMLElement.prototype, "focus").mockImplementation(function (this: HTMLElement, options?: FocusOptions) {
    if (this.closest("[inert]") === null) {
      focus.call(this, options);
    }
  });
  const user = userEvent.setup();
  const { router } = renderApp("/settings/profile", signedInApp());
  await screen.findByRole("heading", { level: 1, name: "Profile" });
  const menuButton = screen.getByRole("button", { name: "Navigation" });

  await user.click(menuButton);
  await screen.findByRole("dialog", { name: "Navigation" });
  await user.click(within(shell()).getByRole("button", { name: "Ada" }));
  await user.click(await screen.findByRole("menuitem", { name: "Settings" }));

  await waitFor(() => expect(shell().hasAttribute("inert")).toBe(true));
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  expect(document.activeElement).toBe(menuButton);
  expect(router.state.location.pathname).toBe("/settings/profile");
  expect(router.state.location.state).toBeNull();
});

test("chosen at the top right of the onboarding, where the settings send it back, Settings leaves the focus on the account's button", async () => {
  const user = userEvent.setup();
  const { router } = renderApp(
    "/onboarding",
    signedInApp({ "GET /api/v0/me": () => json({ ...userJSON, onboarding_steps: [] }) })
  );
  await screen.findByText("Step 1 of 3");

  // No title of the settings takes it: the menu gives it back to its button, still there (review A, S1A-N2).
  const corner = screen.getByRole("button", { name: "Ada" });
  await user.click(corner);
  await user.click(await screen.findByRole("menuitem", { name: "Settings" }));

  // The bounce itself, not the onboarding the cell began on (review C, S1C1-N2).
  await waitFor(() => expect(router.state.location.search).toBe("?next=%2Fsettings%2Fprofile"));
  expect(router.state.location.pathname).toBe("/onboarding");
  await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());
  await waitFor(() => expect(document.activeElement).toBe(corner));
});

test("Settings chosen in the column's foot on another page of theirs: the profile's title takes the focus; its menu opened again gives Escape's focus back to its button", async () => {
  const user = userEvent.setup();
  renderApp("/settings/security", signedInApp());
  await screen.findByRole("heading", { level: 1, name: "Security" });
  const account = screen.getByRole("button", { name: "Ada" });
  expect(account.closest("[data-shell]")).not.toBeNull();
  await user.click(account);
  await user.click(await screen.findByRole("menuitem", { name: "Settings" }));
  const title = await screen.findByRole("heading", { level: 1, name: "Profile" });
  await waitFor(() => expect(document.activeElement).toBe(title));

  // The same column, kept across the settings: what held the focus from its button then holds it no more (review
  // A, S1A-N4, r01).
  await user.click(account);
  await screen.findByRole("menu");
  await user.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());
  await waitFor(() => expect(document.activeElement).toBe(account));
});

test("a new display name is saved, and the user menu shows it", async () => {
  const user = userEvent.setup();
  const { app, sent } = profileServer();
  renderApp("/settings/profile", app);
  const name = await screen.findByLabelText("Display name");
  expect(screen.getByText("ada@example.com")).toBeTruthy();

  await user.clear(name);
  await user.type(name, " Ada Lovelace ");
  await user.click(screen.getByRole("button", { name: "Save" }));

  expect((await screen.findByRole("status")).textContent).toBe("Saved.");
  expect(screen.getByRole("button", { name: "Ada Lovelace" })).toBeTruthy();
  expect(sent).toEqual([{ display_name: "Ada Lovelace" }]);
  // Editing again takes the saved state away.
  await user.type(name, "!");
  expect(screen.getByRole("status").textContent).toBe("");
});

// A save answered after another edit saved the name sent, not the one shown: it is not marked saved, and the
// next save sends what the field holds (R2 of the M1 adversarial review).
test("a name edited while its save is out is not marked saved", async () => {
  const user = userEvent.setup();
  let release: (() => void) | undefined;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  let first = true;
  const { app, sent } = profileServer(async (request) => {
    if (first) {
      first = false;
      await held;
    }
    return json({ ...userJSON, ...((await request.json()) as Partial<User>) });
  });
  renderApp("/settings/profile", app);
  const name = await screen.findByLabelText("Display name");
  const save = screen.getByRole("button", { name: "Save" });

  await user.clear(name);
  await user.type(name, "First name");
  await user.click(save);
  await waitFor(() => expect(sent).toHaveLength(1));
  await user.clear(name);
  await user.type(name, "Second name");
  release?.();
  await waitFor(() => expect(save).toHaveProperty("disabled", false));

  expect(screen.getByRole("status").textContent).toBe("");
  expect(name).toHaveProperty("value", "Second name");
  await user.click(save);
  expect((await screen.findByRole("status")).textContent).toBe("Saved.");
  expect(sent).toEqual([{ display_name: "First name" }, { display_name: "Second name" }]);
});

test("a name left as it was is saved without a request", async () => {
  const user = userEvent.setup();
  const { app, sent } = profileServer();
  renderApp("/settings/profile", app);

  await user.click(await screen.findByRole("button", { name: "Save" }));

  expect((await screen.findByRole("status")).textContent).toBe("Saved.");
  expect(sent).toEqual([]);
});

test("an empty or refused name is marked under the field, which gets the focus", async () => {
  const user = userEvent.setup();
  const { app, sent } = profileServer(() =>
    problem(422, "validation_failed", { errors: [{ field: "display_name", code: "too_long" }] })
  );
  renderApp("/settings/profile", app);
  const name = await screen.findByLabelText("Display name");

  await user.clear(name);
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(await screen.findByText("Required.")).toBeTruthy();
  expect(document.activeElement).toBe(name);
  expect(sent).toEqual([]);

  await user.type(name, "A".repeat(101));
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(await screen.findByText("At most 100 characters.")).toBeTruthy();
  expect(document.activeElement).toBe(name);
  expect(screen.getByRole("status").textContent).toBe("");
  expect(screen.queryByRole("alert")).toBeNull();
});

test("the preferences are the account menu's: chosen here, they apply at once", async () => {
  const user = userEvent.setup();
  const { app } = profileServer();
  renderApp("/settings/profile", app);

  await user.click(await screen.findByRole("radio", { name: "Dark" }));
  expect(app.preferences.theme).toBe("dark");
  expect(document.documentElement.classList.contains("dark")).toBe(true);

  await user.click(screen.getByRole("radio", { name: "简体中文" }));
  expect(app.preferences.locale).toBe("zh-CN");
  expect(await screen.findByRole("heading", { level: 1, name: "个人资料" })).toBeTruthy();
  expect(screen.getByRole("radio", { name: "深色" })).toHaveProperty("checked", true);

  // The account menu's Appearance shows the same choice (M13/P2/S1).
  await user.click(screen.getByRole("button", { name: "Ada" }));
  await user.click(await screen.findByRole("menuitem", { name: "外观" }));
  await waitFor(() =>
    expect(screen.getByRole("menuitemradio", { name: "深色" }).getAttribute("aria-checked")).toBe("true")
  );
});
