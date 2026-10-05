import type { monitorForElements } from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";

import { notebookJSON, problem } from "../../test/fakes";
import { ada, bob, guide, install, linux, notes, pageNode, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// A drop the server refuses (M6/P4 design 6): jsdom does not drag, so the
// tree's monitor and the hitbox's instruction are the test's; the rest is
// the app's.

type Monitor = Parameters<typeof monitorForElements>[0];

const monitors = vi.hoisted((): Monitor[] => []);

vi.mock("@atlaskit/pragmatic-drag-and-drop/element/adapter", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@atlaskit/pragmatic-drag-and-drop/element/adapter")>()),
  monitorForElements: (monitor: Monitor) => {
    monitors.push(monitor);
    return () => {};
  },
}));

vi.mock("@atlaskit/pragmatic-drag-and-drop-hitbox/list-item", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@atlaskit/pragmatic-drag-and-drop-hitbox/list-item")>()),
  extractInstruction: () => ({ operation: "combine", blocked: false }),
}));

const home = `/lab/notebooks/${notebookJSON.id}`;
const tree = () => screen.findByRole("navigation", { name: "Pages of Plans" });

/** Drops the page id into the page onto, as the tree's monitor hears it. */
async function drop(id: string, onto: string) {
  const monitor = monitors.at(-1);
  await act(async () => {
    monitor?.onDrop?.({
      source: { data: { page: id, notebook: notebookJSON.id } },
      location: { current: { dropTargets: [{ data: { page: onto } }] } },
    } as never);
  });
}

/** A second Notes, under Guide: the tree tells the two apart by where they are. */
const otherNotes = pageNode(5, "Notes", guide);

test("a drop whose links' pages are being edited names them and their editors below the tree's heading, the account itself too, each as the tree names it; a rename or a move sent clears it", async () => {
  const user = userEvent.setup();
  const server = pageServer({
    nodes: [guide, install, linux, notes, otherNotes],
    answers: {
      "POST /api/v0/nodes/*/move": () =>
        problem(409, "linking.pages_locked", {
          locks: [
            { page_id: notes.id, ...bob },
            { page_id: otherNotes.id, ...ada },
          ],
        }),
    },
  });
  renderApp(home, server.app);
  const nav = await tree();
  await within(nav).findByRole("link", { name: "Guide" });

  await drop(linux.id, guide.id);
  const alert = await within(nav).findByRole("alert");
  expect(
    within(alert)
      .getAllByRole("listitem")
      .map((item) => item.textContent)
  ).toEqual(["Bob is editing “Notes (in Plans)”.", "You are editing “Notes (in Guide)”."]);

  await user.click(within(nav).getByRole("button", { name: "Actions for Guide" }));
  await user.click(await screen.findByRole("menuitem", { name: "Rename" }));
  const dialog = await screen.findByRole("dialog", { name: "Rename Guide" });
  await user.clear(within(dialog).getByLabelText("Title"));
  await user.type(within(dialog).getByLabelText("Title"), "Handbook");
  await user.click(within(dialog).getByRole("button", { name: "Save" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(within(await tree()).queryByRole("alert")).toBeNull();

  // A move sent, refused in its dialog, clears it too.
  await drop(linux.id, guide.id);
  await within(await tree()).findByRole("alert");
  await user.click(within(await tree()).getByRole("button", { name: "Actions for Handbook" }));
  await user.click(await screen.findByRole("menuitem", { name: "Move to…" }));
  const move = await screen.findByRole("dialog", { name: "Move Handbook" });
  await user.selectOptions(within(move).getByLabelText("Parent page"), "Notes");
  await user.click(within(move).getByRole("button", { name: "Move" }));
  await within(move).findByRole("alert");
  await user.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(within(await tree()).queryByRole("alert")).toBeNull();
});
