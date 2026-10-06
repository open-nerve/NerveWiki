import { completionStatus, currentCompletions } from "@codemirror/autocomplete";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { editorExtensions } from "../../editor/registry";
import { pageEditor } from "../../test/page-editor";
import { guide, install, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The editor's completion of links and tags on the page, through the
// composition root's registry (M6/P7 design 14; M4/P6 editor handoff,
// item 5): the app's editorExtensions, loaded as the editor opens.

/** Guide edited by Ada, with the app's extensions, over server. */
async function editing(server = pageServer()) {
  const user = userEvent.setup();
  renderApp(pagePath(guide.id), server.app, { editorExtensions });
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  return pageEditor();
}

/** typeAtEnd types text at the editor's end, the cursor after it, as the keyboard does. */
function typeAtEnd(view: Awaited<ReturnType<typeof pageEditor>>["view"], text: string) {
  const at = view.state.doc.length;
  view.dispatch({
    changes: { from: at, insert: text },
    selection: { anchor: at + text.length },
    userEvent: "input.type",
  });
}

test("[[ lists the notebook's pages and their aliases, read as the completion opens; # its tags", async () => {
  const server = pageServer();
  server.aliases.set(install.id, ["Setup"]);
  server.tags.set("project", [guide.id, install.id]);
  const { view } = await editing(server);

  typeAtEnd(view, "\n[[");
  await waitFor(() => expect(completionStatus(view.state)).toBe("active"));
  await waitFor(() =>
    expect(currentCompletions(view.state).map(({ displayLabel, label }) => displayLabel ?? label)).toEqual(
      expect.arrayContaining(["Guide", "Install", "Linux", "Notes", "Setup"])
    )
  );
  expect(server.sent).toContain("GET link targets");

  typeAtEnd(view, "Notes]] #pro");
  await waitFor(() =>
    expect(currentCompletions(view.state).map(({ label, detail }) => [label, detail])).toEqual([["project", "2 pages"]])
  );
  expect(server.sent).toContain("GET tags");
});
