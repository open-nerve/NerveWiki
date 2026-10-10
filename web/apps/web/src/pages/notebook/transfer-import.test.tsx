import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import type { TransferFailure } from "../../services/transfer.service";
import { transfers } from "../../test/attachments";
import { instanceJSON, json, notebookJSON, problem } from "../../test/fakes";
import { jobJSON, jobsServer, underWay } from "../../test/jobs-server";
import { assetNode, guide, pageNode } from "../../test/page-server";
import type { TreeNode } from "../../services/page.service";
import { renderApp } from "../../test/render";

// A notebook's imports from its settings (M7/P6 design 4.2).

const transfer = `/lab/notebooks/${notebookJSON.id}/settings/transfer`;
const imports = `POST /api/v0/notebooks/${notebookJSON.id}/imports`;
const vault = () => new File(["PK\u0003\u0004"], "Vault.zip", { type: "application/zip" });
const list = () => screen.findByRole("list", { name: "Recent jobs" });
const row = async (name: string) =>
  within(await list()).findByRole("listitem", { name: (named) => named.startsWith(name) });
const dialog = () => screen.findByRole("dialog", { name: "Import into Plans" });
/** Whether the browser asks before the page is left, as a beforeunload's default prevented tells. */
function asksBeforeLeaving(): boolean {
  const leaving = new Event("beforeunload", { cancelable: true });
  window.dispatchEvent(leaving);
  return leaving.defaultPrevented;
}

/** open opens the import's dialog of the settings' Import section. */
async function open(user: ReturnType<typeof userEvent.setup>): Promise<HTMLElement> {
  await user.click(await screen.findByRole("button", { name: "Import a zip…" }));
  return dialog();
}

test("an editor imports a zip under a page: the job goes first, the jobs read again, its row focused", async () => {
  const user = userEvent.setup();
  const server = jobsServer({ jobs: [jobJSON(1)] });
  renderApp(transfer, server.app);
  expect(await screen.findByRole("heading", { level: 2, name: "Import" })).toBeTruthy();
  // The import's section comes before the export's.
  expect(
    screen
      .getAllByRole("heading", { level: 2 })
      .map((heading) => heading.textContent)
      .slice(-3)
  ).toEqual(["Import", "Export", "Recent jobs"]);

  const shown = await open(user);
  expect(within(shown).getByText("The zip may be at most 512 MB.")).toBeTruthy();
  await user.upload(within(shown).getByLabelText("Zip archive"), vault());
  const place = within(shown).getByLabelText("Import under");
  expect(
    within(place)
      .getAllByRole("option")
      .map((option) => option.textContent)
  ).toContain("Guide");
  await user.selectOptions(place, guide.id);
  await user.click(within(shown).getByRole("button", { name: "Import" }));

  const imported = await row("Import of Vault.zip");
  await waitFor(() => expect(document.activeElement).toBe(imported));
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(server.asked.filter((ask) => ask.startsWith("POST"))).toEqual(["POST import Vault.zip under Guide"]);
  // The list is read again after the import started: its job is polled.
  expect(server.asked.filter((ask) => ask.startsWith("GET jobs")).length).toBeGreaterThan(1);
  expect(asksBeforeLeaving()).toBe(false);
});

test("the places offered are the root and the pages that can hold one more level, by their paths; no attachment", async () => {
  const user = userEvent.setup();
  const chain: TreeNode[] = [];
  for (let level = 1; level <= 10; level++) {
    chain.push(pageNode(40 + level, `L${level.toString()}`, chain.at(-1)));
  }
  renderApp(transfer, jobsServer({ nodes: [...chain, assetNode(60, "a.png")] }).app);

  const shown = await open(user);

  await waitFor(() =>
    expect(within(within(shown).getByLabelText("Import under")).getAllByRole("option").length).toBe(10)
  );
  const offered = within(within(shown).getByLabelText("Import under"))
    .getAllByRole("option")
    .map((option) => option.textContent);
  expect([offered[0], offered[1], offered.at(-1)]).toEqual([
    "The notebook's top level",
    "L1",
    "L1 / L2 / L3 / L4 / L5 / L6 / L7 / L8 / L9",
  ]);
});

test("a reader is told who imports, and has nothing to import with", async () => {
  renderApp(transfer, jobsServer({ role: "reader" }).app);

  expect(await screen.findByText("Only those who can edit this notebook import into it.")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Import a zip…" })).toBeNull();
});

test("nothing goes out without a file, or with one larger than the server takes; one not named .zip goes, said so", async () => {
  const user = userEvent.setup({ applyAccept: false });
  const server = jobsServer({
    answers: { "GET /api/v0/instance": () => json({ ...instanceJSON, import_max_bytes: 3 }) },
  });
  renderApp(transfer, server.app);
  const shown = await open(user);
  const file = within(shown).getByLabelText("Zip archive");

  await user.click(within(shown).getByRole("button", { name: "Import" }));
  expect(file.getAttribute("aria-invalid")).toBe("true");
  expect(within(shown).getByText("Required.")).toBeTruthy();
  await user.upload(file, new File(["1234"], "Vault.zip"));
  await user.click(within(shown).getByRole("button", { name: "Import" }));
  expect(within(shown).getByText("Larger than the server takes.")).toBeTruthy();
  expect(within(shown).getByText("The zip may be at most 3 B.")).toBeTruthy();
  expect(server.asked.filter((ask) => ask.startsWith("POST"))).toEqual([]);

  await user.upload(file, new File(["12"], "notes.tar"));
  expect(within(shown).getByText(/does not end with \.zip/u)).toBeTruthy();
  await user.click(within(shown).getByRole("button", { name: "Import" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(server.asked.filter((ask) => ask.startsWith("POST"))).toEqual(["POST import notes.tar under root"]);
});

test("an import of the notebook under way among the jobs is told, and none starts", async () => {
  const user = userEvent.setup();
  renderApp(transfer, jobsServer({ jobs: [underWay(jobJSON(1, { kind: "import", name: "Old.zip" }), 2, 9)] }).app);
  await row("Import of Old.zip");

  const shown = await open(user);
  expect(within(shown).getByRole("alert").textContent).toBe(
    "An import into this notebook is under way. Wait for it to end before starting another."
  );
  expect(within(shown).getByRole("button", { name: "Import" }).hasAttribute("disabled")).toBe(true);
});

test.each([
  [
    problem(409, "transfer.busy"),
    "An import into this notebook is under way, perhaps someone else's. Wait for it to end, then try again.",
  ],
  [problem(404, "page.not_found"), "The page to import under no longer exists."],
  [problem(503, "server_busy"), "The server is busy. Try again in a moment."],
  [problem(507, "storage_full"), "The server has no room for more files. Ask its administrator."],
  [problem(413, "payload_too_large"), "The request is too large."],
])("a refusal stays in the dialog, the file kept to try again: %#", async (answer, says) => {
  const user = userEvent.setup();
  const server = jobsServer({ answers: { [imports]: () => answer.clone() } });
  renderApp(transfer, server.app);
  const shown = await open(user);
  await user.upload(within(shown).getByLabelText("Zip archive"), vault());

  await user.click(within(shown).getByRole("button", { name: "Import" }));

  expect((await within(shown).findByRole("alert")).textContent).toBe(says);
  expect(within(shown).getByRole("button", { name: "Import" }).hasAttribute("disabled")).toBe(false);
  expect(asksBeforeLeaving()).toBe(false);
});

test("a connection cut, as a refusal before the file is read may reach the browser, says what may have happened", async () => {
  const user = userEvent.setup();
  renderApp(transfer, jobsServer({ answers: { [imports]: () => Promise.reject(new TypeError("reset")) } }).app);
  const shown = await open(user);
  await user.upload(within(shown).getByLabelText("Zip archive"), vault());

  await user.click(within(shown).getByRole("button", { name: "Import" }));

  expect((await within(shown).findByRole("alert")).textContent).toMatch(
    /^The upload did not finish: the connection was cut\./u
  );
});

test("the upload tells its progress; the browser asks before the page is left; closed, the dialog asks before it stops it", async () => {
  const user = userEvent.setup();
  const server = jobsServer({ answers: { [imports]: () => new Promise<Response>(() => undefined) } });
  renderApp(transfer, server.app);
  const shown = await open(user);
  await user.upload(within(shown).getByLabelText("Zip archive"), vault());
  await user.click(within(shown).getByRole("button", { name: "Import" }));

  await waitFor(() => expect(transfers(server.app).length).toBe(1));
  const [going] = transfers(server.app);
  going?.progress(1, 4);
  expect(await within(shown).findByRole("progressbar", { name: "Uploaded 1 B of 4 B" })).toBeTruthy();
  expect(within(shown).getByRole("button", { name: "Uploading…" }).hasAttribute("disabled")).toBe(true);
  expect(asksBeforeLeaving()).toBe(true);

  await user.keyboard("{Escape}");
  expect(within(shown).getByRole("alert").textContent).toMatch(/^Stop the upload\?/u);
  await user.click(within(shown).getByRole("button", { name: "Keep uploading" }));
  expect([screen.queryByRole("dialog") !== null, going?.aborted]).toEqual([true, false]);

  await user.keyboard("{Escape}");
  const ask = within(shown).getByRole("alert");
  await user.click(within(ask).getByRole("button", { name: "Stop the upload" }));

  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(going?.aborted).toBe(true);
  expect(asksBeforeLeaving()).toBe(false);
  expect(await screen.findByText("No jobs yet.")).toBeTruthy();
});

test("Stop the upload stops it, the dialog kept; leaving the page stops it too", async () => {
  const user = userEvent.setup();
  const server = jobsServer({ answers: { [imports]: () => new Promise<Response>(() => undefined) } });
  const { router } = renderApp(transfer, server.app);
  const shown = await open(user);
  await user.upload(within(shown).getByLabelText("Zip archive"), vault());
  await user.click(within(shown).getByRole("button", { name: "Import" }));
  await waitFor(() => expect(transfers(server.app).length).toBe(1));

  await user.click(within(shown).getByRole("button", { name: "Stop the upload" }));
  await waitFor(() => expect(transfers(server.app)[0]?.aborted).toBe(true));
  expect(within(shown).queryByRole("alert")).toBeNull();
  expect(within(shown).getByRole("button", { name: "Import" }).hasAttribute("disabled")).toBe(false);

  await user.click(within(shown).getByRole("button", { name: "Import" }));
  await waitFor(() => expect(transfers(server.app).length).toBe(2));
  await router.navigate(`/lab/notebooks/${notebookJSON.id}/settings/general`);

  await waitFor(() => expect(transfers(server.app)[1]?.aborted).toBe(true));
  expect([asksBeforeLeaving(), screen.queryByRole("dialog")]).toEqual([false, null]);
});

test("an import's report counts what it skipped, not missing files, and says what was not imported as it was", async () => {
  const user = userEvent.setup();
  const counts = { pages: 5, attachments: 3, renamed: 1, missing: 0, skipped: 2 };
  const job = jobJSON(1, { kind: "import", name: "Vault.zip", report: { failure: null, counts } });
  const server = jobsServer({ jobs: [job] });
  server.problems.set(job.id, {
    problems: [
      { path: "a:b.md", code: "renamed", to: "a_b" },
      { path: "../x.md", code: "unsafe_path", to: null },
      { path: "y", code: "gremlins" as "renamed", to: null },
    ],
    problems_truncated: false,
  });
  renderApp(transfer, server.app);

  await user.click(within(await row("Import of Vault.zip")).getByRole("button", { name: /^Report on / }));

  const shown = await screen.findByRole("list", { name: "What was not imported as it was" });
  expect(
    [...document.querySelectorAll("dt")].map((term) => `${term.textContent}: ${term.nextElementSibling?.textContent}`)
  ).toEqual(["Pages: 5", "Attachments: 3", "Renamed: 1", "Skipped: 2"]);
  expect(
    within(shown)
      .getAllByRole("listitem")
      .map((item) => item.textContent)
  ).toEqual([
    "a:b.md was imported as a_b: links to its old name do not reach it.",
    "../x.md was skipped: its path leads outside the zip.",
    "y was not imported as it was.",
  ]);
});

test("an import's failures say what befell the import", async () => {
  const failures: TransferFailure[] = ["forbidden", "root_not_found", "not_zip", "tree_changed"];
  const jobs = failures.map((failure, i) =>
    jobJSON(i + 1, {
      kind: "import",
      name: `V${i.toString()}.zip`,
      state: "failed",
      report: { ...jobJSON(1).report!, failure },
    })
  );
  renderApp(transfer, jobsServer({ jobs }).app);
  await list();

  expect(
    await Promise.all(
      jobs.map(async (each) => (await row(`Import of ${each.name}`)).querySelectorAll("p")[1]?.textContent)
    )
  ).toEqual([
    "Failed · The person who started it could no longer edit the notebook.",
    "Failed · The page imported under no longer exists.",
    "Failed · The file is not a zip archive, or the archive is damaged.",
    "Failed · A page the import had written was deleted or moved away while it ran.",
  ]);
});
