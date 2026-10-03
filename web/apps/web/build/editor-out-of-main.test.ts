import { expect, test } from "vitest";

import { editorLeak, type Chunk } from "./editor-out-of-main";

// The build's check that the editor stays out of what loads before it
// (M4/P6 design 3.11), on chunk graphs as rolldown answers them.

const view = "/repo/node_modules/.pnpm/@codemirror+view@6.43.13/node_modules/@codemirror/view/dist/index.js";
const crelt = "/repo/node_modules/crelt/index.js";

function chunk(fileName: string, facts: Partial<Chunk> = {}): Chunk {
  return { fileName, isEntry: false, facadeModuleId: null, moduleIds: [], imports: [], dynamicImports: [], ...facts };
}

const app = (facts: Partial<Chunk> = {}) =>
  chunk("index.js", { isEntry: true, facadeModuleId: "/repo/web/apps/web/index.html", ...facts });
const editor = chunk("source-editor.js", {
  facadeModuleId: "/repo/web/apps/web/src/editor/source-editor.tsx",
  moduleIds: ["/repo/web/apps/web/src/editor/source-editor.tsx"],
  imports: ["codemirror.js"],
});
const codemirror = chunk("codemirror.js", { moduleIds: [view, crelt] });

test("the editor and what it loads hold the editor's modules: nothing leaks", () => {
  expect(
    editorLeak([
      app({ dynamicImports: ["page.js"] }),
      chunk("page.js", { dynamicImports: ["source-editor.js"] }),
      editor,
      codemirror,
    ])
  ).toBeUndefined();
});

test("the entry holding an editor's module leaks", () => {
  expect(editorLeak([app({ moduleIds: [crelt] })])).toBe(
    `index.js, which is loaded before the editor, holds the editor's ${crelt}`
  );
});

test("a route's chunk that imports the editor's chunk statically leaks, said by the way to it", () => {
  expect(
    editorLeak([
      app({ dynamicImports: ["page.js"] }),
      chunk("page.js", { imports: ["codemirror.js"] }),
      editor,
      codemirror,
    ])
  ).toBe(`index.js → page.js → codemirror.js, which is loaded before the editor, holds the editor's ${view}`);
});

test("a chunk reached by a dynamic import of a chunk the entry imports statically leaks", () => {
  expect(
    editorLeak([
      app({ imports: ["shared.js"] }),
      chunk("shared.js", { dynamicImports: ["lazy.js"] }),
      chunk("lazy.js", { moduleIds: [view] }),
    ])
  ).toBe(`index.js → shared.js → lazy.js, which is loaded before the editor, holds the editor's ${view}`);
});

test("a chunk no entry reaches is not looked at", () => {
  expect(editorLeak([app(), chunk("orphan.js", { moduleIds: [view] })])).toBeUndefined();
});
