import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

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
  // With no edit that has changes not saved, it asked nothing (M12/P2/S1).
  expect(screen.queryByRole("alertdialog")).toBeNull();
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
