import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import type { Plugin } from "vite";
import { defineConfig } from "vitest/config";

// make web-dev serves the app with hot reload and forwards everything that is
// the backend's to make run, so the pages talk to the same origin as in
// production.
const backend = "http://127.0.0.1:8080";

/** A module of CodeMirror or lezer, or of a package only they use, wherever pnpm keeps it. */
const editorModule =
  /[\\/]node_modules[\\/](?:\.pnpm[\\/][^\\/]+[\\/]node_modules[\\/])?(?:@(?:codemirror|lezer)|style-mod|w3c-keyname|crelt)[\\/]/;

/** The modules that load the editor: the editor itself and the conflict's diff, each imported when it is needed. */
const editorEntries = /[\\/]src[\\/]editor[\\/](?:source-editor|conflict-view)\.tsx$/;

/**
 * editorOutOfMain fails the build when a chunk the app may load before the
 * editor holds a module of the editor (M4/P6 design 3.11): one the app's
 * entry reaches, by static or dynamic imports (a route's chunk too), short
 * of the editor's own entries. The editor is loaded when a page is first
 * edited, in a chunk of its own; what it loads in turn is the editor's.
 * The error says the way to the chunk, from the entry.
 */
function editorOutOfMain(): Plugin {
  return {
    name: "nervewiki:editor-out-of-main",
    apply: "build",
    generateBundle(_, bundle) {
      const chunks = new Map(
        Object.values(bundle).flatMap((output) => (output.type === "chunk" ? [[output.fileName, output] as const] : []))
      );
      // Each chunk reached, by the way to it.
      const ways = new Map(
        [...chunks.values()].filter((chunk) => chunk.isEntry).map((chunk) => [chunk.fileName, [chunk.fileName]])
      );
      for (const [name, way] of ways) {
        const chunk = chunks.get(name);
        if (chunk === undefined || editorEntries.test(chunk.facadeModuleId ?? "")) {
          continue;
        }
        const editor = chunk.moduleIds.find((id) => editorModule.test(id));
        if (editor !== undefined) {
          this.error(`${way.join(" → ")}, which is loaded before the editor, holds the editor's ${editor}`);
        }
        for (const next of [...chunk.imports, ...chunk.dynamicImports]) {
          if (!ways.has(next)) {
            ways.set(next, [...way, next]);
          }
        }
      }
    },
  };
}

export default defineConfig({
  plugins: [react(), tailwindcss(), editorOutOfMain()],
  server: {
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
    // Keys starting with ^ are regular expressions: a page path such as
    // /api-tokens stays the app's, as it is in production.
    proxy: { "^/api(/|$)": backend, "^/healthz$": backend, "^/readyz$": backend },
  },
  build: {
    rolldownOptions: {
      output: {
        // React and the router change less often than the app: their own chunk
        // stays cached across releases that change only the app. So does the
        // editor's CodeMirror, which the conflict's diff shares; its merge
        // view stays out, loaded with the diff alone.
        codeSplitting: {
          groups: [
            { name: "react", test: /node_modules[\\/](react|react-dom|react-router|scheduler)[\\/]/ },
            {
              name: "codemirror",
              test: /node_modules[\\/](@codemirror[\\/](?!merge[\\/])|@lezer[\\/]|style-mod[\\/]|w3c-keyname[\\/]|crelt[\\/])/,
            },
          ],
        },
      },
    },
  },
  test: {
    environment: "jsdom",
    // Dates are written in the browser's time zone: the tests' is fixed, so
    // that they pass wherever they run.
    env: { TZ: "UTC" },
    setupFiles: ["src/test/setup.ts"],
    restoreMocks: true,
    unstubGlobals: true,
  },
});
