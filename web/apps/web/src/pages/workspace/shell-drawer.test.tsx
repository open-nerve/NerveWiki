import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, onTestFinished, test, vi } from "vitest";

import { json, notebookJSON } from "../../test/fakes";
import { indexFacts } from "../../test/index-server";
import { pageEditor } from "../../test/page-editor";
import { guide, notes, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The left column at a phone's width (M13/P2/S3, 4.3.4): a drawer, inert while closed, a dialog that holds the
// focus while open; the menu of the page's strip opens it. jsdom has no inert: React writes the attribute alone,
// which is what these read (review C's P2C1-N5). Radix's menus give their trigger the focus back in a setTimeout(0),
// which jsdom, the switcher not inert, lets take it from the menu's button: a switch of workspace in the drawer is
// UI1's to check, in the browsers (review A's and C's).

/** A window narrower than md (Tailwind's max-md:), until set says otherwise; its change heard as the browser's is. */
function phone() {
  let narrow = true;
  const heard = new Set<() => void>();
  vi.stubGlobal("matchMedia", (query: string) => ({
    get matches() {
      return query === "(width < 48rem)" && narrow;
    },
    addEventListener: (_: string, listener: () => void) => heard.add(listener),
    removeEventListener: (_: string, listener: () => void) => heard.delete(listener),
  }));
  return {
    set: (to: boolean) =>
      act(() => {
        narrow = to;
        for (const listener of heard) {
          listener();
        }
      }),
  };
}

/** The left column's root: the drawer below md. */
const shell = () => document.querySelector<HTMLElement>("[data-shell]") as HTMLElement;
const menuButton = () => screen.getByRole("button", { name: "Navigation" });
const switcher = () => within(shell()).getByRole("button", { name: "Lab" });
const box = () => within(shell()).getByRole("searchbox", { name: "Search terms" }) as HTMLInputElement;
const treeLink = (title: string) =>
  within(within(shell()).getByRole("navigation", { name: "Pages of Plans" })).getByRole("link", { name: title });
const closed = () => expect(shell().hasAttribute("inert")).toBe(true);
/** The Trash's icon among Plans' places. */
const trash = () =>
  within(within(shell()).getByRole("navigation", { name: "More of Plans" })).getByRole("link", { name: "Trash" });
/** What the strip of a phone's width says beside its menu. */
const strip = () => menuButton().parentElement?.textContent;
/** The overlay beside the drawer open: the root's next sibling. */
const overlay = () => shell().nextElementSibling as HTMLElement;

/** Ctrl+K, Ctrl+Shift+F, Ctrl+O on what has the focus (jsdom is no Mac); whether the browser's own action was let be. */
const press = (key: string, more: KeyboardEventInit = {}) =>
  fireEvent.keyDown(document.activeElement ?? document.body, {
    key,
    code: `Key${key.toUpperCase()}`,
    ctrlKey: true,
    ...more,
  });

/** Guide's page at a phone's width, shown. */
async function guidePage(server = pageServer()) {
  const media = phone();
  const user = userEvent.setup();
  const { router } = renderApp(pagePath(guide.id), server.app);
  const heading = await screen.findByRole("heading", { level: 1, name: "Guide" });
  await waitFor(() => expect(treeLink("Notes")).toBeTruthy());
  return { media, user, router, server, heading };
}

/** Opens the drawer by its menu; resolves it, the dialog. */
async function openDrawer(user: ReturnType<typeof userEvent.setup>) {
  await user.click(menuButton());
  const dialog = await screen.findByRole("dialog", { name: "Navigation" });
  await waitFor(() => expect(document.activeElement).toBe(switcher()));
  return dialog;
}

test("(1) closed, the drawer is inert and no dialog, with no tabindex; the menu opens it, a dialog named Navigation, the focus on the switcher, which it keeps from a focus put outside; Escape closes it, the focus on the menu", async () => {
  const { user, heading } = await guidePage();
  closed();
  expect(shell().getAttribute("role")).toBeNull();
  expect(shell().hasAttribute("tabindex")).toBe(false);
  expect(menuButton().getAttribute("aria-expanded")).toBe("false");
  expect(menuButton().getAttribute("aria-controls")).toBe(shell().id);

  const dialog = await openDrawer(user);
  expect(dialog).toBe(shell());
  expect(shell().hasAttribute("inert")).toBe(false);
  expect(shell().getAttribute("aria-modal")).toBe("true");
  expect(menuButton().getAttribute("aria-expanded")).toBe("true");
  // Put outside by a program, the overlay keeping the pointer off the page: back in (review C-I2).
  act(() => heading.focus());
  await waitFor(() => expect(document.activeElement).toBe(switcher()));

  await user.keyboard("{Escape}");
  await waitFor(closed);
  expect(shell().getAttribute("role")).toBeNull();
  await waitFor(() => expect(document.activeElement).toBe(menuButton()));

  // The overlay beside it closes it too.
  await openDrawer(user);
  expect(overlay().getAttribute("aria-hidden")).toBe("true");
  await user.click(overlay());
  await waitFor(closed);
  await waitFor(() => expect(document.activeElement).toBe(menuButton()));
});

test("(1) open, the window does not scroll behind it: its overflow hidden, its scrollbar's gutter kept where the scrollbar took room, as they were again once closed, by Escape, a page chosen or a wider window (decision 809; review B's S3B1-N1)", async () => {
  const { media, user } = await guidePage();
  const html = document.documentElement.style;
  html.overflow = "auto";
  html.scrollbarGutter = "auto";
  onTestFinished(() => {
    html.overflow = "";
    html.scrollbarGutter = "";
  });
  const was = () => [html.overflow, html.scrollbarGutter];
  await openDrawer(user);
  expect(was()).toEqual(["hidden", "stable"]);
  await user.keyboard("{Escape}");
  await waitFor(closed);
  expect(was()).toEqual(["auto", "auto"]);

  await openDrawer(user);
  await user.click(treeLink("Notes"));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  closed();
  expect(was()).toEqual(["auto", "auto"]);

  await openDrawer(user);
  media.set(false);
  expect(was()).toEqual(["auto", "auto"]);

  // A scrollbar that takes no room (overlay or hidden, a page that fits; jsdom's clientWidth is 0): no gutter, which
  // would shift the page behind by a scrollbar's width.
  media.set(true);
  vi.spyOn(document.documentElement, "clientWidth", "get").mockReturnValue(window.innerWidth);
  await openDrawer(user);
  expect(was()).toEqual(["hidden", "auto"]);
  await user.keyboard("{Escape}");
  await waitFor(closed);
  expect(was()).toEqual(["auto", "auto"]);
});

test("the strip says where the page is: the notebook of the address, else the workspace", async () => {
  const { router } = await guidePage();
  expect(strip()).toBe("Plans");
  await act(() => router.navigate("/lab"));
  await screen.findByRole("heading", { level: 1, name: "Lab" });
  expect(strip()).toBe("Lab");
});

test("in Chinese the drawer and its menu are named 导航", async () => {
  phone();
  const user = userEvent.setup();
  const server = pageServer();
  server.app.preferences.setLocale("zh-CN");
  renderApp(pagePath(guide.id), server.app);
  await screen.findByRole("heading", { level: 1, name: "Guide" });
  await user.click(screen.getByRole("button", { name: "导航" }));
  expect(await screen.findByRole("dialog", { name: "导航" })).toBe(shell());
});

test("(2) the page loaded, the focus is in the body: the drawer's scope takes none as it mounts", async () => {
  await guidePage();
  await act(() => new Promise((resolve) => setTimeout(resolve, 50)));
  expect(document.activeElement).toBe(document.body);
});

test("(3) closed, Mod+K opens it at the box; open, Mod+K and Mod+Shift+F focus the box, Mod+O opens no quick switch; Home chosen in it, Mod+K opens it again", async () => {
  const { user } = await guidePage();
  await user.type(box(), "Linux");
  act(() => box().blur());

  expect(press("k")).toBe(false);
  expect(await screen.findByRole("dialog", { name: "Navigation" })).toBe(shell());
  await waitFor(() => expect(document.activeElement).toBe(box()));
  expect([box().selectionStart, box().selectionEnd]).toEqual([0, 5]);

  act(() => switcher().focus());
  expect(press("k")).toBe(false);
  expect(document.activeElement).toBe(box());
  act(() => switcher().focus());
  expect(press("F", { shiftKey: true })).toBe(false);
  expect(document.activeElement).toBe(box());
  // The drawer is a dialog to the page's other shortcuts.
  expect(press("o")).toBe(false);
  expect(screen.queryByRole("dialog", { name: "Go to a page" })).toBeNull();

  // Home chosen in it: closed, the focus on the menu; the shortcuts read the drawer as it is now (review A's P2A1-M2).
  await user.click(within(shell()).getByRole("link", { name: "Home" }));
  await screen.findByRole("heading", { level: 1, name: "Lab" });
  closed();
  await waitFor(() => expect(document.activeElement).toBe(menuButton()));
  expect(press("k")).toBe(false);
  expect(await screen.findByRole("dialog", { name: "Navigation" })).toBe(shell());
  await waitFor(() => expect(document.activeElement).toBe(box()));
});

test("(3) a menu open in the main, Mod+K and Mod+Shift+F open no drawer under it: the menu's, as a dialog's, kept from the browser (decision 806)", async () => {
  const { user } = await guidePage();
  await user.click(within(screen.getByRole("main")).getByRole("button", { name: "Page actions" }));
  const menu = await screen.findByRole("menu");
  expect(press("k")).toBe(false);
  expect(press("F", { shiftKey: true })).toBe(false);
  await act(() => new Promise((resolve) => setTimeout(resolve, 20)));
  closed();
  expect(screen.queryByRole("dialog", { name: "Navigation" })).toBeNull();
  expect(screen.getByRole("menu")).toBe(menu);
});

test("(4) a page chosen in its tree closes it, the page's title taking the focus; Home, the menu; a search from it, the results' title; the page shown, the menu", async () => {
  const { user, router } = await guidePage();

  await openDrawer(user);
  await user.click(treeLink("Notes"));
  const title = await screen.findByRole("heading", { level: 1, name: "Notes" });
  closed();
  await waitFor(() => expect(document.activeElement).toBe(title));
  expect(router.state.location.state).toEqual({ arrived: true });

  await openDrawer(user);
  await user.click(within(shell()).getByRole("link", { name: "Home" }));
  await screen.findByRole("heading", { level: 1, name: "Lab" });
  closed();
  await waitFor(() => expect(document.activeElement).toBe(menuButton()));

  await openDrawer(user);
  await user.type(box(), "Guide{Enter}");
  const results = await screen.findByRole("heading", { level: 1, name: "Search “Guide”" });
  closed();
  await waitFor(() => expect(document.activeElement).toBe(results));

  // The page shown chosen again: its address replaced, opened as ever, the heading taking nothing (probe T8).
  await act(() => router.navigate(pagePath(notes.id)));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  await openDrawer(user);
  await user.click(treeLink("Notes"));
  closed();
  await waitFor(() => expect(document.activeElement).toBe(menuButton()));
  expect(router.state.location.state).toEqual({ opened: true });
});

test("(4) in an LLM Wiki its Directory row closes it, the directory's title taking the focus (review A's P2A2-M1); the directory shown, the menu", async () => {
  phone();
  const user = userEvent.setup();
  const server = pageServer({
    mode: "llm_wiki",
    answers: { [`GET /api/v0/notebooks/${notebookJSON.id}/index`]: () => json(indexFacts()) },
  });
  const { router } = renderApp(pagePath(guide.id), server.app);
  await screen.findByRole("heading", { level: 1, name: "Guide" });
  await waitFor(() => expect(treeLink("Directory")).toBeTruthy());
  await openDrawer(user);
  await user.click(treeLink("Directory"));
  const title = await screen.findByRole("heading", { level: 1, name: "Directory" });
  closed();
  await waitFor(() => expect(document.activeElement).toBe(title));

  // The directory shown chosen again: its address replaced, opened as ever, the menu taking the focus (review C's
  // S3C-M2).
  await openDrawer(user);
  await user.click(treeLink("Directory"));
  closed();
  await waitFor(() => expect(document.activeElement).toBe(menuButton()));
  expect(router.state.location.state).toEqual({ opened: true });
});

test("(4) its places' icons are followed as the tree's rows are: a place's title taking the focus; the place shown, the menu (decision 808)", async () => {
  const { user, router } = await guidePage();
  await openDrawer(user);
  await user.click(trash());
  const title = await screen.findByRole("heading", { level: 1, name: "Trash" });
  closed();
  await waitFor(() => expect(document.activeElement).toBe(title));
  expect(router.state.location.state).toEqual({ arrived: true });

  await openDrawer(user);
  await user.click(trash());
  closed();
  await waitFor(() => expect(document.activeElement).toBe(menuButton()));
  expect(router.state.location.state).toEqual({ opened: true });
});

test("(4) Back to the entry it was opened at leaves it closed: its state went with the navigation (review C's S3C-M1)", async () => {
  const { user, router } = await guidePage();
  await openDrawer(user);
  await user.click(treeLink("Notes"));
  const title = await screen.findByRole("heading", { level: 1, name: "Notes" });
  closed();
  await waitFor(() => expect(document.activeElement).toBe(title));

  await act(() => router.navigate(-1));
  await screen.findByRole("heading", { level: 1, name: "Guide" });
  await act(() => new Promise((resolve) => setTimeout(resolve, 20)));
  closed();
  expect(screen.queryByRole("dialog", { name: "Navigation" })).toBeNull();
});

test("(4) Back, the drawer open, to a page the tree opened gives its title the focus (probe T9)", async () => {
  const { media, user, router } = await guidePage();
  // Opened by the tree at a wide window, where its links are opened ones.
  media.set(false);
  await user.click(treeLink("Notes"));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  expect(router.state.location.state).toEqual({ opened: true });
  media.set(true);
  await openDrawer(user);
  await user.click(treeLink("Guide"));
  await screen.findByRole("heading", { level: 1, name: "Guide" });
  closed();

  // Open again, the focus in it: Back leaves it for the page's title, from no one.
  await openDrawer(user);
  await act(() => router.navigate(-1));
  const title = await screen.findByRole("heading", { level: 1, name: "Notes" });
  closed();
  await waitFor(() => expect(document.activeElement).toBe(title));
});

test("(5) Escape in its account menu closes the menu alone; in a composition it is the IME's; in the box holding text the browser's, which empties it first", async () => {
  const { user } = await guidePage();
  const dialog = await openDrawer(user);

  await user.click(within(shell()).getByRole("button", { name: "Ada" }));
  await screen.findByRole("menu");
  await user.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());
  expect(screen.getByRole("dialog", { name: "Navigation" })).toBe(dialog);

  act(() => switcher().focus());
  fireEvent.keyDown(switcher(), { key: "Escape", isComposing: true });
  await act(() => new Promise((resolve) => setTimeout(resolve, 20)));
  expect(screen.getByRole("dialog", { name: "Navigation" })).toBe(dialog);

  // A browser empties the box after the key's dispatch, its default action: as this listener does.
  await user.type(box(), "x");
  const empty = (event: KeyboardEvent) => {
    if (event.key === "Escape") {
      queueMicrotask(() => {
        box().value = "";
      });
    }
  };
  box().addEventListener("keydown", empty);
  fireEvent.keyDown(box(), { key: "Escape" });
  await act(() => new Promise((resolve) => setTimeout(resolve, 20)));
  expect(screen.getByRole("dialog", { name: "Navigation" })).toBe(dialog);
  fireEvent.keyDown(box(), { key: "Escape" });
  closed();
  box().removeEventListener("keydown", empty);

  // One that does not empty it (jsdom does not, nor Playwright's WebKit): it closes, a task later.
  await openDrawer(user);
  await user.type(box(), "y");
  fireEvent.keyDown(box(), { key: "Escape" });
  expect(screen.getByRole("dialog", { name: "Navigation" })).toBe(dialog);
  await waitFor(closed);
  await waitFor(() => expect(document.activeElement).toBe(menuButton()));
});

test("(5) the box's Escape, the drawer closed before its task came, closes nothing more: the next page chosen in it takes the focus", async () => {
  const { user } = await guidePage();
  await openDrawer(user);
  await user.type(box(), "z");
  fireEvent.keyDown(box(), { key: "Escape" });
  // The overlay pressed before the Escape's task.
  fireEvent.click(overlay());
  await waitFor(closed);
  await waitFor(() => expect(document.activeElement).toBe(menuButton()));
  await act(() => new Promise((resolve) => setTimeout(resolve, 20)));

  await openDrawer(user);
  await user.click(treeLink("Notes"));
  const title = await screen.findByRole("heading", { level: 1, name: "Notes" });
  await waitFor(() => expect(document.activeElement).toBe(title));
  await act(() => new Promise((resolve) => setTimeout(resolve, 20)));
  expect(document.activeElement).toBe(title);
});

test("(6) the window wider, it is no drawer, the focus where it was, a click on its blank keeping it in the body; narrower again, closed", async () => {
  const { media, user } = await guidePage();
  await openDrawer(user);

  media.set(false);
  expect(shell().hasAttribute("inert")).toBe(false);
  expect(shell().getAttribute("role")).toBeNull();
  expect(shell().hasAttribute("tabindex")).toBe(false);
  expect(document.activeElement).toBe(switcher());
  act(() => switcher().blur());
  await user.click(shell());
  expect(document.activeElement).toBe(document.body);

  media.set(true);
  closed();
  expect(shell().getAttribute("role")).toBeNull();
  expect(document.activeElement).toBe(document.body);
});

test("(6) the window narrower, the focus in the column goes to the menu, not to the body as the drawer closed is inert (review A's S3A-N1)", async () => {
  const { media } = await guidePage();
  media.set(false);
  act(() => switcher().focus());
  // Not scrolled to: the strip at the page's top, a page scrolled down would jump to it (review A's S3A1-N1).
  const focus = vi.spyOn(menuButton(), "focus");
  media.set(true);
  closed();
  expect(document.activeElement).toBe(menuButton());
  expect(focus).toHaveBeenCalledWith({ preventScroll: true });
});

test("(7) an unsaved edit, a page chosen in the drawer asks first; Stay closes it quietly, the focus back in the editor, the address the same (decision 799)", async () => {
  phone();
  const user = userEvent.setup();
  const { router } = renderApp(pagePath(guide.id), pageServer().app);
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  const { type } = await pageEditor();
  type("draft");
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Unsaved changes"));

  await openDrawer(user);
  await user.click(treeLink("Notes"));
  const ask = await screen.findByRole("alertdialog", { name: "Leave without saving?" });
  await user.click(within(ask).getByRole("button", { name: "Stay" }));
  closed();
  expect(shell().getAttribute("role")).toBeNull();
  const { content } = await pageEditor();
  await waitFor(() => expect(document.activeElement).toBe(content));
  expect(router.state.location.pathname).toBe(pagePath(guide.id));
});

test("(5) a choice of Where to look waiting, Mod+K opens it at the box: the choice used as the box takes the focus, a replace of the query alone, leaves it open, the focus in the box (decision 807)", async () => {
  phone();
  const server = pageServer();
  const { router } = renderApp(`/lab/search?q=Guide`, server.app);
  await screen.findByRole("heading", { level: 1, name: "Search “Guide”" });
  const scope = screen.getByRole("combobox", { name: "Where to look" }) as HTMLSelectElement;
  act(() => scope.focus());
  // Stepped to with the keys, as the browsers send them: waiting until the select is left (useChoice).
  fireEvent.keyDown(scope, { key: "ArrowDown" });
  fireEvent.change(scope, { target: { value: notebookJSON.id } });
  const searches = () => server.sent.filter((each) => each.startsWith("GET search")).length;
  const before = searches();

  expect(press("k")).toBe(false);
  await waitFor(() => expect(router.state.location.search).toBe(`?q=Guide&notebook_id=${notebookJSON.id}`));
  await waitFor(() => expect(searches()).toBe(before + 1));
  await act(() => new Promise((resolve) => setTimeout(resolve, 20)));
  expect(screen.getByRole("dialog", { name: "Navigation" })).toBe(shell());
  expect(document.activeElement).toBe(box());
});

test("(5) only a replace of the query alone leaves it open: a search from it on the results page, a push, closes it, the menu taking the focus (review A's S3A-Q2, as it was); Back to it, and a replace to another page and query, close it (decision 807)", async () => {
  phone();
  const user = userEvent.setup();
  const { router } = renderApp(`/lab/search?q=Guide`, pageServer().app);
  await screen.findByRole("heading", { level: 1, name: "Search “Guide”" });
  await openDrawer(user);
  await user.clear(box());
  await user.type(box(), "Notes{Enter}");
  await screen.findByRole("heading", { level: 1, name: "Search “Notes”" });
  closed();
  await waitFor(() => expect(document.activeElement).toBe(menuButton()));

  // Back to the first search, the same path, another query: no replace, it closes (review C's S3C1-N2).
  await openDrawer(user);
  await act(() => router.navigate(-1));
  await screen.findByRole("heading", { level: 1, name: "Search “Guide”" });
  closed();

  await openDrawer(user);
  await act(() => router.navigate(`${pagePath(notes.id)}?from=search`, { replace: true }));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  closed();
});
