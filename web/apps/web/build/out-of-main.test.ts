import { expect, test } from "vitest";

import { lazyLeak, type Chunk } from "./out-of-main";

// The build's check that the editor, KaTeX and mermaid stay out of what
// loads before them (M4/P6 design 3.11, M6/P6 design 10), on chunk graphs
// as rolldown answers them.

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
/** registry is the app's entry, whose registry of the editor's extensions may import one's module. */
const registry = (facts: Partial<Chunk>) => app({ dynamicImports: ["page.js"], ...facts });

test("the editor and what it loads hold the editor's modules: nothing leaks", () => {
  expect(
    lazyLeak([
      app({ dynamicImports: ["page.js"] }),
      chunk("page.js", { dynamicImports: ["source-editor.js"] }),
      editor,
      codemirror,
    ])
  ).toBeUndefined();
});

test("an editor's extension the editor loads (editor/loaded/), by a dynamic import of the app's, is the editor's: nothing leaks", () => {
  const extension = chunk("link-completion.js", {
    facadeModuleId: "/repo/web/apps/web/src/editor/loaded/link-completion.ts",
    moduleIds: ["/repo/web/apps/web/src/editor/loaded/link-completion.ts"],
    imports: ["codemirror.js"],
  });
  expect(
    lazyLeak([registry({ dynamicImports: ["page.js", "link-completion.js"] }), chunk("page.js"), extension, codemirror])
  ).toBeUndefined();
  // Imported statically, it loads with the app.
  expect(lazyLeak([registry({ imports: ["link-completion.js"] }), chunk("page.js"), extension, codemirror])).toBe(
    `index.js → link-completion.js → codemirror.js, which is loaded before the editor, holds the editor's ${view}`
  );
  // A module of React's there (.tsx) too.
  const tsx = { ...extension, facadeModuleId: "/repo/web/apps/web/src/editor/loaded/link-panel.tsx" };
  expect(lazyLeak([registry({ dynamicImports: ["link-completion.js"] }), tsx, codemirror])).toBeUndefined();
  // Elsewhere than editor/loaded/, it is not the editor's.
  const elsewhere = { ...extension, facadeModuleId: "/repo/web/apps/web/src/editor/link-completion.ts" };
  expect(lazyLeak([registry({ dynamicImports: ["link-completion.js"] }), elsewhere, codemirror])).toBe(
    `index.js → link-completion.js → codemirror.js, which is loaded before the editor, holds the editor's ${view}`
  );
});

test("the entry holding an editor's module leaks", () => {
  expect(lazyLeak([app({ moduleIds: [crelt] })])).toBe(
    `index.js, which is loaded before the editor, holds the editor's ${crelt}`
  );
});

test("a route's chunk that imports the editor's chunk statically leaks, said by the way to it", () => {
  expect(
    lazyLeak([
      app({ dynamicImports: ["page.js"] }),
      chunk("page.js", { imports: ["codemirror.js"] }),
      editor,
      codemirror,
    ])
  ).toBe(`index.js → page.js → codemirror.js, which is loaded before the editor, holds the editor's ${view}`);
});

test("a chunk reached by a dynamic import of a chunk the entry imports statically leaks", () => {
  expect(
    lazyLeak([
      app({ imports: ["shared.js"] }),
      chunk("shared.js", { dynamicImports: ["lazy.js"] }),
      chunk("lazy.js", { moduleIds: [view] }),
    ])
  ).toBe(`index.js → shared.js → lazy.js, which is loaded before the editor, holds the editor's ${view}`);
});

test("a chunk no entry reaches is not looked at", () => {
  expect(lazyLeak([app(), chunk("orphan.js", { moduleIds: [view] })])).toBeUndefined();
});

const katex = "/repo/node_modules/.pnpm/katex@0.16.47/node_modules/katex/dist/katex.mjs";
const katexStyles = "/repo/node_modules/.pnpm/katex@0.16.47/node_modules/katex/dist/katex.min.css";
const mermaid = "/repo/node_modules/.pnpm/mermaid@11.17.2/node_modules/mermaid/dist/mermaid.core.mjs";
const mermaidParser =
  "/repo/node_modules/.pnpm/@mermaid-js+parser@1.0.0/node_modules/@mermaid-js/parser/dist/mermaid-parser.core.mjs";

test("KaTeX and mermaid in chunks of their own, each loaded by its module, leak nothing: mermaid's own KaTeX neither", () => {
  expect(
    lazyLeak([
      app({ dynamicImports: ["page.js", "katex.js", "katex-css.js", "mermaid.js"] }),
      chunk("page.js"),
      chunk("katex.js", { facadeModuleId: katex, moduleIds: [katex] }),
      chunk("katex-css.js", { facadeModuleId: katexStyles, moduleIds: [katexStyles] }),
      chunk("mermaid.js", { facadeModuleId: mermaid, moduleIds: [mermaid, mermaidParser, katex] }),
    ])
  ).toBeUndefined();
});

test("KaTeX or mermaid in a chunk loaded before them leaks", () => {
  expect(lazyLeak([app({ moduleIds: [katex] })])).toBe(
    `index.js, which is loaded before KaTeX, holds KaTeX's ${katex}`
  );
  expect(
    lazyLeak([
      app({ imports: ["shared.js"] }),
      chunk("shared.js", { moduleIds: ["/repo/web/apps/web/src/a.ts", mermaidParser] }),
    ])
  ).toBe(`index.js → shared.js, which is loaded before mermaid, holds mermaid's ${mermaidParser}`);
});

test("a route's chunk that imports KaTeX's, mermaid's or the editor's own chunk statically leaks: it loads with the route", () => {
  const route = (imports: string[]) => [
    app({ dynamicImports: ["page.js", "katex.js", "mermaid.js", "source-editor.js"] }),
    chunk("page.js", { imports }),
    chunk("katex.js", { facadeModuleId: katex, moduleIds: [katex] }),
    chunk("mermaid.js", { facadeModuleId: mermaid, moduleIds: [mermaid] }),
    editor,
    codemirror,
  ];
  expect(lazyLeak(route(["katex.js"]))).toBe(
    `index.js → page.js → katex.js, which is loaded before KaTeX, holds KaTeX's ${katex}`
  );
  expect(lazyLeak(route(["mermaid.js"]))).toBe(
    `index.js → page.js → mermaid.js, which is loaded before mermaid, holds mermaid's ${mermaid}`
  );
  expect(lazyLeak(route(["source-editor.js"]))).toBe(
    `index.js → page.js → source-editor.js → codemirror.js, which is loaded before the editor, holds the editor's ${view}`
  );
});
