import { createRequire } from "node:module";
import path from "node:path";

// The parse page's script (M12/P4 design 4.4): entry.ts as one IIFE, by a library build of the web app's own Vite,
// loaded by its path (e2e does not depend on it; knip.jsonc says so). Not the web app's vite.config.ts: it splits
// the build into chunks, which an IIFE cannot be. No alias: e2e's CodeMirror packages are the web app's, from the
// same store, so the script holds one CodeMirror.

const web = path.join(import.meta.dirname, "../../../web/apps/web");

/** Builds the parse page's script into out, and answers its path. */
export async function buildParse(out: string): Promise<string> {
  const require = createRequire(path.join(web, "package.json"));
  const vite = (await import(require.resolve("vite"))) as {
    build: (config: object) => Promise<unknown>;
  };
  await vite.build({
    root: web,
    configFile: false,
    logLevel: "warn",
    build: {
      lib: {
        entry: path.join(import.meta.dirname, "entry.ts"),
        formats: ["iife"],
        name: "nwParsePage",
        fileName: () => "parse.js",
      },
      outDir: out,
      emptyOutDir: true,
      minify: false,
      copyPublicDir: false,
    },
  });
  return path.join(out, "parse.js");
}
