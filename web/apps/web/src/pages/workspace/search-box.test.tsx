import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, onTestFinished, test, vi } from "vitest";

import { eventHandlers } from "../../events/handlers";
import { eventServer, withEvents } from "../../test/event-server";
import { notebookJSON } from "../../test/fakes";
import { guide, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// A workspace's search box (M8/P5 design 4.2). jsdom's platform is not macOS: Mod is Ctrl.

afterEach(() => {
  vi.useRealTimers();
});

const home = `/lab/notebooks/${notebookJSON.id}`;

/** box is the search box's field; form, its landmark. */
const box = () => screen.getByRole("searchbox", { name: "Search terms" }) as HTMLInputElement;
const form = () => screen.getByRole("search", { name: "Search “Lab”" });

/** hintOf is what describes the field: its problem, then where it looks. */
const hintOf = (field: HTMLElement) =>
  (field.getAttribute("aria-describedby") ?? "")
    .split(" ")
    .map((id) => document.getElementById(id)?.textContent)
    .join(" | ");

/** Presses Mod+Shift+F (Ctrl here) on target, with more of the press; tells whether the browser's own action was let be. */
const modShiftF = (target: Element = document.body, more: KeyboardEventInit = {}) =>
  fireEvent.keyDown(target, { key: "F", code: "KeyF", ctrlKey: true, shiftKey: true, ...more });

test("the box is at the top of the workspace's left column, a search named for it: its field labelled, described by where it looks, with its shortcut, and a named Search; submitted from another page, the results' title takes the focus, described by what was found", async () => {
  const user = userEvent.setup();
  const events = eventServer();
  const server = pageServer({ answers: { "GET /api/v0/events": events.answer } });
  const { router } = renderApp("/lab", withEvents(server.app), { eventHandlers });

  const field = await screen.findByRole("searchbox", { name: "Search terms" });
  const nav = screen.getByRole("navigation", { name: "Lab" });
  expect(form().compareDocumentPosition(nav) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(field.getAttribute("type")).toBe("search");
  expect(field.getAttribute("aria-keyshortcuts")).toBe("Control+Shift+F");
  expect(hintOf(field)).toBe("Searches the whole workspace.");
  expect(within(form()).getByRole("button", { name: "Search" }).getAttribute("type")).toBe("submit");

  await user.type(field, "  Plans  {Enter}");
  const title = await screen.findByRole("heading", { level: 1, name: "Search “Plans”" });
  expect(router.state.location.pathname).toBe("/lab/search");
  expect(router.state.location.search).toBe("?q=Plans");
  await waitFor(() => expect(document.activeElement).toBe(title));
  await waitFor(() => expect(title.getAttribute("aria-describedby")).not.toBeNull());
  expect(document.getElementById(title.getAttribute("aria-describedby") ?? "")?.textContent).toBe("Nothing found.");
  expect(box().value).toBe("Plans");
});

test("in a notebook it searches the notebook, its hint saying so, on any of its pages; on the results page, the one the page searches", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  const { router } = renderApp(home, server.app);
  await waitFor(() => expect(hintOf(box())).toBe("Searches in “Plans”."));
  await act(() => router.navigate(`${home}/trash`));
  await screen.findByRole("heading", { level: 1, name: "Trash" });
  expect(hintOf(box())).toBe("Searches in “Plans”.");
  await act(() => router.navigate("/lab"));
  await waitFor(() => expect(hintOf(box())).toBe("Searches the whole workspace."));
  await act(() => router.navigate(pagePath(guide.id)));
  await screen.findByRole("heading", { level: 1, name: "Guide" });
  await waitFor(() => expect(hintOf(box())).toBe("Searches in “Plans”."));

  await user.type(box(), "Guide{Enter}");
  await screen.findByRole("list", { name: "Search results" });
  expect(router.state.location.search).toBe(`?q=Guide&notebook_id=${notebookJSON.id}`);
  expect(hintOf(box())).toBe("Searches in “Plans”.");

  // Asked again here: in the notebook still, the focus staying in the box.
  await user.clear(box());
  await user.type(box(), "Linux{Enter}");
  await waitFor(() => expect(router.state.location.search).toBe(`?q=Linux&notebook_id=${notebookJSON.id}`));
  expect(document.activeElement).toBe(box());

  await user.selectOptions(screen.getByRole("combobox", { name: "Where to look" }), "All notebooks");
  await waitFor(() => expect(hintOf(box())).toBe("Searches the whole workspace."));
  await user.click(box());
  await user.keyboard("{Enter}");
  await waitFor(() => expect(router.state.location.search).toBe("?q=Linux"));
});

test("a q past 256 bytes or holding NUL is not sent: under the box it says why, NUL first, as an alert said anew each time, the field invalid and taking the focus; one blank does nothing", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  const { router } = renderApp("/lab", server.app);
  const field = await screen.findByRole("searchbox", { name: "Search terms" });

  // 86 characters of 3 bytes: 258.
  fireEvent.change(field, { target: { value: "数".repeat(86) } });
  await user.click(within(form()).getByRole("button", { name: "Search" }));
  expect(field.getAttribute("aria-invalid")).toBe("true");
  expect(hintOf(field)).toBe(
    "At most 256 bytes and 8 terms, and at most 64 ASCII letters and digits in a row: shorten it. | Searches the whole workspace."
  );
  expect(document.activeElement).toBe(field);
  expect(router.state.location.pathname).toBe("/lab");

  fireEvent.change(field, { target: { value: `a${String.fromCharCode(0)}b` } });
  await user.click(within(form()).getByRole("button", { name: "Search" }));
  expect(hintOf(field)).toBe("It may not hold a NUL character. | Searches the whole workspace.");
  expect(router.state.location.pathname).toBe("/lab");
  // Past 256 bytes too: NUL, as the server checks it first.
  fireEvent.change(field, { target: { value: `${"数".repeat(86)}${String.fromCharCode(0)}` } });
  await user.click(within(form()).getByRole("button", { name: "Search" }));
  expect(hintOf(field)).toBe("It may not hold a NUL character. | Searches the whole workspace.");

  // Submitted with Enter, the field keeping the focus: an alert says it, and says it again the next time (P5B review A-M3).
  const said = within(form()).getByRole("alert");
  expect(said.textContent).toBe("It may not hold a NUL character.");
  await user.click(field);
  await user.keyboard("{Enter}");
  const again = within(form()).getByRole("alert");
  expect(again.textContent).toBe("It may not hold a NUL character.");
  expect(again).not.toBe(said);
  expect(document.activeElement).toBe(field);

  fireEvent.change(field, { target: { value: "   " } });
  await user.click(within(form()).getByRole("button", { name: "Search" }));
  expect(router.state.location.pathname).toBe("/lab");

  // 85 of them and a letter: 256, sent.
  fireEvent.change(field, { target: { value: `${"数".repeat(85)}a` } });
  await user.click(within(form()).getByRole("button", { name: "Search" }));
  await waitFor(() => expect(router.state.location.pathname).toBe("/lab/search"));
  expect(field.getAttribute("aria-invalid")).toBeNull();
  expect(hintOf(field)).toBe("Searches the whole workspace.");
});

test("an Enter that ends an IME's composition submits nothing, Safari's after it (229) neither; the Enter after it does", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  const { router } = renderApp("/lab", server.app);
  const field = await screen.findByRole("searchbox", { name: "Search terms" });
  await user.type(field, "数据");

  // Its implicit submission kept away.
  expect(fireEvent.keyDown(field, { key: "Enter", code: "Enter", isComposing: true })).toBe(false);
  expect(fireEvent.keyDown(field, { key: "Enter", code: "Enter", keyCode: 229 })).toBe(false);
  expect(router.state.location.pathname).toBe("/lab");

  await user.keyboard("{Enter}");
  await waitFor(() => expect(router.state.location.pathname).toBe("/lab/search"));
  expect(router.state.location.search).toBe(`?q=${encodeURIComponent("数据")}`);
});

test("Mod+Shift+F gives the box the focus, its text selected, instead of the browser's own; not while held, nor in a composition; over a dialog it only keeps the browser's", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  renderApp("/lab", server.app);
  const field = (await screen.findByRole("searchbox", { name: "Search terms" })) as HTMLInputElement;
  await user.type(field, "数据库");
  act(() => field.blur());

  // In a composition, the IME's: the focus does not leave with half a word.
  expect(modShiftF(document.body, { key: "Process", isComposing: true })).toBe(true);
  expect(modShiftF(document.body, { keyCode: 229 })).toBe(true);
  expect(document.activeElement).toBe(document.body);
  // Held: the browser's own action kept away, the focus where it is.
  expect(modShiftF(document.body, { repeat: true })).toBe(false);
  expect(document.activeElement).toBe(document.body);
  // Ctrl+F, Ctrl+Alt+Shift+F: not it.
  expect(fireEvent.keyDown(document.body, { key: "f", code: "KeyF", ctrlKey: true })).toBe(true);
  expect(modShiftF(document.body, { altKey: true })).toBe(true);
  expect(document.activeElement).toBe(document.body);

  expect(modShiftF()).toBe(false);
  expect(document.activeElement).toBe(field);
  expect([field.selectionStart, field.selectionEnd]).toEqual([0, 3]);

  // A Cyrillic layout's key at F's place.
  act(() => field.blur());
  expect(modShiftF(document.body, { key: "А" })).toBe(false);
  expect(document.activeElement).toBe(field);

  // Over a dialog: the focus stays.
  act(() => field.blur());
  const dialog = document.createElement("div");
  dialog.setAttribute("role", "dialog");
  document.body.append(dialog);
  onTestFinished(() => dialog.remove());
  expect(modShiftF()).toBe(false);
  expect(document.activeElement).toBe(document.body);
});

test("on macOS it is Cmd+Shift+F, as the field says", async () => {
  const platform = vi.spyOn(navigator, "platform", "get").mockReturnValue("MacIntel");
  onTestFinished(() => platform.mockRestore());
  const server = pageServer();
  renderApp("/lab", server.app);
  const field = await screen.findByRole("searchbox", { name: "Search terms" });
  expect(field.getAttribute("aria-keyshortcuts")).toBe("Meta+Shift+F");

  expect(modShiftF()).toBe(true);
  expect(document.activeElement).toBe(document.body);
  expect(fireEvent.keyDown(document.body, { key: "f", code: "KeyF", metaKey: true, shiftKey: true })).toBe(false);
  expect(document.activeElement).toBe(field);
});

test("on the results page the box holds the address's q as it changes, unless it was edited since", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  const { router } = renderApp("/lab/search?q=Guide", server.app);
  await screen.findByRole("heading", { level: 1, name: "Search “Guide”" });
  expect(box().value).toBe("Guide");

  await act(() => router.navigate("/lab/search?q=Linux"));
  await waitFor(() => expect(box().value).toBe("Linux"));
  await act(() => router.navigate(-1));
  await waitFor(() => expect(box().value).toBe("Guide"));

  await user.clear(box());
  await user.type(box(), "Notes");
  await act(() => router.navigate("/lab/search?q=Install"));
  await screen.findByRole("heading", { level: 1, name: "Search “Install”" });
  expect(box().value).toBe("Notes");

  // Elsewhere it keeps what it holds.
  await act(() => router.navigate(home));
  await screen.findByRole("heading", { level: 1, name: "Plans" });
  expect(box().value).toBe("Notes");
});
