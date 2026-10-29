import type { dropTargetForElements } from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import { extractInstruction } from "@atlaskit/pragmatic-drag-and-drop-hitbox/list-item";
import { act, screen, within } from "@testing-library/react";
import { expect, test, vi } from "vitest";

import { notebookJSON } from "../../test/fakes";
import { guide, install, notes, pageNode, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// Observe the adapter registration while keeping its real cleanup and the real hitbox calculation.
type Target = Parameters<typeof dropTargetForElements>[0];
const targets = vi.hoisted(() => new Map<Element, Target>());
vi.mock("@atlaskit/pragmatic-drag-and-drop/element/adapter", async (original) => {
  const actual = await original<typeof import("@atlaskit/pragmatic-drag-and-drop/element/adapter")>();
  return {
    ...actual,
    dropTargetForElements: (target: Target) => {
      targets.set(target.element, target);
      const clean = actual.dropTargetForElements(target);
      return () => {
        targets.delete(target.element);
        clean();
      };
    },
  };
});

test.each([
  { role: "editor", y: 20, operation: "combine", blocked: true },
  { role: "editor", y: 1, operation: "reorder-before", blocked: false },
  { role: "admin", y: 20, operation: "combine", blocked: false },
] as const)(
  "source-root ancestor drag hints: $role $operation blocked=$blocked",
  async ({ role, y, operation, blocked }) => {
    const raw = { ...guide, parent_id: notes.id };
    const outside = pageNode(90, "Outside");
    const server = pageServer({ role, mode: "llm_wiki", sourceRootId: raw.id, nodes: [notes, raw, install, outside] });
    renderApp(pagePath(notes.id), server.app);
    const nav = await screen.findByRole("navigation", { name: "Pages of Plans" });
    const row = (await within(nav).findByRole("link", { name: "Outside" })).parentElement!;
    vi.spyOn(row, "getBoundingClientRect").mockReturnValue({
      top: 0,
      bottom: 40,
      left: 0,
      right: 200,
      height: 40,
      width: 200,
      x: 0,
      y: 0,
      toJSON: () => ({}),
    });
    const target = targets.get(row);
    expect(target?.getData).toBeTypeOf("function");
    const input = {
      altKey: false,
      button: 0,
      buttons: 1,
      ctrlKey: false,
      metaKey: false,
      shiftKey: false,
      clientX: 100,
      clientY: y,
      pageX: 100,
      pageY: y,
    };
    const source = {
      element: within(nav).getByRole("link", { name: "Notes" }).parentElement!,
      dragHandle: null,
      data: { page: notes.id, notebook: notebookJSON.id },
    };
    const data = target!.getData!({ element: row, input, source });
    expect(extractInstruction(data)).toMatchObject({ operation, blocked });
    const self = { element: row, data, dropEffect: "move" as const, isActiveDueToStickiness: false };
    await act(async () =>
      target!.onDrag!({
        self,
        source,
        location: {
          initial: { input, dropTargets: [] },
          previous: { dropTargets: [] },
          current: { input, dropTargets: [self] },
        },
      })
    );
    if (operation === "combine") {
      expect(row.classList.contains(blocked ? "ring-destructive" : "ring-primary")).toBe(true);
    } else {
      expect(row.querySelector(".bg-primary")).not.toBeNull();
    }
    expect(server.sent.filter((line) => line.startsWith("MOVE"))).toEqual([]);
  }
);
