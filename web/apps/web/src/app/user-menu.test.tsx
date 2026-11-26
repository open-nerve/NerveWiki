import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";

import { FakeChannels } from "../events/testing/fake-channels";
import { FakePage } from "../events/testing/fake-page";
import { answerClosings } from "../stores/edit-closing";
import { eventServer, withEvents } from "../test/event-server";
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
  // With no edit that has changes not saved, it asked nothing (M12/P2/S1): the sign-in page above is the proof, had it
  // asked, the tab would still be signed in, the question open.
});

// M12/P2/S1, item 7: another tab of the login that has an edit with changes not saved says so; the tab asks first.

const elsewhereEn =
  "A page you are editing in another tab has changes that are not saved. Signing out saves what it can first; what it cannot save is lost.";

/**
 * elsewhere is the app at /acme with the events' deps of tab-0, another tab of login-0, tab-1, having an edit with
 * changes not saved on the same channels; sent are the logouts that went out.
 */
async function elsewhere() {
  const user = userEvent.setup();
  const sent: string[] = [];
  const channels = new FakeChannels();
  const events = eventServer();
  const app = withEvents(
    signedInApp({
      "GET /api/v0/events": events.answer,
      "POST /api/v0/auth/logout": (request) => {
        sent.push(new URL(request.url).pathname);
        return new Response(null, { status: 204 });
      },
    }),
    new FakePage(),
    channels
  );
  const other = answerClosings(
    channels.port("nwiki.edits"),
    { loginId: "login-0", tabId: "tab-1" },
    () => true,
    () => Promise.resolve(),
    () => true
  );
  renderApp("/acme", app);
  await user.click(await screen.findByRole("button", { name: "Ada" }));
  await user.click(await screen.findByRole("menuitem", { name: "Sign out" }));
  return { user, sent, app, other };
}

test("signing out while another tab of the login has an edit not saved asks first, saying it is in another tab; Stay keeps the session and gives the focus back to the menu's button", async () => {
  const { user, sent, app, other } = await elsewhere();

  const dialog = await screen.findByRole("alertdialog", { name: "Sign out anyway?", description: elsewhereEn });
  await user.click(within(dialog).getByRole("button", { name: "Stay" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("button", { name: "Ada" })));
  expect(sent).toEqual([]);
  expect(app.session.tokens.state.status).toBe("signed-in");
  other();
});

test("signing out while another tab of the login has an edit not saved asks first; Sign out signs out", async () => {
  const { user, sent, app, other } = await elsewhere();

  const dialog = await screen.findByRole("alertdialog", { name: "Sign out anyway?" });
  await user.click(within(dialog).getByRole("button", { name: "Sign out" }));

  expect(await screen.findByRole("heading", { name: "Sign in" })).toBeTruthy();
  expect(app.session.tokens.state.status).toBe("signed-out");
  expect(sent).toEqual(["/api/v0/auth/logout"]);
  other();
});

test("signing out while another tab has an edit not saved asks first, in Chinese too", async () => {
  const user = userEvent.setup();
  const channels = new FakeChannels();
  const events = eventServer();
  const app = withEvents(signedInApp({ "GET /api/v0/events": events.answer }), new FakePage(), channels);
  app.preferences.setLocale("zh-CN");
  const other = answerClosings(
    channels.port("nwiki.edits"),
    { loginId: "login-0", tabId: "tab-1" },
    () => true,
    () => Promise.resolve(),
    () => true
  );
  renderApp("/acme", app);
  await user.click(await screen.findByRole("button", { name: "Ada" }));
  await user.click(await screen.findByRole("menuitem", { name: "退出登录" }));

  const dialog = await screen.findByRole("alertdialog", {
    name: "仍要退出登录吗？",
    description: "另一个标签页里正在编辑的页面有还没有保存的修改。退出登录会先尽量保存，保存不了的会丢失。",
  });
  expect(within(dialog).getByRole("button", { name: "退出登录" })).toBeTruthy();
  expect(within(dialog).getByRole("button", { name: "留下" })).toBeTruthy();
  other();
});

test("the user menu says which version the server runs", async () => {
  const user = userEvent.setup();
  renderApp("/lab", signedInApp());

  await user.click(await screen.findByRole("button", { name: "Ada" }));

  expect(await screen.findByText("Nerve Wiki 1.2.3 (4f2a9c1)")).toBeTruthy();
});

// The account's row and its menu (M13/P2/S1, decision 770).

test("the account's button is named for its display name alone, its avatar's letter hidden", async () => {
  renderApp("/lab", signedInApp());

  // A name matches exactly: what e2e's accountMenu asks for.
  const button = await screen.findByRole("button", { name: "Ada" });
  const avatar = [...button.querySelectorAll("span")].find((span) => span.textContent === "A");
  expect(avatar).toBeDefined();
  expect(avatar?.closest('[aria-hidden="true"]')).toBe(avatar);
  expect(button.closest("[data-shell]")).not.toBeNull();
});

// A name cut short has its whole as its title, on its text: the button's name it is already (M13 closeout, review A's
// M13CA-M1). The column's row and the top right's.
test.each(["/lab", "/create-workspace"])("at %s the account's name has its whole as its title", async (path) => {
  renderApp(path, signedInApp());

  const button = await screen.findByRole("button", { name: "Ada" });
  expect([button.getAttribute("title"), button.querySelector("[title]")?.getAttribute("title")]).toEqual([null, "Ada"]);
  expect(button.querySelector("[title]")?.textContent).toBe("Ada");
});

test("the account's menu: Settings, Appearance, Sign out, then the version", async () => {
  const user = userEvent.setup();
  renderApp("/lab", signedInApp());

  await user.click(await screen.findByRole("button", { name: "Ada" }));
  const menu = await screen.findByRole("menu");
  await within(menu).findByText("Nerve Wiki 1.2.3 (4f2a9c1)");

  expect(
    [...menu.children].map((child) => (child.getAttribute("role") === "separator" ? "—" : child.textContent))
  ).toEqual(["Settings", "Appearance", "—", "Sign out", "—", "Nerve Wiki 1.2.3 (4f2a9c1)"]);
  expect(within(menu).getByRole("menuitem", { name: "Appearance" }).getAttribute("aria-haspopup")).toBe("menu");
});

test("the column's account opens its menu above it, from its start; the top right's below it, from its end", async () => {
  const user = userEvent.setup();
  const { unmount } = renderApp("/lab", signedInApp());
  await user.click(await screen.findByRole("button", { name: "Ada" }));
  // Radix's placement, as it asks for it: jsdom lays nothing out to flip it by.
  const row = await screen.findByRole("menu");
  expect([row.getAttribute("data-side"), row.getAttribute("data-align")]).toEqual(["top", "start"]);
  unmount();

  // A page without a left column: the settings have one (M13/P3/S1, decision 811).
  renderApp("/create-workspace", signedInApp());
  await user.click(await screen.findByRole("button", { name: "Ada" }));
  const corner = await screen.findByRole("menu");
  expect([corner.getAttribute("data-side"), corner.getAttribute("data-align")]).toEqual(["bottom", "end"]);
  // No wider than 16rem, the version line wrapping, nor than the room Radix measures: at 375 px that line would push
  // it off the screen, at 1440 px stretch it to 444 px (review B, S1B-N3). jsdom lays nothing out, so the cap is what
  // it can see; the S1 screenshots show the rest.
  expect(corner.className).toContain("max-w-[min(16rem,var(--radix-dropdown-menu-content-available-width))]");
});

// The column's and the top right's (review C, S1C-N2), the latter on a page without a column (M13/P3/S1).
test.each(["/lab", "/create-workspace"])(
  "below md the account's menu at %s has the theme and the language in it, no submenu, a choice keeping it open",
  async (path) => {
    // A submenu has no room beside a menu as wide as a phone's column (M13/P2/S1, decision 801).
    vi.stubGlobal("matchMedia", (query: string) => ({
      matches: query === "(width < 48rem)",
      addEventListener: () => {},
      removeEventListener: () => {},
    }));
    const user = userEvent.setup();
    const { app } = renderApp(path, signedInApp());

    await user.click(await screen.findByRole("button", { name: "Ada" }));
    const menu = await screen.findByRole("menu");
    await within(menu).findByText("Nerve Wiki 1.2.3 (4f2a9c1)");
    expect(within(menu).queryByRole("menuitem", { name: "Appearance" })).toBeNull();
    expect(
      [...menu.children].map((child) =>
        child.getAttribute("role") === "separator" ? "—" : (child.getAttribute("aria-label") ?? child.textContent)
      )
    ).toEqual([
      "Settings",
      "—",
      "Theme",
      "Theme",
      "—",
      "Language",
      "Language",
      "—",
      "Sign out",
      "—",
      "Nerve Wiki 1.2.3 (4f2a9c1)",
    ]);
    expect(
      within(within(menu).getByRole("group", { name: "Theme" }))
        .getAllByRole("menuitemradio")
        .map((item) => item.textContent)
    ).toEqual(["System", "Light", "Dark"]);

    await user.click(within(menu).getByRole("menuitemradio", { name: "Dark" }));
    expect(app.preferences.theme).toBe("dark");
    expect(menu.isConnected).toBe(true);
    expect(within(menu).getByRole("menuitemradio", { name: "Dark" }).getAttribute("aria-checked")).toBe("true");
  }
);

test("a theme and a language chosen in Appearance apply at once, its menu staying open", async () => {
  const user = userEvent.setup();
  const { app } = renderApp("/lab", signedInApp());

  await user.click(await screen.findByRole("button", { name: "Ada" }));
  await user.click(await screen.findByRole("menuitem", { name: "Appearance" }));
  // Clicked, not moved to: jsdom lays nothing out, and Radix takes the pointer leaving Appearance for a pointer going
  // elsewhere, which closes the submenu, where a browser sees it go towards it.
  fireEvent.click(await screen.findByRole("menuitemradio", { name: "Dark" }));

  expect(app.preferences.theme).toBe("dark");
  const light = screen.getByRole("menuitemradio", { name: "Light" });
  expect(screen.getByRole("menuitemradio", { name: "Dark" }).getAttribute("aria-checked")).toBe("true");
  fireEvent.click(screen.getByRole("menuitemradio", { name: "简体中文" }));
  expect(app.preferences.locale).toBe("zh-CN");
  // The same submenu, in Chinese now.
  expect(light.isConnected).toBe(true);
  expect(light.textContent).toBe("浅色");
});

test("the account's menu by the keyboard: Enter opens it, Down to Appearance, Right opens it at its first choice, Escape closes it all, the focus back on the button", async () => {
  const user = userEvent.setup();
  renderApp("/lab", signedInApp());
  const button = await screen.findByRole("button", { name: "Ada" });
  button.focus();

  await user.keyboard("{Enter}");
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "Settings" })));
  await user.keyboard("{ArrowDown}");
  expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "Appearance" }));
  await user.keyboard("{ArrowRight}");
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("menuitemradio", { name: "System" })));
  await user.keyboard("{Escape}");

  await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());
  await waitFor(() => expect(document.activeElement).toBe(button));
});

test("in Chinese, the account's menu has 外观", async () => {
  const user = userEvent.setup();
  const app = signedInApp();
  app.preferences.setLocale("zh-CN");
  renderApp("/lab", app);

  await user.click(await screen.findByRole("button", { name: "Ada" }));

  expect(await screen.findByRole("menuitem", { name: "外观" })).toBeTruthy();
  expect(screen.getByRole("menuitem", { name: "设置" })).toBeTruthy();
});
