import { expect, test } from "vitest";

import type { WorkspaceRole } from "../services/member.service";
import type { NotebookRole, WorkspaceAccess } from "../services/notebook.service";
import { upgradeOf } from "../test/fakes";
import { effectiveNotebookRole, frozenByUpgrade, writesNotebook, writesPages } from "./effective-role";

// The server's table (shared.TestEffectiveNotebookRole): for each role in
// the workspace and the notebook's access, the effective role of no
// membership, a reader's, an editor's and an admin's.
const explicit = [undefined, "reader", "editor", "admin"] as const;
const table: [WorkspaceRole, WorkspaceAccess, readonly (NotebookRole | undefined)[]][] = [
  ["admin", "none", [undefined, "reader", "editor", "admin"]],
  ["admin", "viewer", ["reader", "reader", "editor", "admin"]],
  ["admin", "editor", ["editor", "editor", "editor", "admin"]],
  ["member", "none", [undefined, "reader", "editor", "admin"]],
  ["member", "viewer", ["reader", "reader", "editor", "admin"]],
  ["member", "editor", ["editor", "editor", "editor", "admin"]],
  ["guest", "none", [undefined, "reader", "editor", "admin"]],
  ["guest", "viewer", [undefined, "reader", "editor", "admin"]],
  ["guest", "editor", [undefined, "reader", "editor", "admin"]],
];

test.each(table)("a workspace %s in a notebook open as %s", (workspace, access, want) => {
  expect(explicit.map((role) => effectiveNotebookRole(role, access, workspace))).toEqual(want);
});

test("editors and admins write a notebook's pages, readers and roles unknown do not", () => {
  expect((["reader", "editor", "admin"] as const).map(writesPages)).toEqual([false, true, true]);
  // A role this client does not know yet writes nothing.
  expect(writesPages("owner" as NotebookRole)).toBe(false);
});

// An upgrade under way freezes the notebook for every role on the web, its initiator's too (M11/P1 design 4.4).
test.each([
  ["admin", null, true, false],
  ["editor", null, true, false],
  ["reader", null, false, false],
  ["admin", "upgrading", false, true],
  ["editor", "reviewing", false, true],
  ["admin", "rolling_back", false, true],
  ["reader", "upgrading", false, false],
] as const)(
  "a notebook's %s, its upgrade %s, writes from the web: %s; only the upgrade keeps it from it: %s",
  (role, state, writes, frozen) => {
    const notebook = { role, upgrade: state === null ? null : upgradeOf(state) };
    expect([writesNotebook(notebook), frozenByUpgrade(notebook)]).toEqual([writes, frozen]);
  }
);
