import type { Plugin } from "vite";

/** A package's module, of a name in packages, wherever pnpm keeps it. */
const packageModule = (packages: string) =>
  new RegExp(String.raw`[\\/]node_modules[\\/](?:\.pnpm[\\/][^\\/]+[\\/]node_modules[\\/])?(?:${packages})[\\/]`);

/**
 * What the app loads when a page first needs it, each in chunks of its own
 * (M4/P6 design 3.11, M6/P6 design 10): its modules, and the chunks that
 * load it, whose modules and what they load in turn are its own. The
 * editor, CodeMirror and lezer with the packages only they use, loads
 * with the editor or the conflict's diff; KaTeX with a view's first
 * formula, and mermaid with its first diagram, each by its own module.
 */
const lazy = [
  {
    name: "the editor",
    modules: packageModule(String.raw`@(?:codemirror|lezer)|style-mod|w3c-keyname|crelt`),
    entries: /[\\/]src[\\/]editor[\\/](?:source-editor|conflict-view)\.tsx$/,
  },
  { name: "KaTeX", modules: packageModule("katex"), entries: packageModule("katex") },
  {
    name: "mermaid",
    modules: packageModule(String.raw`mermaid|@mermaid-js[\\/][^\\/]+`),
    entries: packageModule(String.raw`mermaid|@mermaid-js[\\/][^\\/]+`),
  },
];

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
 * lazyLeak finds a chunk the app may load before what loads only when a
 * page needs it (lazy) that holds a module of it: one the app's entry
 * reaches, by static or dynamic imports (a route's chunk too), short of
 * the chunks that load such a thing. It answers what is wrong, with the
 * way to the chunk from the entry, or undefined when nothing is.
 */
export function lazyLeak(output: readonly Chunk[]): string | undefined {
  const chunks = new Map(output.map((chunk) => [chunk.fileName, chunk]));
  // Each chunk reached, by the way to it.
  const ways = new Map(output.filter((chunk) => chunk.isEntry).map((chunk) => [chunk.fileName, [chunk.fileName]]));
  for (const [name, way] of ways) {
    const chunk = chunks.get(name);
    if (chunk === undefined || lazy.some(({ entries }) => entries.test(chunk.facadeModuleId ?? ""))) {
      continue;
    }
    for (const { name: what, modules } of lazy) {
      const held = chunk.moduleIds.find((id) => modules.test(id));
      if (held !== undefined) {
        return `${way.join(" → ")}, which is loaded before ${what}, holds ${what}'s ${held}`;
      }
    }
    for (const next of [...chunk.imports, ...chunk.dynamicImports]) {
      if (!ways.has(next)) {
        ways.set(next, [...way, next]);
      }
    }
  }
  return undefined;
}

/**
 * outOfMain fails the build on a lazyLeak: the editor, KaTeX and mermaid
 * are loaded when a page first needs them, each in chunks of its own.
 */
export function outOfMain(): Plugin {
  return {
    name: "nervewiki:out-of-main",
    apply: "build",
    generateBundle(_, bundle) {
      const leak = lazyLeak(Object.values(bundle).flatMap((output) => (output.type === "chunk" ? [output] : [])));
      if (leak !== undefined) {
        this.error(leak);
      }
    },
  };
}
