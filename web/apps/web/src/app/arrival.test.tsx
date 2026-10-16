import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { eventHandlers } from "../events/handlers";
import { eventServer, withEvents } from "../test/event-server";
import { notebookJSON, workspaceJSON } from "../test/fakes";
import { assetNode, guide, install, linux, notes, pagePath, pageServer } from "../test/page-server";
import { renderApp } from "../test/render";
import { arrived } from "./arrival";

// The arrival focus (M3/P4 design 3.5): a page's heading takes it as the page shows, arrived at; one that shows
// later for the same arrival, the page deleted or restored elsewhere, takes it only from no one (M8 closeout K1-1).

const treeEvent = { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, tree: true, pages: [] };
const photo = assetNode(21, "photo.png", install);

/** shown is Install arrived at with state, its stream connected: another's writes come as events. */
async function shown(state: unknown) {
  const user = userEvent.setup();
  const events = eventServer();
  const server = pageServer({
    nodes: [guide, install, linux, notes, photo],
    answers: { "GET /api/v0/events": events.answer },
  });
  const { router } = renderApp([{ pathname: pagePath(install.id), state }], withEvents(server.app), {
    eventHandlers,
  });
  const heading = await screen.findByRole("heading", { level: 1, name: "Install" });
  await waitFor(() => expect(events.streams).toHaveLength(1));
  events.last().hello();
  await act(() => new Promise((resolve) => setTimeout(resolve, 100)));
  const box = screen.getByRole("searchbox", { name: "Search terms" }) as HTMLInputElement;
  return { user, events, server, router, heading, box };
}

/** deleted has another delete Install and what is under it, its event coming. */
function deleted({ events, server }: Awaited<ReturnType<typeof shown>>) {
  server.nodes = server.nodes.filter((each) => each.id !== install.id && each.id !== linux.id && each.id !== photo.id);
  events.last().send("pages", treeEvent);
}

// Check-1's probe: the theft it found in NotFoundPage, with either arrival.
test.each([
  ["arrived at", arrived],
  ["gone to for an attachment", { attachment: photo.id }],
] as const)(
  "a page %s, deleted elsewhere as the reader types in the search box, says it is not found, the focus and the keys staying in the box",
  async (_, state) => {
    const page = await shown(state);
    const { user, box } = page;
    await user.click(box);
    await user.keyboard("abc");

    deleted(page);
    await screen.findByRole("heading", { level: 1, name: "Page not found" });
    await act(() => new Promise((resolve) => setTimeout(resolve, 100)));
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
  await act(() => new Promise((resolve) => setTimeout(resolve, 600)));
  events.last().send("pages", treeEvent);
  await screen.findByRole("heading", { level: 1, name: "Install" });
  await act(() => new Promise((resolve) => setTimeout(resolve, 100)));
  await user.keyboard("d");
  expect(document.activeElement).toBe(box);
  expect(box.value).toBe("abcd");
});

test("a page gone to for an attachment that is not found takes the focus, as one arrived at", async () => {
  const user = userEvent.setup();
  const server = pageServer({ nodes: [guide, notes] });
  renderApp([{ pathname: pagePath(install.id), state: { attachment: photo.id } }], server.app);
  const notFound = await screen.findByRole("heading", { level: 1, name: "Page not found" });
  await waitFor(() => expect(document.activeElement).toBe(notFound));
  void user;
});

test("the first heading shown for an arrival takes the focus wherever it is; Back to an arrival, the page left took its focus with it", async () => {
  const { user, router, heading, box } = await shown(arrived);
  await waitFor(() => expect(document.activeElement).toBe(heading));
  await user.click(box);

  await act(() => router.navigate(pagePath(linux.id), { state: arrived }));
  const linuxHeading = await screen.findByRole("heading", { level: 1, name: "Linux" });
  await waitFor(() => expect(document.activeElement).toBe(linuxHeading));

  await act(() => router.navigate(-1));
  const again = await screen.findByRole("heading", { level: 1, name: "Install" });
  await waitFor(() => expect(document.activeElement).toBe(again));
});
