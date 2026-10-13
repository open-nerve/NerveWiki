import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, onTestFinished, test, vi } from "vitest";

import type { NotebookRole } from "../../services/notebook.service";
import type { RevertConflict } from "../../services/page.service";
import { json, notebookJSON, problem, type Answer } from "../../test/fakes";
import { changesetId, moved, renamed, wrote } from "../../test/history-server";
import { ada, bob, guide, install, linux, notes, pageNode, pagePath, pageServer } from "../../test/page-server";
import { readAgain } from "../../test/page-panel";
import { renderApp } from "../../test/render";

// A changeset's Undo change (M8 design 4.8; M8/P3 design 4.4).

const home = `/lab/notebooks/${notebookJSON.id}`;

afterEach(() => {
  vi.useRealTimers();
});
const changesetPath = (id: string) => `${home}/history/${id}`;

const old = pageNode(7, "Old");

/**
 * reworked is pageServer with Ada's rework in its history: it renamed
 * Notes from Old notes, and wrote Guide; undone, Notes is Old notes again.
 * Old is in the trash.
 */
function reworked(role: NotebookRole = "admin", answers: Record<string, Answer> = {}) {
  const server = pageServer({ role, answers, nodes: [guide, install, linux, notes, old] });
  server.trashNode(old.id);
  const rework = server.log("rework", {
    at: "2026-10-05T08:01:00Z",
    changes: [renamed(notes, "Old notes"), wrote(guide, 1, 2)],
    undo: () => {
      server.nodes = server.nodes.map((node) => (node.id === notes.id ? { ...node, name: "Old notes" } : node));
    },
  });
  return { server, rework };
}

/** opened opens the dialog of Undo change, once the changeset's page shows: it answers the dialog. */
async function opened(user: ReturnType<typeof userEvent.setup>): Promise<HTMLElement> {
  await user.click(await screen.findByRole("button", { name: "Undo change" }));
  return screen.findByRole("alertdialog", { name: "Undo this change?" });
}

/** title is the changeset's page's title. */
const title = () => screen.getByRole("heading", { level: 1, name: "Edit · Ada" });

/** reverts are the reverts that went out. */
const reverts = (sent: string[]) => sent.filter((each) => each.startsWith("REVERT"));

test("Undo change asks first: what the undo does, with its counts, that links written since stay; with a message, sent as written", async () => {
  const user = userEvent.setup();
  const { server, rework } = reworked();
  renderApp(changesetPath(rework.id), server.app);

  const dialog = await opened(user);

  expect(dialog.getAttribute("aria-describedby")).not.toBeNull();
  expect(document.getElementById(dialog.getAttribute("aria-describedby") ?? "")?.textContent).toBe(
    "Nodes: 1, contents: 1. They go back to how they were before this change, as a new change of its own, an undo. Whatever changed since, or cannot go back, is listed instead, and nothing changes."
  );
  expect(within(dialog).getByText("Links written since to the new names or places are not changed back.")).toBeTruthy();
  await user.type(within(dialog).getByRole("textbox", { name: "Message (optional)" }), "  Back to <old>  ");
  await user.click(within(dialog).getByRole("button", { name: "Undo" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(reverts(server.sent)).toEqual(['REVERT rework "Back to <old>"']);
  expect(server.changesets.at(-1)?.message).toBe("Back to <old>");
});

test("an undo done closes the dialog: the page says so, leading to the undo, the title taking the focus, described by it; the changeset, its nodes and the tree are read again", async () => {
  const user = userEvent.setup();
  const { server, rework } = reworked();
  renderApp(changesetPath(rework.id), server.app);
  const tree = await screen.findByRole("navigation", { name: "Pages of Plans" });
  await screen.findByRole("list", { name: "Nodes of this change" });
  const dialog = await opened(user);
  const before = server.sent.length;

  await user.click(within(dialog).getByRole("button", { name: "Undo" }));

  const status = screen.getByRole("status");
  await waitFor(() => expect(status.textContent).toBe("Undone. See the undo"));
  const undo = server.changesets.at(-1);
  expect(undo?.label).toBe("revert of rework");
  expect(within(status).getByRole("link", { name: "See the undo" }).getAttribute("href")).toBe(
    changesetPath(undo?.id ?? "")
  );
  expect(document.activeElement).toBe(title());
  expect(title().getAttribute("aria-describedby")).toBe(status.id);
  // Its body is sent as is: no message.
  expect(server.sent.slice(before)).toEqual(
    expect.arrayContaining(["REVERT rework", "GET changeset rework", "GET changes rework", "GET nodes"])
  );
  expect(server.sent[before]).toBe("REVERT rework");
  expect(await within(tree).findByRole("link", { name: "Old notes" })).toBeTruthy();
  await screen.findByText("Undone by:");
  expect(screen.getByRole("link", { name: `Undo change, ID ${(undo?.id ?? "").slice(-6)}` })).toBeTruthy();
  expect(screen.getByText("Undone before: undoing it again succeeds only once that undo is undone.")).toBeTruthy();
});

test("a message of spaces alone is none", async () => {
  const user = userEvent.setup();
  const { server, rework } = reworked();
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await opened(user);

  await user.type(within(dialog).getByRole("textbox", { name: "Message (optional)" }), "   ");
  await user.click(within(dialog).getByRole("button", { name: "Undo" }));

  await waitFor(() => expect(reverts(server.sent)).toEqual(["REVERT rework"]));
});

test.each([
  { what: "only wrote", changes: [wrote(guide, 1, 2)], note: false },
  { what: "moved a node", changes: [moved(linux, notes.id)], note: true },
  { what: "renamed a node", changes: [renamed(notes, "Old notes")], note: true },
])("the dialog of a changeset that $what says links written since stay: $note", async ({ changes, note }) => {
  const user = userEvent.setup();
  const server = pageServer();
  const changed = server.log("changed", { changes });
  renderApp(changesetPath(changed.id), server.app);

  const dialog = await opened(user);

  expect(
    within(dialog).queryByText("Links written since to the new names or places are not changed back.") !== null
  ).toBe(note);
});

test("an undo of nothing to undo says so as the dialog closes, the focus back on Undo change", async () => {
  const user = userEvent.setup();
  const { server, rework } = reworked();
  rework.unchanged = true;
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await opened(user);
  const logged = server.changesets.length;

  await user.click(within(dialog).getByRole("button", { name: "Undo" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(screen.getByRole("status").textContent).toBe("Nothing to undo: every node is as it was before this change.");
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "Undo change" }));
  expect(server.changesets).toHaveLength(logged);
});

/** conflict is a node of name in the way of a revert, for reason, another node holding its name. */
const conflict = (node: string, name: string, reason: RevertConflict["reason"], holder?: string): RevertConflict =>
  holder === undefined ? { node_id: node, name, reason } : { node_id: node, name, reason, holder_id: holder };

/** links are the links of item, each by its text, its name and where it leads. */
const links = (item: HTMLElement | undefined) =>
  within(item as HTMLElement)
    .queryAllByRole("link")
    .map((link) => [link.textContent, link.getAttribute("aria-label"), link.getAttribute("href")]);

test("conflicts stay in the dialog: how many, their title taking the focus; each node, why, and what to do there; renamed, it can be renamed back; Close gives the focus back to Undo change, nothing undone", async () => {
  const user = userEvent.setup();
  const { server, rework } = reworked();
  rework.conflicts = [
    conflict(notes.id, "Notes", "changed"),
    conflict(guide.id, "Guide", "children_added"),
    conflict(linux.id, "Linux", "parent_gone"),
    conflict(install.id, "Install", "title_taken", guide.id),
    conflict(old.id, "Old", "title_taken", notes.id),
    conflict(pageNode(30, "Loop").id, "Loop", "cycle"),
    conflict(pageNode(31, "Deep").id, "Deep", "too_deep"),
    conflict(pageNode(32, "Gone").id, "Gone", "expired"),
  ];
  renderApp(changesetPath(rework.id), server.app);
  await screen.findByRole("navigation", { name: "Pages of Plans" });
  const dialog = await opened(user);
  const logged = server.changesets.length;

  await user.click(within(dialog).getByRole("button", { name: "Undo" }));

  const heading = await within(dialog).findByRole("heading", {
    level: 3,
    name: "Not undone. Nodes changed since, or that cannot go back: 8",
  });
  expect(document.activeElement).toBe(heading);
  expect(dialog.getAttribute("aria-describedby")).toBe(heading.id);
  expect(within(dialog).queryByText(/are listed/)).toBeNull();
  const items = within(within(dialog).getByRole("list", { name: "Nodes in the way" })).getAllByRole("listitem");
  expect(items.map((item) => item.querySelector("p")?.textContent)).toEqual([
    "“Notes” changed since.",
    "“Guide” was put there since, and would go with the undo: move it away first.",
    "The page “Linux” was under is gone: in the trash, or purged.",
    "Where “Install” goes back, a page or attachment has its name: rename that one first.",
    "Where “Old” goes back, a page or attachment has its name: rename that one first.",
    "“Loop” would go back under itself: the page it was under is below it now.",
    "“Deep” would nest more than 10 levels deep where it goes back.",
    "“Gone” is past its time in the trash.",
  ]);
  expect(items.map(links)).toEqual([
    [["History of this node", "History of this node: “Notes”", `${home}/history?page=${notes.id}`]],
    [["Open it", "Open it: “Guide”", pagePath(guide.id)]],
    [["Trash", "Trash: the page “Linux” was under may be there", `${home}/trash`]],
    [["Open the one with this name", "Open the one with this name: “Install”", pagePath(guide.id)]],
    [
      ["Open the one with this name", "Open the one with this name: “Old”", pagePath(notes.id)],
      [
        "Or restore it from the trash (a restore numbers its name)",
        "Or restore it from the trash (a restore numbers its name): “Old”",
        `${home}/trash/${old.id}`,
      ],
    ],
    [],
    [],
    [],
  ]);
  expect(
    within(dialog).getByText(
      "You can also rename it back in the page tree: an ordinary rename, which writes the links again."
    )
  ).toBeTruthy();
  expect(server.changesets).toHaveLength(logged);

  await user.click(within(dialog).getByRole("button", { name: "Close" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "Undo change" }));
  expect(screen.getByRole("status").textContent).toBe("");
  // Opened again, it asks again.
  expect(within(await opened(user)).getByRole("button", { name: "Undo" })).toBeTruthy();
});

test("more conflicts than the hundred listed say how many, and that the first hundred are listed; a changeset that renamed nothing has no rename back", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  const write = server.log("write", { changes: [wrote(guide, 1, 2)] });
  write.conflicts = Array.from({ length: 101 }, (_, n) =>
    conflict(pageNode(200 + n, `Page ${n.toString()}`).id, `Page ${n.toString()}`, "changed")
  );
  renderApp(changesetPath(write.id), server.app);
  const dialog = await opened(user);

  await user.click(within(dialog).getByRole("button", { name: "Undo" }));

  expect(
    await within(dialog).findByRole("heading", {
      level: 3,
      name: "Not undone. Nodes changed since, or that cannot go back: 101",
    })
  ).toBeTruthy();
  expect(within(dialog).getByText("The first 100 are listed.")).toBeTruthy();
  expect(within(within(dialog).getByRole("list", { name: "Nodes in the way" })).getAllByRole("listitem")).toHaveLength(
    100
  );
  expect(within(dialog).queryByText(/rename it back/)).toBeNull();
});

test("a changeset the purge trimmed meanwhile says so as the dialog closes, the title taking the focus, described by it; read again, Undo change goes", async () => {
  const user = userEvent.setup();
  const { server, rework } = reworked();
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await opened(user);
  rework.trimmed = true;

  await user.click(within(dialog).getByRole("button", { name: "Undo" }));

  const alert = await screen.findByRole("alert");
  expect(alert.textContent).toBe("Part of this change has been purged from the trash: it can no longer be reverted.");
  await waitFor(() => expect(screen.queryByRole("button", { name: "Undo change" })).toBeNull());
  expect(
    screen.getByText(
      "Some of the pages this change touched have been purged from the trash: it can no longer be undone as a whole."
    )
  ).toBeTruthy();
  expect(document.activeElement).toBe(title());
  expect(title().getAttribute("aria-describedby")).toBe(alert.id);
});

test.each<[string, (rework: ReturnType<typeof reworked>["rework"]) => void, string]>([
  [
    "a page someone edits names them and the page",
    (rework) => {
      rework.lockedBy = { page: guide.id, holder: bob };
    },
    "Bob is editing “Guide”.",
  ],
  [
    "a changeset too large says what is, and how many nodes it touched, a thousand as many or more",
    (rework) => {
      rework.tooLarge = true;
      rework.counts = { nodes: 1000, contents: 3 };
    },
    "This change is too large to undo here: its nodes, pages, content, links or those pages' history are more than one undo does. Nodes it touched: 1,000+.",
  ],
])("an undo refused as %s, in the dialog, which stays", async (_, refuse, text) => {
  const user = userEvent.setup();
  const { server, rework } = reworked();
  refuse(rework);
  renderApp(changesetPath(rework.id), server.app);
  await screen.findByRole("navigation", { name: "Pages of Plans" });
  const dialog = await opened(user);

  await user.click(within(dialog).getByRole("button", { name: "Undo" }));

  expect((await within(dialog).findByRole("alert")).textContent).toBe(text);
  expect(screen.getByRole("alertdialog")).toBe(dialog);
});

test("a server too busy says so in the dialog, which stays: Undo again undoes", async () => {
  const user = userEvent.setup();
  const { server, rework } = reworked();
  server.revertsBusy = true;
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await opened(user);

  await user.click(within(dialog).getByRole("button", { name: "Undo" }));

  expect((await within(dialog).findByRole("alert")).textContent).toBe("The server is busy. Try again in a moment.");
  server.revertsBusy = false;
  await user.click(within(dialog).getByRole("button", { name: "Undo" }));
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Undone. See the undo"));
  expect(reverts(server.sent)).toEqual(["REVERT rework", "REVERT rework"]);
});

test("a message is held to its bytes, and to no NUL, before it is sent: the problem under it, which takes the focus; the server's under it too", async () => {
  const user = userEvent.setup();
  let refuse = false;
  const { server, rework } = reworked("admin", {
    "POST /api/v0/changesets/*/revert": () => {
      server.sent.push("REVERT refused");
      return refuse
        ? problem(422, "validation_failed", { errors: [{ field: "message", code: "too_long", message: "" }] })
        : json({ changeset_id: null, changed: false });
    },
  });
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await opened(user);
  const field = within(dialog).getByRole("textbox", { name: "Message (optional)" });
  const undo = within(dialog).getByRole("button", { name: "Undo" });

  // 2,048 characters of two bytes each and one of one: 2,049 characters, one byte too many.
  fireEvent.change(field, { target: { value: `${"é".repeat(2048)}a` } });
  await user.click(undo);
  await waitFor(() => expect(field.getAttribute("aria-invalid")).toBe("true"));
  expect(document.getElementById(field.getAttribute("aria-describedby") ?? "")?.textContent).toBe(
    "At most 4,096 bytes: shorten it."
  );
  expect(document.activeElement).toBe(field);

  fireEvent.change(field, { target: { value: `a${String.fromCharCode(0)}b` } });
  await user.click(undo);
  await waitFor(() =>
    expect(document.getElementById(field.getAttribute("aria-describedby") ?? "")?.textContent).toBe(
      "It may not hold a NUL character."
    )
  );
  expect(reverts(server.sent)).toEqual([]);

  refuse = true;
  fireEvent.change(field, { target: { value: "é".repeat(2048) } });
  await user.click(undo);
  await waitFor(() =>
    expect(document.getElementById(field.getAttribute("aria-describedby") ?? "")?.textContent).toBe(
      "At most 4,096 bytes: shorten it."
    )
  );
  expect(reverts(server.sent)).toEqual(["REVERT refused"]);
  expect(within(dialog).queryByRole("alert")).toBeNull();
});

test("Undo pressed twice sends one undo; nothing closes the dialog while it is out", async () => {
  const user = userEvent.setup();
  const { server, rework } = reworked();
  server.revertsHeld = true;
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await opened(user);
  const undo = within(dialog).getByRole("button", { name: "Undo" });

  await user.dblClick(undo);
  await user.click(undo);
  await waitFor(() => expect(reverts(server.sent)).toEqual(["REVERT rework"]));
  expect(within(dialog).getByRole("button", { name: "Undoing…" }).hasAttribute("disabled")).toBe(true);
  expect(within(dialog).getByRole("button", { name: "Cancel" }).hasAttribute("disabled")).toBe(true);
  await user.keyboard("{Escape}");
  expect(screen.getByRole("alertdialog")).toBe(dialog);

  act(() => server.release());
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Undone. See the undo"));
  expect(reverts(server.sent)).toEqual(["REVERT rework"]);
});

test("an undo answered after the reader scrolled leaves the focus where the dialog gives it back: they did something", async () => {
  const user = userEvent.setup();
  const { server, rework } = reworked();
  server.revertsHeld = true;
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await opened(user);
  await user.click(within(dialog).getByRole("button", { name: "Undo" }));
  await waitFor(() => expect(reverts(server.sent)).toEqual(["REVERT rework"]));

  act(() => void window.dispatchEvent(new WheelEvent("wheel")));
  act(() => server.release());

  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Undone. See the undo"));
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "Undo change" }));
});

test("an undo refused, the role taken away meanwhile, says why as the dialog closes, and reads the notebooks again: a reader by now, Undo change goes, the focus to the title", async () => {
  const user = userEvent.setup();
  let role: NotebookRole = "admin";
  const reads: string[] = [];
  const { server, rework } = reworked("admin", {
    "GET /api/v0/workspaces/lab/notebooks": () => {
      reads.push("GET notebooks");
      return json({ data: [{ ...notebookJSON, role }] });
    },
    "POST /api/v0/changesets/*/revert": () => problem(403, "forbidden"),
  });
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await opened(user);
  role = "reader";
  reads.length = 0;

  await user.click(within(dialog).getByRole("button", { name: "Undo" }));

  const alert = await screen.findByRole("alert");
  expect(alert.textContent).toBe("You do not have permission to do this.");
  await waitFor(() => expect(screen.queryByRole("button", { name: "Undo change" })).toBeNull());
  expect(screen.getByText("Editors and admins can undo this change.")).toBeTruthy();
  expect(reads).toEqual(["GET notebooks"]);
  expect(document.activeElement).toBe(title());
  expect(title().getAttribute("aria-describedby")).toBe(alert.id);
});

test("an undo refused as not found closes the dialog, and reads the changeset and the notebooks again: gone, it is no page", async () => {
  const user = userEvent.setup();
  const reads: string[] = [];
  const { server, rework } = reworked("admin", {
    "GET /api/v0/workspaces/lab/notebooks": () => {
      reads.push("GET notebooks");
      return json({ data: [notebookJSON] });
    },
  });
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await opened(user);
  server.changesets = server.changesets.filter((each) => each !== rework);
  reads.length = 0;

  await user.click(within(dialog).getByRole("button", { name: "Undo" }));

  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeTruthy();
  expect(reads).toEqual(["GET notebooks"]);
  expect(server.sent.filter((each) => each === `GET changeset ${rework.id}`)).toHaveLength(1);
});

/** Ada's undo before this one: the newest known as the undo is sent. */
const earlier = "earlier";

/**
 * lost is reworked, undone before by Ada, a hundred times in all, whose
 * next undo's answer is lost, as answer says (a 500, or no answer), once
 * the server did what does.
 */
function lost(answer: () => Response | Promise<Response>, does: (server: LostServer) => void) {
  const { server, rework } = reworked("admin", {
    "POST /api/v0/changesets/*/revert": () => {
      server.sent.push("REVERT rework");
      does(server);
      return answer();
    },
  });
  const before = server.log(earlier, { kind: "revert", reverts: rework.id });
  rework.revertedBy = [before.id, ...Array.from({ length: 99 }, (_, n) => changesetId(8000 + n))];
  return { server, rework };
}

type LostServer = ReturnType<typeof reworked>["server"];

/** undoneBy has who undo rework again, its newest undo: the list of them stays a hundred long. */
const undoneBy = (by: typeof bob) => (server: LostServer) => {
  const rework = server.changesets.find((each) => each.label === "rework");
  const undo = server.log("lost undo", { kind: "revert", reverts: rework?.id ?? "", by });
  if (rework !== undefined) {
    rework.revertedBy = [undo.id, ...rework.revertedBy].slice(0, 100);
  }
};

const answersLost = [
  { how: "a 500", answer: () => problem(500, "internal_error") },
  { how: "no answer", answer: () => Promise.reject(new TypeError("offline")) },
];
const outcomes = [
  { whose: "the account's", does: undoneBy(ada), text: "Undone. See the undo" },
  { whose: "another's", does: undoneBy(bob), text: "This change has been undone by Bob." },
  { whose: "the one known as it was sent", does: () => undefined, text: "It was not undone: you can try again." },
];

test.each(
  answersLost.flatMap(({ how, answer }) =>
    outcomes.map(({ whose, does, text }) => ({ how, answer, whose, does, text }))
  )
)(
  "an undo whose answer was lost ($how) reads the changeset again: its newest undo $whose says $text",
  async ({ answer, does, text }) => {
    const user = userEvent.setup();
    const { server, rework } = lost(answer, does);
    renderApp(changesetPath(rework.id), server.app);
    const dialog = await opened(user);
    const before = server.sent.length;

    await user.click(within(dialog).getByRole("button", { name: "Undo" }));

    const status = screen.getByRole("status");
    await waitFor(() => expect(status.textContent).toBe(text));
    expect(document.activeElement).toBe(title());
    expect(title().getAttribute("aria-describedby")).toBe(status.id);
    expect(server.sent.slice(before)).toContain("GET changeset rework");
    if (text.startsWith("Undone")) {
      const undo = server.changesets.find((each) => each.label === "lost undo");
      expect(within(status).getByRole("link", { name: "See the undo" }).getAttribute("href")).toBe(
        changesetPath(undo?.id ?? "")
      );
    }
  }
);

test("an undo whose answer was lost, its changeset not read again, says the outcome is not known", async () => {
  const user = userEvent.setup();
  const { server, rework } = lost(
    () => problem(500, "internal_error"),
    (state) => {
      state.historyDown = true;
    }
  );
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await opened(user);

  await user.click(within(dialog).getByRole("button", { name: "Undo" }));

  await waitFor(() =>
    expect(screen.getByRole("status").textContent).toBe(
      "Whether it was undone is not known: this change could not be read again. Reload the page to see."
    )
  );
});

// M8/P1 fix check 3, R3-1: the session left as the answer's body is read, its headers past the session's check.
test.each([
  { answer: "undone", status: 200, body: { changeset_id: changesetId(7000), changed: true } },
  { answer: "refused, the role taken away", status: 403, body: { status: 403, code: "forbidden" } },
  { answer: "lost", status: 500, body: { status: 500, code: "internal_error" } },
])(
  "an undo $answer, read once the tab signed out, its cache gone, leaves no rejection unhandled",
  async ({ status, body }) => {
    const user = userEvent.setup();
    let release: (() => void) | undefined;
    const { server, rework } = reworked("admin", {
      "POST /api/v0/changesets/*/revert": () =>
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
    const { app } = renderApp(changesetPath(rework.id), server.app);
    const dialog = await opened(user);
    const rejections: unknown[] = [];
    const record = (reason: unknown) => void rejections.push(reason);
    process.on("unhandledRejection", record);
    onTestFinished(() => {
      process.off("unhandledRejection", record);
    });

    await user.click(within(dialog).getByRole("button", { name: "Undo" }));
    await waitFor(() => expect(release).toBeDefined());
    // The answer's headers pass the session's check; its body is still on its way.
    await act(async () => new Promise((resolve) => setTimeout(resolve, 10)));
    await act(() => app.session.tokens.signOut());
    await waitFor(() => expect(screen.queryByRole("heading", { level: 1, name: "Edit · Ada" })).toBeNull());
    await act(async () => {
      release?.();
      await new Promise((resolve) => setTimeout(resolve, 50));
    });
    expect(rejections).toEqual([]);
  }
);

// The negative controls (M8/P3 design section 6).

/** gate is a wait a test opens: what waits for its promise goes on once open is called. */
function gate() {
  let open!: () => void;
  const promise = new Promise<void>((resolve) => {
    open = resolve;
  });
  return { promise, open };
}

test("an undo whose answer was lost says its outcome is not known as it reads the changeset again", async () => {
  const user = userEvent.setup();
  const { server, rework } = lost(
    () => problem(500, "internal_error"),
    () => undefined
  );
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await opened(user);
  const read = gate();
  server.historyWaits.changeset = read.promise;

  await user.click(within(dialog).getByRole("button", { name: "Undo" }));

  await waitFor(() =>
    expect(screen.getByRole("status").textContent).toBe(
      "Whether it was undone is not known: reading this change again…"
    )
  );
  act(() => read.open());
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("It was not undone: you can try again."));
});

test("each undo says what came of it alone: what the one before said goes as it is sent", async () => {
  const user = userEvent.setup();
  const { server, rework } = reworked();
  rework.unchanged = true;
  renderApp(changesetPath(rework.id), server.app);
  await user.click(within(await opened(user)).getByRole("button", { name: "Undo" }));
  await waitFor(() =>
    expect(screen.getByRole("status").textContent).toBe("Nothing to undo: every node is as it was before this change.")
  );
  rework.unchanged = false;
  server.revertsHeld = true;

  await user.click(within(await opened(user)).getByRole("button", { name: "Undo" }));

  await waitFor(() => expect(reverts(server.sent)).toHaveLength(2));
  // Outside the dialog, hidden while it is open.
  expect(screen.getByRole("status", { hidden: true }).textContent).toBe("");
  act(() => server.release());
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Undone. See the undo"));
});

test("Enter pressed twice in the message sends one undo", async () => {
  const user = userEvent.setup();
  const { server, rework } = reworked();
  server.revertsHeld = true;
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await opened(user);
  await user.type(within(dialog).getByRole("textbox", { name: "Message (optional)" }), "Why");

  await user.keyboard("{Enter}{Enter}");
  await waitFor(() => expect(reverts(server.sent)).toEqual(['REVERT rework "Why"']));
  act(() => server.release());

  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Undone. See the undo"));
  expect(reverts(server.sent)).toEqual(['REVERT rework "Why"']);
});

test("the history read before an undo lists it as soon as it is gone back to", async () => {
  const user = userEvent.setup();
  const { server, rework } = reworked();
  const { router } = renderApp(`${home}/history`, server.app);
  const list = await screen.findByRole("list", { name: "Changes" });
  await user.click(within(list).getAllByRole("link")[0] as HTMLElement);
  const dialog = await opened(user);

  await user.click(within(dialog).getByRole("button", { name: "Undo" }));
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Undone. See the undo"));
  await act(() => router.navigate(`${home}/history`));

  await waitFor(() =>
    expect(within(screen.getByRole("list", { name: "Changes" })).getAllByRole("listitem")).toHaveLength(2)
  );
  expect(rework.revertedBy).toHaveLength(1);
});

test("an undo answered no longer watches the reader", async () => {
  const user = userEvent.setup();
  const { server, rework } = reworked();
  renderApp(changesetPath(rework.id), server.app);
  const dialog = await opened(user);
  const removed = vi.spyOn(window, "removeEventListener");
  onTestFinished(() => removed.mockRestore());

  await user.click(within(dialog).getByRole("button", { name: "Undo" }));

  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Undone. See the undo"));
  expect(removed.mock.calls.filter(([, , options]) => options === true).map(([type]) => type)).toEqual(
    expect.arrayContaining(["wheel", "touchmove", "pointerdown", "keydown"])
  );
});

test("Undo change read away as it has the focus, the changeset trimmed meanwhile, gives the focus to the title", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const { server, rework } = reworked();
  renderApp(changesetPath(rework.id), server.app);
  const undo = await screen.findByRole("button", { name: "Undo change" });
  act(() => undo.focus());

  rework.trimmed = true;
  await readAgain();

  await waitFor(() => expect(screen.queryByRole("button", { name: "Undo change" })).toBeNull());
  expect(document.activeElement).toBe(title());
});

// M8/P1 fix check 3, R3-1: the session left as the answer's body is read, its headers past the session's check.
test("an undo whose answer was lost, its changeset read again as the tab signed out, its cache gone, leaves no rejection unhandled", async () => {
  const user = userEvent.setup();
  let release: (() => void) | undefined;
  let held = false;
  const { server, rework } = reworked("admin", {
    "POST /api/v0/changesets/*/revert": () => problem(500, "internal_error"),
    "GET /api/v0/changesets/*": () => {
      const body = JSON.stringify({
        id: rework.id,
        notebook_id: notebookJSON.id,
        kind: "edit",
        client: "web",
        message: null,
        created_by: ada,
        created_at: rework.at,
        updated_at: rework.at,
        reverts: null,
        reverted_by: [],
        trimmed: false,
        node_count: 1,
        content_count: 1,
        changes: [],
        renames: true,
        moves: false,
      });
      if (!held) {
        return json(JSON.parse(body));
      }
      return new Response(
        new ReadableStream<Uint8Array>({
          start(controller) {
            release = () => {
              controller.enqueue(new TextEncoder().encode(body));
              controller.close();
            };
          },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } }
      );
    },
  });
  const { app } = renderApp(changesetPath(rework.id), server.app);
  const dialog = await opened(user);
  const rejections: unknown[] = [];
  const record = (reason: unknown) => void rejections.push(reason);
  process.on("unhandledRejection", record);
  onTestFinished(() => {
    process.off("unhandledRejection", record);
  });
  held = true;

  await user.click(within(dialog).getByRole("button", { name: "Undo" }));
  await waitFor(() => expect(release).toBeDefined());
  // The read's headers pass the session's check; its body is still on its way.
  await act(async () => new Promise((resolve) => setTimeout(resolve, 10)));
  await act(() => app.session.tokens.signOut());
  await waitFor(() => expect(screen.queryByRole("heading", { level: 1, name: "Edit · Ada" })).toBeNull());
  await act(async () => {
    release?.();
    await new Promise((resolve) => setTimeout(resolve, 50));
  });
  expect(rejections).toEqual([]);
});
