import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { json, problem, type Answer } from "../../test/fakes";
import { marked } from "../../test/page-editor";
import { section } from "../../test/page-panel";
import { bob, install, pagePath, pageServer, versionJSON } from "../../test/page-server";
import { renderApp } from "../../test/render";

// A version restored (M8 design 4.4; P1 design 4.5): each answer.

/** versionPath is the address of the version of the page id that holds revision. */
const versionPath = (id: string, revision: number) => `${pagePath(id)}/versions/${revision.toString()}`;

/** installed is Install over pageServer with three versions, the page now as the newest; answers answer instead of it. */
function installed(answers: Record<string, Answer> = {}) {
  const server = pageServer({ answers });
  server.withVersions(install.id, [
    versionJSON(1, null, "Install\n", { by: bob, at: "2026-10-01T09:00:00Z" }),
    versionJSON(4, 1, "Install\nthree\n", { at: "2026-10-02T09:00:00Z" }),
    versionJSON(6, 4, "Install now\n", { at: "2026-10-03T09:00:00Z" }),
  ]);
  server.views.set(install.id, { html: "<p>Install now</p>", revision: 6, assets_expire_at: null });
  return server;
}

/**
 * restoring is Install with three versions, its version 4 shown and its
 * restore's dialog open, what it compares read; answers answer instead of
 * the server.
 */
async function restoring(answers: Record<string, Answer> = {}) {
  const user = userEvent.setup();
  const server = installed(answers);
  const { router } = renderApp(versionPath(install.id, 4), server.app);
  const restore = await screen.findByRole("button", { name: "Restore this version" });
  await user.click(restore);
  const dialog = await screen.findByRole("alertdialog", { name: "Restore version 4?" });
  await within(dialog).findByRole("textbox", { name: "Changes from the page now to version 4" });
  return { user, server, router, restore, dialog };
}

/** restores is what of sent restored a version. */
const restores = (sent: readonly string[]) => sent.filter((each) => each.startsWith("RESTORE"));

test("the dialog shows what the restore changes of the page as it is now; restored, the tab goes to the page, its title focused, which says so, its view and versions read again", async () => {
  const user = userEvent.setup();
  const server = installed();
  const { router } = renderApp(pagePath(install.id), server.app);
  // The page's view, read as it was, which the restore changes.
  expect((await screen.findByRole("article", { name: "Install" })).textContent).toBe("Install now");
  const version = await waitFor(() => {
    const link = within(section("Versions"))
      .getAllByRole("link")
      .find((each) => each.getAttribute("href")?.endsWith("/versions/4"));
    expect(link).toBeTruthy();
    return link as HTMLElement;
  });
  await user.click(version);
  await user.click(await screen.findByRole("button", { name: "Restore this version" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Restore version 4?" });
  const diff = await within(dialog).findByRole("textbox", { name: "Changes from the page now to version 4" });
  expect(
    within(dialog).getByText(
      "The page's content becomes this version's, as a new version: the versions before it stay. Below, what the restore changes."
    )
  ).toBeTruthy();
  expect(dialog.textContent).toContain("The page now → Version 4");
  // From the page now to the version: what the restore changes.
  expect(marked(diff)).toEqual({ removed: ["Install now"], added: ["Install", "three"] });
  expect(server.sent).toContain("GET content Install");

  const before = server.sent.length;
  await user.click(within(dialog).getByRole("button", { name: "Restore" }));

  const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
  // The view cached before the restore is not shown, not even at first.
  expect(screen.queryByRole("article", { name: "Install" })?.textContent).not.toBe("Install now");
  expect(router.state.location.pathname).toBe(pagePath(install.id));
  await waitFor(() => expect(document.activeElement).toBe(heading));
  const status = screen.getByText("Restored version 4.");
  expect(status.tagName).toBe("OUTPUT");
  // Heard with the title, which has the focus.
  expect(heading.getAttribute("aria-describedby")).toBe(status.id);
  expect(restores(server.sent)).toEqual(["RESTORE Install 4 on 6"]);
  expect((await screen.findByRole("article", { name: "Install" })).textContent).toBe("Install\nthree");
  await waitFor(() => expect(within(section("Versions")).getAllByRole("listitem")).toHaveLength(4));
  expect(within(section("Versions")).getAllByRole("listitem")[0]?.textContent).toMatch(/CurrentAda · Restore$/);
  expect(server.sent.slice(before)).toEqual(expect.arrayContaining(["GET view Install", "GET versions Install"]));
});

test("a version the page holds already writes nothing: the dialog closes, the version's page saying so, the focus back on Restore", async () => {
  const { user, server, router, restore, dialog } = await restoring();
  server.contents.set(install.id, { content: "Install\nthree\n", revision: 6 });

  await user.click(within(dialog).getByRole("button", { name: "Restore" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(screen.getByText("The page is the same as this version: nothing changed.").tagName).toBe("OUTPUT");
  await waitFor(() => expect(document.activeElement).toBe(restore));
  expect(router.state.location.pathname).toBe(versionPath(install.id, 4));
  expect(restores(server.sent)).toEqual(["RESTORE Install 4 on 6"]);
});

test("a page written since it was read has its content read again, and the comparison with it; the dialog stays, saying so, sending nothing again until confirmed", async () => {
  const { user, server, dialog } = await restoring();
  server.contents.set(install.id, { content: "Install later\n", revision: 7 });

  await user.click(within(dialog).getByRole("button", { name: "Restore" }));

  expect(
    await within(dialog).findByText("The page changed meanwhile: the changes below are from the page as it is now.")
  ).toBeTruthy();
  await waitFor(() =>
    expect(
      within(dialog).getByRole("textbox", { name: "Changes from the page now to version 4" }).parentElement?.textContent
    ).toContain("Install later")
  );
  expect(server.sent.filter((each) => each === "GET content Install")).toHaveLength(2);
  expect(restores(server.sent)).toEqual(["RESTORE Install 4 on 6"]);

  await user.click(within(dialog).getByRole("button", { name: "Restore" }));
  await screen.findByRole("heading", { level: 1, name: "Install" });
  expect(restores(server.sent)).toEqual(["RESTORE Install 4 on 6", "RESTORE Install 4 on 7"]);
});

test("a page someone edits says who in the dialog, which stays; the page's lock is read again", async () => {
  const { user, server, dialog } = await restoring();
  server.hold(install.id, bob);

  await user.click(within(dialog).getByRole("button", { name: "Restore" }));

  expect((await within(dialog).findByRole("alert")).textContent).toBe("Bob is editing “Install”.");
  expect(screen.getByRole("alertdialog")).toBe(dialog);
  expect(await screen.findByText("Bob is editing this page.")).toBeTruthy();
});

test("a page gone closes the dialog, the version's page saying so", async () => {
  const { user, server, dialog } = await restoring();
  server.nodes = server.nodes.filter((node) => node.id !== install.id);

  await user.click(within(dialog).getByRole("button", { name: "Restore" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(
    screen.getByText("The page is no longer there: deleted, or out of your reach. Nothing was restored.").tagName
  ).toBe("OUTPUT");
});

test("a version no longer there closes the dialog, the version's page saying why", async () => {
  const { user, dialog } = await restoring({
    "POST /api/v0/pages/*/versions/*/restore": () => problem(404, "page.version_not_found"),
  });

  await user.click(within(dialog).getByRole("button", { name: "Restore" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(screen.getByText("This page has no such version.").tagName).toBe("OUTPUT");
});

test("the page now that could not be read says why in the dialog, and is read on Try again", async () => {
  const user = userEvent.setup();
  let fail = true;
  const server = installed({
    "GET /api/v0/pages/*/content": () =>
      fail ? problem(500, "internal") : json({ ...server.contents.get(install.id), content_hash: "0".repeat(64) }),
  });
  renderApp(versionPath(install.id, 4), server.app);
  await user.click(await screen.findByRole("button", { name: "Restore this version" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Restore version 4?" });

  const retry = await within(dialog).findByRole("button", { name: "Try again" });
  expect(within(dialog).getByRole("button", { name: "Restore" }).hasAttribute("disabled")).toBe(true);
  fail = false;
  await user.click(retry);
  expect(await within(dialog).findByRole("textbox", { name: "Changes from the page now to version 4" })).toBeTruthy();
  expect(within(dialog).getByRole("button", { name: "Restore" }).hasAttribute("disabled")).toBe(false);
});

test("another refusal stays in the dialog with its reason", async () => {
  const { user, dialog } = await restoring({
    "POST /api/v0/pages/*/versions/*/restore": () => problem(403, "forbidden"),
  });

  await user.click(within(dialog).getByRole("button", { name: "Restore" }));

  expect((await within(dialog).findByRole("alert")).textContent).toBe("You do not have permission to do this.");
  expect(screen.getByRole("alertdialog")).toBe(dialog);
});

test("the restore goes out once, however often Restore is pressed, and nothing closes the dialog meanwhile", async () => {
  const held: (() => void)[] = [];
  const { user, server, dialog } = await restoring({
    "POST /api/v0/pages/*/versions/*/restore": async () => {
      server.sent.push("RESTORE held");
      await new Promise<void>((resolve) => held.push(resolve));
      return problem(409, "page.revision_mismatch");
    },
  });
  const confirm = within(dialog).getByRole("button", { name: "Restore" });

  await user.click(confirm);
  await user.dblClick(within(dialog).getByRole("button", { name: "Restoring…" }));
  await user.keyboard("{Escape}");
  expect(screen.getByRole("alertdialog")).toBe(dialog);
  expect(restores(server.sent)).toEqual(["RESTORE held"]);

  act(() => held[0]?.());
  expect(await within(dialog).findByRole("button", { name: "Restore" })).toBeTruthy();
  expect(restores(server.sent)).toEqual(["RESTORE held"]);
});
