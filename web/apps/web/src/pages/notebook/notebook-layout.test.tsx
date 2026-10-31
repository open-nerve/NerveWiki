import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import type { Notebook } from "../../services/notebook.service";
import { json, notebookJSON, signedInApp, type Answer } from "../../test/fakes";
import { renderApp } from "../../test/render";

// The shell of a notebook's pages, and its home (M3/P4 design 3.4).

const atlas: Notebook = { ...notebookJSON, id: "0199a2b4-0000-7000-8000-0000000000c2", name: "Atlas", member_count: 3 };

const withNotebooks = (answer: Answer = () => json({ data: [atlas, notebookJSON] })) =>
  signedInApp({ "GET /api/v0/workspaces/lab/notebooks": answer });

test("a notebook of the workspace's list opens on its home, which the left column marks", async () => {
  renderApp(`/lab/notebooks/${notebookJSON.id}`, withNotebooks());

  const heading = await screen.findByRole("heading", { level: 1, name: "Plans" });
  expect(screen.getByText("No pages yet.")).toBeTruthy();
  // Opened, not arrived at: the focus stays where the browser puts it.
  expect(document.activeElement).not.toBe(heading);
  const nav = screen.getByRole("navigation", { name: "Lab" });
  expect(within(nav).getByRole("link", { name: "Plans" }).getAttribute("aria-current")).toBe("page");
  expect(within(nav).getByRole("link", { name: "Atlas" }).getAttribute("aria-current")).toBeNull();
  expect(within(nav).getByRole("link", { name: "Home" }).getAttribute("aria-current")).toBeNull();
});

test.each([
  ["a notebook the account does not see", "0199a2b4-0000-7000-8000-0000000000ff"],
  ["an id that is no notebook's", "plans"],
])("%s is not found, inside the workspace", async (_name, id) => {
  renderApp(`/lab/notebooks/${id}`, withNotebooks());

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
  expect(screen.getByRole("navigation", { name: "Lab" })).toBeTruthy();
});

test("a notebook's pages start anew with each notebook", async () => {
  const user = userEvent.setup();
  renderApp(`/lab/notebooks/${notebookJSON.id}`, withNotebooks());
  const plans = await screen.findByRole("heading", { level: 1, name: "Plans" });

  await user.click(within(screen.getByRole("navigation", { name: "Lab" })).getByRole("link", { name: "Atlas" }));

  const atlasHeading = await screen.findByRole("heading", { level: 1, name: "Atlas" });
  expect(atlasHeading).not.toBe(plans);
  expect(plans.isConnected).toBe(false);
});

test("the shell says so when the notebooks cannot be loaded; Try again loads them", async () => {
  const user = userEvent.setup();
  let down = true;
  renderApp(
    `/lab/notebooks/${notebookJSON.id}`,
    withNotebooks(() => (down ? Promise.reject(new TypeError("offline")) : json({ data: [notebookJSON] })))
  );
  const alerts = await screen.findAllByRole("alert");
  expect(alerts.map((alert) => alert.textContent)).toEqual([
    "Cannot reach the server. Check the connection and try again.",
    "Cannot reach the server. Check the connection and try again.",
  ]);

  down = false;
  await user.click(screen.getAllByRole("button", { name: "Try again" })[1] as HTMLElement);

  expect(await screen.findByRole("heading", { level: 1, name: "Plans" })).toBeTruthy();
  expect(screen.queryByRole("alert")).toBeNull();
});

// M8/P2 design 4.5, M8/P3 design 4.5: the tree's links are its pages; the notebook's other places are a navigation of
// their own, health before history before trash.
test("the notebook's health, history and trash are in a navigation of its own below its pages, not the tree's; each place marks its own page", async () => {
  const user = userEvent.setup();
  const { router } = renderApp(`/lab/notebooks/${notebookJSON.id}`, withNotebooks());
  await screen.findByRole("heading", { level: 1, name: "Plans" });

  const tree = screen.getByRole("navigation", { name: "Pages of Plans" });
  expect(within(tree).queryByRole("link", { name: "Health" })).toBeNull();
  expect(within(tree).queryByRole("link", { name: "Trash" })).toBeNull();
  expect(within(tree).queryByRole("link", { name: "History" })).toBeNull();
  const places = screen.getByRole("navigation", { name: "More of Plans" });
  expect(
    within(places)
      .getAllByRole("link")
      .map((link) => link.textContent)
  ).toEqual(["Health", "History", "Trash"]);
  const history = within(places).getByRole("link", { name: "History" });
  const trash = within(places).getByRole("link", { name: "Trash" });
  expect(trash.getAttribute("aria-current")).toBeNull();
  expect(history.getAttribute("aria-current")).toBeNull();
  // Below the tree, in the document's order.
  expect(tree.compareDocumentPosition(places) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

  await user.click(trash);

  expect(await screen.findByRole("heading", { level: 1, name: "Trash" })).toBeTruthy();
  expect(trash.getAttribute("aria-current")).toBe("page");

  // An item's preview is under the trash, not the trash (M8/P2B review A-N2).
  await act(() => router.navigate(`/lab/notebooks/${notebookJSON.id}/trash/0199a2b4-0000-7000-8000-000000000777`));
  await waitFor(() => expect(trash.getAttribute("aria-current")).toBeNull());

  await user.click(history);

  expect(await screen.findByRole("heading", { level: 1, name: "History" })).toBeTruthy();
  expect(history.getAttribute("aria-current")).toBe("page");
  expect(trash.getAttribute("aria-current")).toBeNull();
  // A changeset's page is under the history, not the history.
  await act(() => router.navigate(`/lab/notebooks/${notebookJSON.id}/history/0199a2b4-0000-7000-8000-000000000777`));
  await waitFor(() => expect(history.getAttribute("aria-current")).toBeNull());
});
