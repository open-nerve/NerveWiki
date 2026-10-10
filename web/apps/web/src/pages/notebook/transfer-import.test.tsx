import { act, configure, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, beforeAll, expect, test } from "vitest";

import { eventHandlers } from "../../events/handlers";
import { FakePage } from "../../events/testing/fake-page";
import type { TransferFailure } from "../../services/transfer.service";
import { transfers } from "../../test/attachments";
import { eventServer, withEvents } from "../../test/event-server";
import { instanceJSON, json, notebookJSON, problem, workspaceJSON } from "../../test/fakes";
import { jobJSON, jobsServer, underWay } from "../../test/jobs-server";
import { assetNode, guide, pageNode } from "../../test/page-server";
import type { TreeNode } from "../../services/page.service";
import { renderApp } from "../../test/render";

// A notebook's imports from its settings (M7/P6 design 4.2), in StrictMode as the app runs.

beforeAll(() => configure({ reactStrictMode: true }));
afterAll(() => configure({ reactStrictMode: false }));

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
  const server = jobsServer({ role: "editor", jobs: [jobJSON(1)] });
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
  expect(document.activeElement).toBe(within(shown).getByLabelText("Zip archive"));
  expect(within(shown).getByText("The zip may be at most 512 MB.")).toBeTruthy();
  expect(within(shown).getByText(/^Keep this dialog open while the zip uploads;/u)).toBeTruthy();
  expect(within(shown).getByLabelText("Zip archive").getAttribute("accept")).toBe(".zip,application/zip");
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

  // Cancelled later, the dialog gives the focus back to its button, not to the job started before.
  await user.click(within(await open(user)).getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("button", { name: "Import a zip…" })));
});

test("imported while the jobs cannot be read, the focus goes to their title", async () => {
  const user = userEvent.setup();
  const server = jobsServer();
  server.listDown = true;
  renderApp(transfer, server.app);
  await screen.findByRole("alert");

  const shown = await open(user);
  await user.upload(within(shown).getByLabelText("Zip archive"), vault());
  await user.click(within(shown).getByRole("button", { name: "Import" }));

  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("heading", { name: "Recent jobs" })));
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
  expect(document.activeElement).toBe(file);
  await user.upload(file, new File(["1234"], "Vault.zip"));
  await user.click(within(shown).getByRole("button", { name: "Import" }));
  expect(within(shown).getByText("Larger than the server takes.")).toBeTruthy();
  expect(within(shown).getByText("The zip may be at most 3 B.")).toBeTruthy();
  expect(server.asked.filter((ask) => ask.startsWith("POST"))).toEqual([]);

  await user.upload(file, new File(["123"], "VAULT.ZIP"));
  expect(within(shown).queryByText(/does not end with \.zip/u)).toBeNull();
  // A file as large as the server takes goes.
  await user.upload(file, new File(["123"], "notes.tar"));
  expect(within(shown).getByText(/does not end with \.zip/u)).toBeTruthy();
  await user.click(within(shown).getByRole("button", { name: "Import" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(server.asked.filter((ask) => ask.startsWith("POST"))).toEqual(["POST import notes.tar under root"]);
});

test("the instance not read, no size is told, and none is checked", async () => {
  const user = userEvent.setup();
  const server = jobsServer({ answers: { "GET /api/v0/instance": () => problem(404, "not_found") } });
  renderApp(transfer, server.app);
  const shown = await open(user);

  expect(within(shown).queryByText(/^The zip may be at most/u)).toBeNull();
  await user.upload(within(shown).getByLabelText("Zip archive"), vault());
  await user.click(within(shown).getByRole("button", { name: "Import" }));
  await waitFor(() =>
    expect(server.asked.filter((ask) => ask.startsWith("POST"))).toEqual(["POST import Vault.zip under root"])
  );
});

test("a page chosen that the tree, read again, no longer has is not imported under: it is chosen again", async () => {
  const user = userEvent.setup();
  const events = eventServer();
  const server = jobsServer({ answers: { "GET /api/v0/events": events.answer } });
  renderApp(transfer, withEvents(server.app, new FakePage()), { eventHandlers });
  const shown = await open(user);
  await user.upload(within(shown).getByLabelText("Zip archive"), vault());
  const place = within(shown).getByLabelText("Import under");
  const offered = () =>
    within(place)
      .getAllByRole("option")
      .map((option) => option.textContent);
  await waitFor(() => expect(offered()).toContain("Guide"));
  await user.selectOptions(place, guide.id);

  // Another deletes the page: its event reads the tree again.
  server.nodes = server.nodes.filter((node) => node.id !== guide.id && node.parent_id !== guide.id);
  await waitFor(() => expect(events.streams).toHaveLength(1));
  act(() => events.last().hello());
  act(() =>
    events.last().send("pages", { workspace_id: workspaceJSON.id, notebook_id: notebookJSON.id, tree: true, pages: [] })
  );
  await waitFor(() => expect(offered()).not.toContain("Guide"));
  expect((place as HTMLSelectElement).selectedOptions[0]?.textContent).toBe("Choose a place");
  await user.click(within(shown).getByRole("button", { name: "Import" }));

  expect(within(shown).getByText("The page chosen is no longer among the places: choose again.")).toBeTruthy();
  expect(place.getAttribute("aria-invalid")).toBe("true");
  expect(server.asked.filter((ask) => ask.startsWith("POST"))).toEqual([]);
  await user.selectOptions(place, "The notebook's top level");
  await user.click(within(shown).getByRole("button", { name: "Import" }));
  await waitFor(() =>
    expect(server.asked.filter((ask) => ask.startsWith("POST"))).toEqual(["POST import Vault.zip under root"])
  );
});

test("an import of the notebook under way among the jobs is told, and none starts", async () => {
  const user = userEvent.setup();
  const server = jobsServer({ jobs: [underWay(jobJSON(1, { kind: "import", name: "Old.zip" }), 2, 9)] });
  renderApp(transfer, server.app);
  await row("Import of Old.zip");

  const shown = await open(user);
  expect(within(shown).getByRole("alert").textContent).toBe(
    "An import into this notebook is under way. Wait for it to end before starting another."
  );
  const confirm = within(shown).getByRole("button", { name: "Import" });
  expect([confirm.getAttribute("aria-disabled"), confirm.hasAttribute("disabled")]).toEqual(["true", false]);
  await user.upload(within(shown).getByLabelText("Zip archive"), vault());
  await user.click(confirm);
  expect(transfers(server.app)).toEqual([]);
});

test.each([
  [
    problem(409, "transfer.busy"),
    "An import into this notebook is under way, perhaps someone else's. Wait for it to end, then try again.",
  ],
  [problem(404, "page.not_found"), "The page to import under no longer exists."],
  [problem(503, "server_busy"), "Too many jobs are waiting on the server. Try again in a few minutes."],
  [problem(507, "storage_full"), "The server has no room for more files. Ask its administrator."],
  [problem(413, "payload_too_large"), "The zip is larger than the server takes."],
  [
    problem(400, "bad_request"),
    "The upload did not arrive whole, or arrived too slowly. Try again, on a faster connection if you can.",
  ],
  [problem(404, "notebook.not_found"), "This notebook does not exist, or you have no access to it."],
  [problem(403, "forbidden"), "You do not have permission to do this."],
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
  expect(within(shown).queryByRole("progressbar")).toBeNull();

  // The file kept, it goes again as it is; refused, the dialog closes without asking.
  await user.click(within(shown).getByRole("button", { name: "Import" }));
  await waitFor(() => expect(transfers(server.app).length).toBe(2));
  await waitFor(() =>
    expect(within(shown).getByRole("button", { name: "Import" }).hasAttribute("disabled")).toBe(false)
  );
  await user.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
});

test("a connection cut, as a refusal before the file is read may reach the browser, says what may have happened", async () => {
  const user = userEvent.setup();
  const server = jobsServer({ answers: { [imports]: () => Promise.reject(new TypeError("reset")) } });
  renderApp(transfer, server.app);
  const shown = await open(user);
  await user.upload(within(shown).getByLabelText("Zip archive"), vault());

  await user.click(within(shown).getByRole("button", { name: "Import" }));

  expect((await within(shown).findByRole("alert")).textContent).toMatch(
    /^The upload did not finish: the connection was cut\./u
  );
  expect(transfers(server.app).length).toBe(1);
  expect(within(shown).getByRole("button", { name: "Import" }).hasAttribute("disabled")).toBe(false);
  expect(asksBeforeLeaving()).toBe(false);
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
  expect(document.activeElement?.textContent).toBe("Stop the upload");
  going?.progress(1, 4);
  const bar = (await within(shown).findByRole("progressbar", { name: "Uploaded 1 B of 4 B" })) as HTMLProgressElement;
  expect([bar.value, bar.max]).toEqual([1, 4]);
  expect(within(shown).getByRole("button", { name: "Uploading…" }).hasAttribute("disabled")).toBe(true);
  expect(asksBeforeLeaving()).toBe(true);

  await user.keyboard("{Escape}");
  expect(within(shown).getByRole("alert").textContent).toMatch(/^Stop the upload\?/u);
  expect(document.activeElement?.textContent).toBe("Keep uploading");
  await user.click(within(shown).getByRole("button", { name: "Keep uploading" }));
  expect([screen.queryByRole("dialog") !== null, within(shown).queryByRole("alert"), going?.aborted]).toEqual([
    true,
    null,
    false,
  ]);
  expect(document.activeElement?.textContent).toBe("Stop the upload");
  await user.keyboard("{Escape}");
  await user.keyboard("{Escape}");
  expect([within(shown).queryByRole("alert"), going?.aborted]).toEqual([null, false]);

  await user.keyboard("{Escape}");
  const ask = within(shown).getByRole("alert");
  await user.click(within(ask).getByRole("button", { name: "Stop and close" }));

  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(going?.aborted).toBe(true);
  expect(asksBeforeLeaving()).toBe(false);
  expect(await screen.findByText("No jobs yet.")).toBeTruthy();
});

test("Stop the upload stops it, the dialog kept; leaving the page asks first, and stops it", async () => {
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
  expect(within(shown).queryByRole("progressbar")).toBeNull();
  expect(within(shown).getByRole("button", { name: "Import" }).hasAttribute("disabled")).toBe(false);

  await user.click(within(shown).getByRole("button", { name: "Import" }));
  await waitFor(() => expect(transfers(server.app).length).toBe(2));
  const general = `/lab/notebooks/${notebookJSON.id}/settings/general`;
  await router.navigate(general);
  await user.click(within(await within(shown).findByRole("alert")).getByRole("button", { name: "Keep uploading" }));
  expect([router.state.location.pathname === general, transfers(server.app)[1]?.aborted]).toEqual([false, false]);
  expect(within(shown).queryByRole("alert")).toBeNull();

  await router.navigate(general);
  await user.click(within(await within(shown).findByRole("alert")).getByRole("button", { name: "Stop and leave" }));
  await waitFor(() => expect(router.state.location.pathname).toBe(general));
  expect(transfers(server.app)[1]?.aborted).toBe(true);
  await waitFor(() => expect([asksBeforeLeaving(), screen.queryByRole("dialog")]).toEqual([false, null]));
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
    "Failed · A page the import had written was deleted while it ran.",
  ]);
});

const counts = (pages: number) => ({ pages, attachments: 0, renamed: 0, missing: 0, skipped: 0 });

test("an import that did not succeed says that what it counts stays in the notebook; one that did, or wrote nothing, does not", async () => {
  const user = userEvent.setup();
  const jobs = [
    jobJSON(1, {
      kind: "import",
      name: "A.zip",
      state: "failed",
      report: { failure: "tree_changed", counts: counts(2) },
    }),
    jobJSON(2, { kind: "import", name: "B.zip", report: { failure: null, counts: counts(2) } }),
    jobJSON(3, { kind: "import", name: "C.zip", state: "failed", report: { failure: "not_zip", counts: counts(0) } }),
  ];
  renderApp(transfer, jobsServer({ jobs }).app);

  async function keeps(name: string) {
    const report = within(await row(`Import of ${name}`)).getByRole("button", { name: /^Report on / });
    await user.click(report);
    await waitFor(() => expect(screen.queryAllByText("Pages").length).toBeGreaterThan(0));
    const said = screen.queryByText("The pages and attachments counted here stay in the notebook.") !== null;
    await user.click(report);
    return said;
  }
  expect([await keeps("A.zip"), await keeps("B.zip"), await keeps("C.zip")]).toEqual([true, false, false]);
});

test("pages that cannot be read leave the root to import at, and are read again on asking", async () => {
  const user = userEvent.setup();
  let reads = 0;
  const server = jobsServer({
    answers: {
      [`GET /api/v0/notebooks/${notebookJSON.id}/nodes`]: () => {
        reads += 1;
        return problem(500, "internal");
      },
    },
  });
  renderApp(transfer, server.app);
  const shown = await open(user);

  const retry = await within(shown).findByRole("button", { name: "Try again" });
  const place = within(shown).getByLabelText("Import under");
  expect([...place.querySelectorAll("option")].map((option) => option.textContent)).toEqual([
    "The notebook's top level",
  ]);
  const before = reads;
  await user.click(retry);
  await waitFor(() => expect(reads).toBeGreaterThan(before));
});

test("an upload that ends while the dialog asks is asked about no more; one that ends as the page is left lets it go", async () => {
  const user = userEvent.setup();
  const answers: ((response: Response) => void)[] = [];
  const server = jobsServer({
    answers: {
      [imports]: () =>
        new Promise<Response>((resolve) => {
          answers.push(resolve);
        }),
    },
  });
  const { router } = renderApp(transfer, server.app);
  const shown = await open(user);
  await user.upload(within(shown).getByLabelText("Zip archive"), vault());
  await user.click(within(shown).getByRole("button", { name: "Import" }));
  await waitFor(() => expect(answers.length).toBe(1));

  await user.keyboard("{Escape}");
  expect(within(shown).getByRole("button", { name: "Keep uploading" })).toBeTruthy();
  answers[0]?.(problem(409, "transfer.busy"));
  await waitFor(() => expect(within(shown).queryByRole("button", { name: "Keep uploading" })).toBeNull());
  expect(screen.queryByRole("dialog")).not.toBeNull();

  await user.click(within(shown).getByRole("button", { name: "Import" }));
  await waitFor(() => expect(answers.length).toBe(2));
  // The next upload does not find the question asked.
  expect(within(shown).queryByRole("button", { name: "Keep uploading" })).toBeNull();
  expect(document.activeElement?.textContent).toBe("Stop the upload");
  const general = `/lab/notebooks/${notebookJSON.id}/settings/general`;
  await router.navigate(general);
  await within(shown).findByRole("button", { name: "Stop and leave" });
  answers[1]?.(problem(409, "transfer.busy"));
  await waitFor(() => expect(router.state.location.pathname).toBe(general));
});

test("signed out elsewhere as the zip uploads, the upload stops with the account's pages, and nothing is read for them", async () => {
  const user = userEvent.setup();
  const server = jobsServer({ answers: { [imports]: () => new Promise<Response>(() => undefined) } });
  const { app } = renderApp(transfer, server.app);
  const shown = await open(user);
  await user.upload(within(shown).getByLabelText("Zip archive"), vault());
  await user.click(within(shown).getByRole("button", { name: "Import" }));
  await waitFor(() => expect(transfers(server.app).length).toBe(1));

  await act(() => app.session.tokens.signOut());

  await waitFor(() => expect(transfers(server.app)[0]?.aborted).toBe(true));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(asksBeforeLeaving()).toBe(false);
});

type User = ReturnType<typeof userEvent.setup>;
const held = (): Promise<Response> => new Promise<Response>(() => undefined);

test.each([
  ["refused", (): Promise<Response> => Promise.resolve(problem(409, "transfer.busy")), async () => undefined],
  [
    "stopped",
    held,
    async (user: User, shown: HTMLElement) => {
      await user.click(within(shown).getByRole("button", { name: "Stop the upload" }));
    },
  ],
  [
    "stopped and closed",
    held,
    async (user: User, shown: HTMLElement) => {
      await user.keyboard("{Escape}");
      await user.click(within(shown).getByRole("button", { name: "Stop and close" }));
    },
  ],
] as const)("an upload %s reads the jobs again, none under way to be polled", async (how, answer, end) => {
  const user = userEvent.setup();
  const server = jobsServer({ answers: { [imports]: answer } });
  renderApp(transfer, server.app);
  expect(await screen.findByText("No jobs yet.")).toBeTruthy();
  const shown = await open(user);
  await user.upload(within(shown).getByLabelText("Zip archive"), vault());
  const reads = () => server.asked.filter((ask) => ask.startsWith("GET jobs")).length;
  const before = reads();

  await user.click(within(shown).getByRole("button", { name: "Import" }));
  await waitFor(() => expect(transfers(server.app).length).toBe(1));
  await end(user, shown);

  await waitFor(() => expect(reads()).toBe(before + 1));
  if (how === "stopped and closed") {
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  } else {
    // The dialog kept, the focus goes to Import, to try again: not to Cancel, where Stop the upload was.
    await waitFor(() => expect(document.activeElement).toBe(within(shown).getByRole("button", { name: "Import" })));
  }
});

test("refused, as the jobs read again show an import under way, the focus stays in the dialog, on Import, which cannot go", async () => {
  const user = userEvent.setup();
  const answers: ((response: Response) => void)[] = [];
  const server = jobsServer({
    answers: {
      [imports]: () =>
        new Promise<Response>((resolve) => {
          answers.push(resolve);
        }),
    },
  });
  renderApp(transfer, server.app);
  expect(await screen.findByText("No jobs yet.")).toBeTruthy();
  const shown = await open(user);
  await user.upload(within(shown).getByLabelText("Zip archive"), vault());
  await user.click(within(shown).getByRole("button", { name: "Import" }));
  await waitFor(() => expect(answers.length).toBe(1));

  // Another's import, started meanwhile, refuses this one; the jobs read again show it.
  server.jobs = [underWay(jobJSON(9, { kind: "import", name: "Other.zip" }), 0, 0)];
  answers[0]?.(problem(409, "transfer.busy"));

  const confirm = await within(shown).findByRole("button", { name: "Import" });
  await waitFor(() => expect(confirm.getAttribute("aria-disabled")).toBe("true"));
  expect(document.activeElement).toBe(confirm);
  await user.tab();
  expect(shown.contains(document.activeElement)).toBe(true);
});

test("a focus the user put in the form as the upload went is kept as it ends", async () => {
  const user = userEvent.setup();
  const answers: ((response: Response) => void)[] = [];
  const server = jobsServer({
    answers: {
      [`GET /api/v0/notebooks/${notebookJSON.id}/nodes`]: () => problem(500, "internal"),
      [imports]: () =>
        new Promise<Response>((resolve) => {
          answers.push(resolve);
        }),
    },
  });
  renderApp(transfer, server.app);
  const shown = await open(user);
  const retry = await within(shown).findByRole("button", { name: "Try again" });
  await user.upload(within(shown).getByLabelText("Zip archive"), vault());
  await user.click(within(shown).getByRole("button", { name: "Import" }));
  await waitFor(() => expect(answers.length).toBe(1));

  act(() => retry.focus());
  answers[0]?.(problem(409, "transfer.busy"));

  await waitFor(() =>
    expect(within(shown).getByRole("button", { name: "Import" }).hasAttribute("disabled")).toBe(false)
  );
  expect(document.activeElement).toBe(retry);
});
