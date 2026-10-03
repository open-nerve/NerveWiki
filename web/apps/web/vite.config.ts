import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import type { Plugin } from "vite";
import { defineConfig } from "vitest/config";

// make web-dev serves the app with hot reload and forwards everything that is
// the backend's to make run, so the pages talk to the same origin as in
// production.
const backend = "http://127.0.0.1:8080";

/** A module of CodeMirror or lezer, wherever pnpm keeps it. */
const editorModule = /[\\/]node_modules[\\/](?:\.pnpm[\\/][^\\/]+[\\/]node_modules[\\/])?@(?:codemirror|lezer)[\\/]/;

/**
 * editorOutOfMain fails the build when the app's entry, or a chunk it
 * imports statically, holds a module of the editor (M4/P6 design 3.11):
 * the editor is loaded when a page is first edited, in a chunk of its own.
 */
function editorOutOfMain(): Plugin {
  return {
    name: "nervewiki:editor-out-of-main",
    apply: "build",
    generateBundle(_, bundle) {
      const chunks = new Map(
        Object.values(bundle).flatMap((output) => (output.type === "chunk" ? [[output.fileName, output] as const] : []))
      );
      const loaded = [...chunks.values()].filter((chunk) => chunk.isEntry).map((chunk) => chunk.fileName);
      // loaded grows as the chunks it holds import others.
      for (let i = 0; i < loaded.length; i++) {
        const name = loaded[i] ?? "";
        const chunk = chunks.get(name);
        loaded.push(...(chunk?.imports ?? []).filter((imported) => !loaded.includes(imported)));
        const editor = chunk?.moduleIds.find((id) => editorModule.test(id));
        if (editor !== undefined) {
          this.error(`${name}, which the app loads first, holds the editor's ${editor}`);
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
