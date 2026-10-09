import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { notebookJSON, problem } from "../../test/fakes";
import { jobJSON, jobsServer } from "../../test/jobs-server";
import { guide, pagePath } from "../../test/page-server";
import { pageEditor } from "../../test/page-editor";
import { renderApp } from "../../test/render";

// A page's menu beside its title: Export this page (M7/P5 design 4.5).

const transfer = `/lab/notebooks/${notebookJSON.id}/settings/transfer`;
const menu = () => screen.findByRole("button", { name: "Page actions" });

/** Opens Guide's menu and its export's dialog; before, check sees the page. */
async function exporting(server: ReturnType<typeof jobsServer>, check: () => void = () => {}) {
  const user = userEvent.setup();
  const rendered = renderApp(pagePath(guide.id), server.app);
  const opener = await menu();
  check();
  await user.click(opener);
  await user.click(await screen.findByRole("menuitem", { name: "Export this page" }));
  const dialog = await screen.findByRole("alertdialog", { name: "Export the page Guide and its subpages?" });
  return { user, dialog, ...rendered };
}

test("a reader exports a page with its subtree; the notebook's imports and exports show, the job's row focused", async () => {
  const server = jobsServer({ role: "reader" });
  const { user, dialog, router } = await exporting(server, () =>
    expect(screen.queryByRole("button", { name: "Edit" })).toBeNull()
  );

  await user.click(within(dialog).getByRole("button", { name: "Export" }));

  const list = await screen.findByRole("list", { name: "Recent jobs" });
  expect(router.state.location.pathname).toBe(transfer);
  const job = within(list).getByRole("listitem", { name: (name) => name.startsWith("Export of the page Guide, ") });
  await waitFor(() => expect(document.activeElement).toBe(job));
  expect(server.asked.filter((ask) => ask.startsWith("POST"))).toEqual(["POST export Guide"]);
});

test("the export cancelled, the focus goes back to the menu, the page still shown", async () => {
  const server = jobsServer();
  const { user, router } = await exporting(server);

  await user.click(screen.getByRole("button", { name: "Cancel" }));

  await waitFor(async () => expect(document.activeElement).toBe(await menu()));
  expect(router.state.location.pathname).toBe(pagePath(guide.id));
  expect(server.asked).toEqual([]);
});

test("a page gone meanwhile says so in the dialog, which stays", async () => {
  const server = jobsServer({
    answers: { [`POST /api/v0/notebooks/${notebookJSON.id}/exports`]: () => problem(404, "page.not_found") },
  });
  const { user, dialog, router } = await exporting(server);

  await user.click(within(dialog).getByRole("button", { name: "Export" }));

  expect((await within(dialog).findByRole("alert")).textContent).toBe("This page no longer exists.");
  expect(router.state.location.pathname).toBe(pagePath(guide.id));
});

test("the jobs that cannot be read on arrival say why, the job started not shown alone; Try again lists them", async () => {
  const server = jobsServer({ jobs: [jobJSON(1)] });
  const { user, dialog } = await exporting(server);
  server.listDown = true;

  await user.click(within(dialog).getByRole("button", { name: "Export" }));

  expect((await screen.findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );
  expect(screen.queryByRole("list", { name: "Recent jobs" })).toBeNull();
  server.listDown = false;
  await user.click(screen.getByRole("button", { name: "Try again" }));
  const list = await screen.findByRole("list", { name: "Recent jobs" });
  expect(within(list).getAllByRole("listitem")).toHaveLength(2);
});

test("while the page is edited, its menu is not there", async () => {
  const user = userEvent.setup();
  renderApp(pagePath(guide.id), jobsServer().app);
  await menu();

  await user.click(screen.getByRole("button", { name: "Edit" }));
  await pageEditor();

  expect(screen.queryByRole("button", { name: "Page actions" })).toBeNull();
});
