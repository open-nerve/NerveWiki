import type { Plugin } from "vite";

/** A module of CodeMirror or lezer, or of a package only they use, wherever pnpm keeps it. */
const editorModule =
  /[\\/]node_modules[\\/](?:\.pnpm[\\/][^\\/]+[\\/]node_modules[\\/])?(?:@(?:codemirror|lezer)|style-mod|w3c-keyname|crelt)[\\/]/;

/** The modules that load the editor: the editor itself and the conflict's diff, each imported when it is needed. */
const editorEntries = /[\\/]src[\\/]editor[\\/](?:source-editor|conflict-view)\.tsx$/;

/** What the check reads of an output chunk. */
export type Chunk = {
  fileName: string;
  isEntry: boolean;
  facadeModuleId: string | null;
  moduleIds: readonly string[];
  imports: readonly string[];
  dynamicImports: readonly string[];
};

/**
 * editorLeak finds a chunk the app may load before the editor that holds a
 * module of the editor (M4/P6 design 3.11): one the app's entry reaches,
 * by static or dynamic imports (a route's chunk too), short of the
 * editor's own entries, whose chunk and what it loads in turn are the
 * editor's. It answers what is wrong, with the way to the chunk from the
 * entry, or undefined when nothing is.
 */
export function editorLeak(output: readonly Chunk[]): string | undefined {
  const chunks = new Map(output.map((chunk) => [chunk.fileName, chunk]));
  // Each chunk reached, by the way to it.
  const ways = new Map(output.filter((chunk) => chunk.isEntry).map((chunk) => [chunk.fileName, [chunk.fileName]]));
  for (const [name, way] of ways) {
    const chunk = chunks.get(name);
    if (chunk === undefined || editorEntries.test(chunk.facadeModuleId ?? "")) {
      continue;
    }
    const editor = chunk.moduleIds.find((id) => editorModule.test(id));
    if (editor !== undefined) {
      return `${way.join(" → ")}, which is loaded before the editor, holds the editor's ${editor}`;
    }
    for (const next of [...chunk.imports, ...chunk.dynamicImports]) {
      if (!ways.has(next)) {
        ways.set(next, [...way, next]);
      }
    }
  }
  return undefined;
}

/** editorOutOfMain fails the build on an editorLeak: the editor is loaded when a page is first edited, in a chunk of its own. */
export function editorOutOfMain(): Plugin {
  return {
    name: "nervewiki:editor-out-of-main",
    apply: "build",
    generateBundle(_, bundle) {
      const leak = editorLeak(Object.values(bundle).flatMap((output) => (output.type === "chunk" ? [output] : [])));
      if (leak !== undefined) {
        this.error(leak);
      }
    },
  };
}
