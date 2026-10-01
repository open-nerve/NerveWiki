import { screen, within } from "@testing-library/react";
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
