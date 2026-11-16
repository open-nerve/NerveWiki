import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import type { Notebook } from "../services/notebook.service";
import { eventHandlers } from "../events/handlers";
import { eventServer, withEvents } from "../test/event-server";
import { json, notebookJSON, workspaceJSON, type Answer } from "../test/fakes";
import { indexFacts } from "../test/index-server";
import { assetNode, guide, install, linux, notes, pagePath, pageServer } from "../test/page-server";
import { renderApp } from "../test/render";
import { arrived } from "./arrival";

// The arrival focus (M3/P4 design 3.5): a page's heading takes it as the page shows, arrived at, on the first mount
// for a navigation; one that shows later for the same navigation, the page deleted or restored elsewhere, takes it
// only from no one (M8 closeout K1-1, K2-1). Back and Forward are new navigations (K2-2). Each test starts at the
// workspace's home and goes on by a push, as a browser does, so it stands alone.

const treeEvent = { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, tree: true, pages: [] };
const photo = assetNode(21, "photo.png", install);

/** settle lets what an event or a navigation starts finish. */
const settle = (ms = 100) => act(() => new Promise((resolve) => setTimeout(resolve, ms)));

/**
 * home is the workspace's home shown, its stream connected: another's writes come as events. Plans is mode's, the
 * server answering answers too.
 */
async function home(
  nodes = [guide, install, linux, notes, photo],
  { mode, answers = {} }: { mode?: Notebook["mode"]; answers?: Record<string, Answer> } = {}
) {
  const user = userEvent.setup();
  const events = eventServer();
  const server = pageServer({ nodes, mode, answers: { ...answers, "GET /api/v0/events": events.answer } });
  const { router } = renderApp("/lab", withEvents(server.app), { eventHandlers });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  events.last().hello();
  const box = (await screen.findByRole("searchbox", { name: "Search terms" })) as HTMLInputElement;
  await settle();
  return { user, events, server, router, box };
}

/** shown is Install gone to from home with state, a push, shown. */
async function shown(state: unknown) {
  const page = await home();
  await act(() => page.router.navigate(pagePath(install.id), { state }));
  const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
  return { ...page, heading };
}

/** deleted has another delete Install and what is under it, its event coming. */
function deleted({ events, server }: Awaited<ReturnType<typeof home>>) {
  server.nodes = server.nodes.filter((each) => each.id !== install.id && each.id !== linux.id && each.id !== photo.id);
  events.last().send("pages", treeEvent);
}

/** treeLink is the link to title in the notebook's tree. */
async function treeLink(title: string) {
  const tree = screen.getByRole("navigation", { name: "Pages of Plans" });
  return waitFor(() => {
    const link = [...tree.querySelectorAll("a")].find((each) => each.textContent === title);
    expect(link).toBeDefined();
    return link as HTMLAnchorElement;
  });
}

// Check-1's probe and check-2's P1: the theft in NotFoundPage, with either arrival.
test.each([
  ["arrived at", arrived, () => screen.getByRole("heading", { level: 1, name: "Install" })],
  [
    "gone to for an attachment",
    { attachment: photo.id },
    () => document.querySelector(`[data-attachment="${photo.id}"]`),
  ],
] as const)(
  "a page %s, deleted elsewhere as the reader types in the search box, says it is not found, the focus and the keys staying in the box",
  async (_, state, focused) => {
    const page = await shown(state);
    const { user, box } = page;
    // The page's heading, or the attachment's row, took the focus.
    await waitFor(() => expect(document.activeElement).toBe(focused()));
    await settle();
    await user.click(box);
    await user.keyboard("abc");

    deleted(page);
    await screen.findByRole("heading", { level: 1, name: "Page not found" });
    await settle();
    await user.keyboard("d");
    expect(document.activeElement).toBe(box);
    expect(box.value).toBe("abcd");
  }
);

test("a page arrived at, deleted elsewhere, its title having the focus, says it is not found, its title taking the focus; restored elsewhere as the reader types in the search box, it shows again, the focus and the keys staying in the box", async () => {
  const page = await shown(arrived);
  const { user, events, server, heading, box } = page;
  await waitFor(() => expect(document.activeElement).toBe(heading));

  deleted(page);
  const notFound = await screen.findByRole("heading", { level: 1, name: "Page not found" });
  // The shell, gone, took the focus with it: no one has it.
  await waitFor(() => expect(document.activeElement).toBe(notFound));

  await user.click(box);
  await user.keyboard("abc");
  server.nodes = [guide, install, linux, notes, photo];
  // Past the tree's interval of reads.
  await settle(600);
  events.last().send("pages", treeEvent);
  await screen.findByRole("heading", { level: 1, name: "Install" });
  await settle();
  await user.keyboard("d");
  expect(document.activeElement).toBe(box);
  expect(box.value).toBe("abcd");
});

test("a page gone to for an attachment that is not found takes the focus, as one arrived at", async () => {
  const { router } = await home([guide, notes]);
  await act(() => router.navigate(pagePath(install.id), { state: { attachment: photo.id } }));
  const notFound = await screen.findByRole("heading", { level: 1, name: "Page not found" });
  await waitFor(() => expect(document.activeElement).toBe(notFound));
});

// Check-2's P3: Back to a page arrived at, the focus left on the tree's link that went on from it.
test("Back to a page arrived at, from a page gone to by the tree, takes the focus from the tree's link to its title", async () => {
  const { user, router, heading } = await shown(arrived);
  await waitFor(() => expect(document.activeElement).toBe(heading));

  const link = await treeLink("Notes");
  await user.click(link);
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  await settle();
  expect(document.activeElement).toBe(link);

  await act(() => router.navigate(-1));
  const again = await screen.findByRole("heading", { level: 1, name: "Install" });
  await waitFor(() => expect(document.activeElement).toBe(again));
});

test.each([
  ["on the tree's link", async () => (await treeLink("Notes")).focus()],
  ["in the search box", async () => screen.getByRole("searchbox", { name: "Search terms" }).focus()],
] as const)(
  "the first title shown for a navigation takes the focus %s: a page arrived at, Back to one, Forward to one",
  async (_, focusElsewhere) => {
    const { router, heading } = await shown(arrived);
    await waitFor(() => expect(document.activeElement).toBe(heading));

    await act(focusElsewhere);
    await act(() => router.navigate(pagePath(linux.id), { state: arrived }));
    const linuxHeading = await screen.findByRole("heading", { level: 1, name: "Linux" });
    await waitFor(() => expect(document.activeElement).toBe(linuxHeading));

    await act(focusElsewhere);
    await act(() => router.navigate(-1));
    const installAgain = await screen.findByRole("heading", { level: 1, name: "Install" });
    await waitFor(() => expect(document.activeElement).toBe(installAgain));

    await act(focusElsewhere);
    await act(() => router.navigate(1));
    const linuxAgain = await screen.findByRole("heading", { level: 1, name: "Linux" });
    await waitFor(() => expect(document.activeElement).toBe(linuxAgain));
  }
);

// The heading that stays mounted, the results page's, its navigation changing under it.
test("Back on the results page, from a search asked again there to the one arrived at, takes the focus from the box to its title", async () => {
  const { user, router, box } = await home();
  await user.type(box, "Install{Enter}");
  const title = await screen.findByRole("heading", { level: 1, name: "Search “Install”" });
  await waitFor(() => expect(document.activeElement).toBe(title));

  // Asked again here: the focus stays in the box.
  await user.clear(box);
  await user.type(box, "Linux{Enter}");
  await screen.findByRole("heading", { level: 1, name: "Search “Linux”" });
  await settle();
  expect(document.activeElement).toBe(box);

  await act(() => router.navigate(-1));
  await screen.findByRole("heading", { level: 1, name: "Search “Install”" });
  await waitFor(() => expect(document.activeElement).toBe(title));
});

/**
 * late is home whose tree, read as a page of Plans shows first, waits until release: the page's title shows late.
 * Held until the test holds it.
 */
async function late() {
  const nodes = [guide, install, linux, notes, photo];
  let holding = false;
  let release!: () => void;
  const released = new Promise<void>((resolve) => (release = resolve));
  const page = await home(nodes, {
    answers: {
      [`GET /api/v0/notebooks/${notebookJSON.id}/nodes`]: async () => {
        if (holding) {
          await released;
        }
        return json({ data: nodes });
      },
    },
  });
  holding = true;
  return { ...page, release: () => act(async () => release()) };
}

// M12/P2/S1, 28.4: a title that shows late for a page arrived at, its notebook's tree still read, takes the focus
// only from no one once the reader did something since the navigation committed (v0.1 design 13.2, item 26).
test("a title that shows late for a page arrived at, the reader typing in the search box meanwhile, takes no focus: the keys stay in the box", async () => {
  const { user, router, box, release } = await late();
  await act(() => router.navigate(pagePath(install.id), { state: arrived }));
  await user.click(box);
  await user.keyboard("abc");

  await release();
  await screen.findByRole("heading", { level: 1, name: "Install" });
  await settle();
  await user.keyboard("d");
  expect(document.activeElement).toBe(box);
  expect(box.value).toBe("abcd");
});

test("a title that shows late takes the focus wherever it is, the reader having done nothing", async () => {
  const { router, box, release } = await late();
  await act(() => router.navigate(pagePath(install.id), { state: arrived }));
  // A screen reader moves the focus by no reader's event.
  act(() => box.focus());

  await release();
  const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
  await waitFor(() => expect(document.activeElement).toBe(heading));
});

test("a replace of the entry's state alone is the same navigation: the keys typed before it still keep a late title from the box", async () => {
  const { user, router, box, release } = await late();
  await act(() => router.navigate(pagePath(install.id), { state: { ...arrived, said: 1 } }));
  await user.click(box);
  await user.keyboard("abc");
  await act(() => router.navigate(pagePath(install.id), { replace: true, state: arrived }));

  await release();
  await screen.findByRole("heading", { level: 1, name: "Install" });
  await settle();
  await user.keyboard("d");
  expect(document.activeElement).toBe(box);
  expect(box.value).toBe("abcd");
});

test("a replace of the entry's state alone is the same navigation: nor is a heading that mounts after it the navigation's first, Page not found taking the focus only from no one", async () => {
  const page = await shown({ ...arrived, said: 1 });
  const { router, heading, box } = page;
  await waitFor(() => expect(document.activeElement).toBe(heading));
  // Moved by no reader's event, as a screen reader moves it: no watch says the reader acted.
  act(() => box.focus());
  await act(() => router.navigate(pagePath(install.id), { replace: true, state: arrived }));

  deleted(page);
  await screen.findByRole("heading", { level: 1, name: "Page not found" });
  await settle();
  expect(document.activeElement).toBe(box);
});

// M12/P2/S1, 28.5: a page the tree opened keeps its mark in its entry; Back or Forward to it gives its title the focus
// from no one, as the page left took the focus with it.
test("a page the tree opened is no arrival; Back to it from a page arrived at from it gives its title the focus from no one", async () => {
  const { user, router, heading } = await shown(arrived);
  await waitFor(() => expect(document.activeElement).toBe(heading));
  const link = await treeLink("Notes");
  await user.click(link);
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  await settle();
  expect(document.activeElement).toBe(link);

  await act(() => router.navigate(pagePath(linux.id), { state: arrived }));
  const linuxHeading = await screen.findByRole("heading", { level: 1, name: "Linux" });
  await waitFor(() => expect(document.activeElement).toBe(linuxHeading));
  await act(() => router.navigate(-1));
  const notesHeading = await screen.findByRole("heading", { level: 1, name: "Notes" });
  await waitFor(() => expect(document.activeElement).toBe(notesHeading));
});

test("Back to a page the tree opened, the focus on the tree's link, leaves it there", async () => {
  const { user, router, heading } = await shown(arrived);
  await waitFor(() => expect(document.activeElement).toBe(heading));
  await user.click(await treeLink("Notes"));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  const guideLink = await treeLink("Guide");
  await user.click(guideLink);
  await screen.findByRole("heading", { level: 1, name: "Guide" });
  await settle();
  expect(document.activeElement).toBe(guideLink);

  await act(() => router.navigate(-1));
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  await settle();
  expect(document.activeElement).toBe(guideLink);
});

test("a page the tree opens, the click leaving the focus in the body (as Safari leaves it), is no arrival: its title takes no focus", async () => {
  const { heading } = await shown(arrived);
  await waitFor(() => expect(document.activeElement).toBe(heading));
  const link = await treeLink("Notes");
  act(() => heading.blur());
  expect(document.activeElement).toBe(document.body);

  fireEvent.click(link);
  await screen.findByRole("heading", { level: 1, name: "Notes" });
  await settle();
  expect(document.activeElement).toBe(document.body);
});

test("a page loaded first, its entry no arrival nor opened by the tree, takes no focus: it stays in the body", async () => {
  const server = pageServer({ nodes: [guide, install, linux, notes] });
  renderApp(pagePath(install.id), server.app);
  await screen.findByRole("heading", { level: 1, name: "Install" });
  await settle();
  expect(document.activeElement).toBe(document.body);
});

test.each([
  [
    "directory",
    "Directory",
    async (user: ReturnType<typeof userEvent.setup>) => {
      await screen.findByRole("option", { name: "note (1)" });
      await user.selectOptions(screen.getByRole("combobox", { name: "Type" }), "note");
    },
  ],
  [
    "history",
    "History",
    async (user: ReturnType<typeof userEvent.setup>) =>
      user.selectOptions(await screen.findByRole("combobox", { name: "Client" }), "API"),
  ],
] as const)(
  "the %s the tree opened keeps its mark through a filter: Back to it gives its title the focus",
  async (_, title, filter) => {
    const { user, router } = await home(undefined, {
      mode: "llm_wiki",
      answers: { [`GET /api/v0/notebooks/${notebookJSON.id}/index`]: () => json(indexFacts()) },
    });
    await act(() => router.navigate(pagePath(install.id), { state: arrived }));
    await screen.findByRole("heading", { level: 1, name: "Install" });
    const tree = screen.getByRole("navigation", { name: "Pages of Plans" });
    await user.click(await within(tree).findByRole("link", { name: title }));
    await screen.findByRole("heading", { level: 1, name: title });

    await filter(user);
    await waitFor(() => expect(router.state.location.search).not.toBe(""));
    expect(router.state.location.state).toEqual({ opened: true });
    await act(() => router.navigate(pagePath(linux.id), { state: arrived }));
    const linuxHeading = await screen.findByRole("heading", { level: 1, name: "Linux" });
    await waitFor(() => expect(document.activeElement).toBe(linuxHeading));
    await act(() => router.navigate(-1));
    const again = await screen.findByRole("heading", { level: 1, name: title });
    await waitFor(() => expect(document.activeElement).toBe(again));
  }
);
