import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, onTestFinished, test, vi } from "vitest";

import { lineDiff, type LineDiffResult } from "../../lib/line-diff";
import { json, problem, type Answer } from "../../test/fakes";
import { marked } from "../../test/page-editor";
import { section } from "../../test/page-panel";
import { bob, install, pagePath, pageServer, versionJSON } from "../../test/page-server";
import { renderApp } from "../../test/render";

// A version restored (M8 design 4.4; P1 design 4.5): each answer.

// The worker's comparisons, of a page too large for the page's thread, answer when a test says.
const worker = vi.hoisted(() => ({ asked: [] as { answer: () => void; fail: () => void }[] }));
vi.mock("../../lib/line-diff-client", async (original) => ({
  ...(await original<typeof import("../../lib/line-diff-client")>()),
  compareInWorker: (a: string, b: string) =>
    new Promise<LineDiffResult>((resolve, reject) =>
      worker.asked.push({ answer: () => resolve(lineDiff(a, b)), fail: () => reject(new Error("failed")) })
    ),
}));

/** answered answers the comparison asked nth, or fails it. */
async function answered(nth: number, how: "answer" | "fail" = "answer"): Promise<void> {
  await waitFor(() => expect(worker.asked.length).toBeGreaterThan(nth));
  await act(async () => {
    worker.asked[nth]?.[how]();
    await Promise.resolve();
  });
}

/** largeShown is Install of over a thousand lines, its version 4 shown, no comparison asked yet. */
function largeShown() {
  worker.asked.length = 0;
  const user = userEvent.setup();
  const lines = Array.from({ length: 1200 }, (_, i) => `line ${i.toString()}\n`).join("");
  const server = pageServer();
  server.withVersions(install.id, [
    versionJSON(1, null, "Install\n", { by: bob, at: "2026-10-01T09:00:00Z" }),
    versionJSON(4, 1, `${lines}Install\nthree\n`, { at: "2026-10-02T09:00:00Z" }),
    versionJSON(6, 4, `${lines}Install now\n`, { at: "2026-10-03T09:00:00Z" }),
  ]);
  renderApp(versionPath(install.id, 4), server.app);
  return { user, server, lines };
}

/** large is Install of over a thousand lines, its version 4 shown and its restore's dialog open. */
async function large() {
  const { user, server, lines } = largeShown();
  await user.click(await screen.findByRole("button", { name: "Restore this version" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Restore version 4?" });
  return { user, server, lines, dialog, confirm: within(dialog).getByRole("button", { name: "Restore" }) };
}

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
  const { app, router } = renderApp(versionPath(install.id, 4), server.app);
  const restore = await screen.findByRole("button", { name: "Restore this version" });
  await user.click(restore);
  const dialog = await screen.findByRole("alertdialog", { name: "Restore version 4?" });
  await within(dialog).findByRole("textbox", { name: "Changes from the page now to version 4" });
  return { user, app, server, router, restore, dialog };
}

/** same is Install, its content now, its version 4 version, shown with its restore's dialog open: texts the same once split. */
async function same(now: string, version: string) {
  worker.asked.length = 0;
  const user = userEvent.setup();
  const server = pageServer();
  server.withVersions(install.id, [
    versionJSON(1, null, "Install\n"),
    versionJSON(4, 1, version),
    versionJSON(6, 4, now),
  ]);
  renderApp(versionPath(install.id, 4), server.app);
  await user.click(await screen.findByRole("button", { name: "Restore this version" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Restore version 4?" });
  return { dialog, confirm: within(dialog).getByRole("button", { name: "Restore" }) };
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
  // The banner alone: the page was read again.
  expect(within(dialog).getAllByRole("alert")).toHaveLength(1);
  expect(server.sent.filter((each) => each === "GET content Install")).toHaveLength(2);
  expect(restores(server.sent)).toEqual(["RESTORE Install 4 on 6"]);

  await user.click(within(dialog).getByRole("button", { name: "Restore" }));
  await screen.findByRole("heading", { level: 1, name: "Install" });
  expect(restores(server.sent)).toEqual(["RESTORE Install 4 on 6", "RESTORE Install 4 on 7"]);
});

test("the banner of a page written meanwhile goes as Restore is pressed again: a refusal after it is said alone", async () => {
  const { user, server, dialog } = await restoring();
  server.contents.set(install.id, { content: "Install later\n", revision: 7 });
  await user.click(within(dialog).getByRole("button", { name: "Restore" }));
  await within(dialog).findByText("The page changed meanwhile: the changes below are from the page as it is now.");

  server.hold(install.id, bob);
  await user.click(within(dialog).getByRole("button", { name: "Restore" }));

  expect(await within(dialog).findByText("Bob is editing “Install”.")).toBeTruthy();
  expect(within(dialog).queryByText(/The page changed meanwhile/)).toBeNull();
});

// M8/P1 fix check 1, N1: what a restore sends goes with what shows.
test("on a large page, Restore waits for the comparison with the page now: the one before it shows meanwhile, saying so, nothing sent on the newer revision until its own shows", async () => {
  const { user, server, lines, dialog, confirm } = await large();
  await waitFor(() => expect(worker.asked).toHaveLength(1));
  expect(confirm.hasAttribute("disabled")).toBe(true);
  await answered(0);
  const diff = () => within(dialog).getByRole("textbox", { name: "Changes from the page now to version 4" });
  await waitFor(() => expect(marked(diff()).removed).toEqual(["Install now"]));
  expect(confirm.hasAttribute("disabled")).toBe(false);

  // Someone writes the page: a paragraph the restore would take away.
  server.contents.set(install.id, { content: `${lines}Install now\nA new paragraph\n`, revision: 7 });
  await user.click(confirm);
  await waitFor(() => expect(worker.asked).toHaveLength(2));
  // The comparison before shows, saying a newer one runs; no banner speaks of it, and Restore waits.
  expect(marked(diff()).removed).toEqual(["Install now"]);
  expect(within(dialog).getByText("Comparing…")).toBeTruthy();
  expect(within(dialog).queryByText(/The page changed meanwhile/)).toBeNull();
  expect(confirm.hasAttribute("disabled")).toBe(true);
  await user.click(confirm);
  expect(restores(server.sent)).toEqual(["RESTORE Install 4 on 6"]);

  await answered(1);
  await waitFor(() => expect(marked(diff()).removed).toEqual(["Install now", "A new paragraph"]));
  expect(
    within(dialog).getByText("The page changed meanwhile: the changes below are from the page as it is now.")
  ).toBeTruthy();
  expect(within(dialog).queryByText("Comparing…")).toBeNull();
  await user.click(confirm);
  await screen.findByRole("heading", { level: 1, name: "Install" });
  expect(restores(server.sent)).toEqual(["RESTORE Install 4 on 6", "RESTORE Install 4 on 7"]);
});

test("a comparison with the page now that failed restores nothing until one shows, on Try again; failed again, it says a reload helps", async () => {
  const { user, dialog, confirm } = await large();
  await answered(0, "fail");
  expect(await within(dialog).findByText("The comparison failed.")).toBeTruthy();
  expect(confirm.hasAttribute("disabled")).toBe(true);

  // M8/P1 fix check 2, R2-2: a worker of an older build than the server's fails each time; a reload replaces it.
  const retry = within(dialog).getByRole("button", { name: "Try again" });
  await user.click(retry);
  await answered(1, "fail");
  expect(
    await within(dialog).findByText("The comparison failed again. Reloading the page usually fixes this.")
  ).toBeTruthy();
  expect(confirm.hasAttribute("disabled")).toBe(true);
  // M8/P1 fix check 3, R3-2: the focus stayed on Try again; Reload follows it.
  expect(document.activeElement).toBe(retry);
  await user.tab();
  expect(document.activeElement).toBe(within(dialog).getByRole("button", { name: "Reload" }));

  await user.click(retry);
  await answered(2);
  const diff = await within(dialog).findByRole("textbox", { name: "Changes from the page now to version 4" });
  expect(confirm.hasAttribute("disabled")).toBe(false);
  // Gone with the focus, the failure gave it to the comparison: a group named as it is (M8/P1 fix check 4, R4-2).
  expect(within(dialog).getAllByRole("group", { name: "Changes from the page now to version 4" })).toContain(
    document.activeElement
  );
  expect(document.activeElement?.contains(diff)).toBe(true);
});

// M8/P1 fix check 3, R3-2: the version's page, its tab comparing it with the page now, the same.
test("on the tab comparing the version with the page now, Try again failed again keeps the focus, Reload following it", async () => {
  const { user } = largeShown();
  await user.click(await screen.findByRole("tab", { name: "Compare with the current" }));
  await answered(0, "fail");
  const panel = screen.getByRole("tabpanel");
  const retry = await within(panel).findByRole("button", { name: "Try again" });
  await user.click(retry);
  await answered(1, "fail");
  await within(panel).findByRole("button", { name: "Reload" });
  expect(document.activeElement).toBe(retry);
  await user.tab();
  expect(document.activeElement).toBe(within(panel).getByRole("button", { name: "Reload" }));
});

// M8/P1 fix check 2, R2-1: what shows needs no comparison in a worker.
test("a large page the same as the version, compared in no worker, can be restored", async () => {
  const lines = Array.from({ length: 1200 }, (_, i) => `line ${i.toString()}\n`).join("");
  const { dialog, confirm } = await same(`${lines}Install\nthree\n`, `${lines}Install\nthree\n`);
  expect(await within(dialog).findByText("No changes.")).toBeTruthy();
  await waitFor(() => expect(confirm.hasAttribute("disabled")).toBe(false));
  expect(worker.asked).toHaveLength(0);
});

test("a page that differs from the version in its line breaks alone can be restored", async () => {
  const { dialog, confirm } = await same("Install\r\nthree\r\n", "Install\nthree\n");
  expect(await within(dialog).findByText("Only the line breaks or the byte order mark differ.")).toBeTruthy();
  await waitFor(() => expect(confirm.hasAttribute("disabled")).toBe(false));
});

test("a page written since it was read whose content could not be read again says so, claiming no comparison with it", async () => {
  let reads = 0;
  const { user, server, dialog } = await restoring({
    "GET /api/v0/pages/*/content": () =>
      ++reads === 1
        ? json({ content: "Install now\n", revision: 6, content_hash: "0".repeat(64) })
        : problem(500, "internal"),
  });
  server.contents.set(install.id, { content: "Install later\n", revision: 7 });

  await user.click(within(dialog).getByRole("button", { name: "Restore" }));

  expect((await within(dialog).findByRole("alert")).textContent).toBe("This page has changed since you read it.");
  expect(within(dialog).queryByText(/The page changed meanwhile/)).toBeNull();
  expect(reads).toBe(2);
});

// M8/P1 fix check 3, R3-1: the session left as the answer's body is read, its headers past the session's check.
test.each([
  { answer: "refused, the page written since", status: 409, body: { status: 409, code: "page.revision_mismatch" } },
  { answer: "restored", status: 200, body: { revision: 7, changed: true } },
  // M8/P1 fix check 4, R4-1.
  {
    answer: "refused, the page edited",
    status: 409,
    body: { status: 409, code: "page.locked", lock: { page_id: install.id, ...bob } },
  },
])(
  "a restore $answer, read once the tab signed out, its cache gone, leaves no rejection unhandled",
  async ({ status, body }) => {
    let release: (() => void) | undefined;
    const { user, app, dialog } = await restoring({
      "POST /api/v0/pages/*/versions/*/restore": () =>
        new Response(
          new ReadableStream<Uint8Array>({
            start(controller) {
              release = () => {
                controller.enqueue(new TextEncoder().encode(JSON.stringify(body)));
                controller.close();
              };
            },
          }),
          { status, headers: { "Content-Type": status === 200 ? "application/json" : "application/problem+json" } }
        ),
    });
    const rejections: unknown[] = [];
    const record = (reason: unknown) => void rejections.push(reason);
    process.on("unhandledRejection", record);
    onTestFinished(() => {
      process.off("unhandledRejection", record);
    });

    await user.click(within(dialog).getByRole("button", { name: "Restore" }));
    await waitFor(() => expect(release).toBeDefined());
    // The answer's headers pass the session's check; its body is still on its way.
    await act(async () => new Promise((resolve) => setTimeout(resolve, 10)));
    await act(() => app.session.tokens.signOut());
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    await act(async () => {
      release?.();
      await new Promise((resolve) => setTimeout(resolve, 50));
    });
    expect(rejections).toEqual([]);
  }
);

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
