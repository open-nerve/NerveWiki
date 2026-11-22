import { readFileSync } from "node:fs";
import { URL as FileURL } from "node:url";
import { parseSync } from "vite";
import { expect, test } from "vitest";

// The design tokens' conventions (M13/P1/S1, decision 751; v0.1 design 13.2 item 32): both themes have every colour,
// amber marks a state and fills nothing, components name the theme's colours, never a palette's nor a written value.
// styles.css is read from the disk: vitest serves a stylesheet's ?raw as "" (P1 design 1); the sources are read as
// the bundler sees them, the tests and their helpers left out.

const styles = readFileSync(new FileURL("./styles.css", import.meta.url), "utf8");
const sources = Object.entries(
  import.meta.glob<string>(["./**/*.{ts,tsx}", "!./**/*.test.{ts,tsx}", "!./test/**"], {
    query: "?raw",
    import: "default",
    eager: true,
  })
);

/** read answers styles.css and the sources, once it has made sure they were read: a check of nothing passes. */
function read(): { styles: string; sources: [string, string][] } {
  expect(styles.length).toBeGreaterThan(1000);
  expect(sources.length).toBeGreaterThan(250);
  expect(sources.map(([path]) => path)).toContain("./components/ui/button.tsx");
  return { styles, sources };
}

/** hits are the lines of text, named by file, on which a pattern (global) matches, each as "file:line: the line". */
function hits(file: string, text: string, ...patterns: RegExp[]): string[] {
  const lines = text.split("\n");
  const found = new Set<string>();
  for (const pattern of patterns) {
    for (const match of text.matchAll(pattern)) {
      const line = text.slice(0, match.index).split("\n").length;
      found.add(`${file}:${line}: ${lines[line - 1]?.trim() ?? ""}`);
    }
  }
  return [...found];
}

/** Block is a rule at the top of a stylesheet, outside any other: what is before its {, and what is in it. */
type Block = { prelude: string; body: string };

/** blocks are css's top rules, its comments left out; a statement (@import, @custom-variant) is none. */
function blocks(css: string): Block[] {
  const text = css.replaceAll(/\/\*[\s\S]*?\*\//g, (comment) => comment.replaceAll(/[^\n]/g, " "));
  const found: Block[] = [];
  let depth = 0;
  let start = 0;
  let open = 0;
  for (let at = 0; at < text.length; at++) {
    const c = text[at];
    if (c === "{") {
      if (depth === 0) open = at;
      depth++;
    } else if (c === "}") {
      depth--;
      if (depth === 0) {
        found.push({ prelude: text.slice(start, open).trim(), body: text.slice(open + 1, at) });
        start = at + 1;
      }
    } else if (c === ";" && depth === 0) {
      start = at + 1;
    }
  }
  return found;
}

/** declared are the custom properties declared in the top rules of css whose prelude is prelude. */
function declared(css: string, prelude: string): Set<string> {
  const names = new Set<string>();
  for (const block of blocks(css).filter((b) => b.prelude === prelude)) {
    for (const [, name] of block.body.matchAll(/(?:^|[;{\s])(--[\w-]+)\s*:/g)) names.add(name ?? "");
  }
  return names;
}

/** Literal is a string literal of a source, a module's path not one, and the line it starts on. */
type Literal = { text: string; line: number };

/** Node is a node of the parser's tree, as much of it as literals reads. */
type Node = { type: string; start: number; value?: unknown; source?: Node };

/** Where a module's path is: what imports or exports it, a dynamic import, TypeScript's import types and modules. */
const modulePathOf = new Set([
  "ImportDeclaration",
  "ExportNamedDeclaration",
  "ExportAllDeclaration",
  "ImportExpression",
  "TSImportType",
  "TSExternalModuleReference",
  "TSModuleDeclaration",
]);

/** literals are the string literals of source, the parts of its templates each one, modules' paths left out. */
function literals(path: string, source: string): Literal[] {
  const parsed = parseSync(path, source);
  expect(parsed.errors, path).toEqual([]);
  const found: Literal[] = [];
  const lineOf = (at: number) => source.slice(0, at).split("\n").length;
  const visit = (node: Node, parent: Node | undefined): void => {
    if (node.type === "Literal" && typeof node.value === "string" && !modulePathOf.has(parent?.type ?? "")) {
      found.push({ text: node.value, line: lineOf(node.start) });
    } else if (node.type === "TemplateElement") {
      const { cooked, raw } = node.value as { cooked: string | null; raw: string };
      found.push({ text: cooked ?? raw, line: lineOf(node.start) });
    }
    for (const [key, child] of Object.entries(node)) {
      if (key === "parent") continue;
      for (const each of Array.isArray(child) ? child : [child]) {
        if (typeof each === "object" && each !== null && typeof (each as Node).type === "string") {
          visit(each as Node, node);
        }
      }
    }
  };
  visit(parsed.program as unknown as Node, undefined);
  return found;
}

test("every colour of the light theme is the dark theme's too", () => {
  const { styles: css } = read();
  const light = declared(css, ":root");
  // The radius is no colour: the same in either theme.
  light.delete("--radius");
  const dark = declared(css, ".dark");
  for (const name of ["--background", "--accent", "--ring", "--sidebar"]) expect(light, name).toContain(name);
  expect(light.size).toBeGreaterThanOrEqual(20);
  const missing = {
    "light, not dark": [...light].filter((name) => !dark.has(name)),
    "dark, not light": [...dark].filter((name) => !light.has(name)),
  };
  expect(missing).toEqual({ "light, not dark": [], "dark, not light": [] });
});

test("no accent-foreground is left", () => {
  const { styles: css, sources: files } = read();
  const found = [
    ...hits("./styles.css", css, /accent-foreground/g),
    ...files.flatMap(([path, source]) => hits(path, source, /accent-foreground/g)),
  ];
  expect(found).toEqual([]);
});

test("amber is no fill", () => {
  const { styles: css, sources: files } = read();
  const found = [
    // bg-accent-soft is a tint of it, under what is current.
    ...hits("./styles.css", css, /\bbg-accent(?![-\w])/g),
    ...files.flatMap(([path, source]) =>
      hits(path, source, /\bbg-accent(?![-\w])/g, /background(?:Color)?["']?\s*:\s*["'`]var\(--accent\)/g)
    ),
  ];
  expect(found).toEqual([]);
});

// A palette's colour as a class: bg-black/50, text-white, border-red-500 and the like.
const paletteClass =
  /\b(?:bg|text|border|ring|outline|fill|stroke|from|to|via|decoration|divide|placeholder|caret|accent|shadow)-(?:black|white|slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)(?:-\d{2,3})?(?:\/\d+)?\b/g;
// A colour written in a class's arbitrary value: bg-[#000], text-[rgb(0_0_0)].
const arbitraryColour = /-\[(?:#|rgb|hsl|oklch|oklab|color-mix)/g;
// A colour written as a value: #rgb, #rrggbb, #rrggbbaa, rgb(), hsl(), oklch().
const writtenColour = /#(?:[\da-f]{8}|[\da-f]{6}|[\da-f]{3})\b|\b(?:rgba?|hsla?|oklch)\(/i;
// Where a colour may be written (v0.1 design 13.2 item 32): mermaid's themeVariables take hexadecimal colours alone;
// the editor's links and matches until P4/S3.
const writtenColourMayBe = new Set(["./editor/theme.ts", "./reading/diagrams.ts"]);

test("components use the theme's colours, not Tailwind's palette nor written values", () => {
  const { sources: files } = read();
  const found = files.flatMap(([path, source]) => hits(path, source, paletteClass, arbitraryColour));
  for (const [path, source] of files.filter(([file]) => !writtenColourMayBe.has(file))) {
    for (const literal of literals(path, source).filter(({ text }) => writtenColour.test(text))) {
      found.push(`${path}:${literal.line}: ${literal.text}`);
    }
  }
  expect(found).toEqual([]);
});

// The classes of these prefixes in the sources that name no colour: sizes, sides, widths, offsets, styles, an alignment.
const noColour = new Set([
  "border-b",
  "border-b-0",
  "border-b-2",
  "border-r",
  "decoration-dashed",
  "divide-y",
  "outline-none",
  "ring-2",
  "ring-inset",
  "ring-offset-4",
  "text-2xl",
  "text-3xl",
  "text-base",
  "text-lg",
  "text-right",
  "text-sm",
  "text-xl",
  "text-xs",
]);
// What a colour class may name besides the theme's colours.
const noThemeColour = new Set(["transparent", "current", "inherit"]);

/** utility is a word of a class list with its variants (hover:, [&_svg]:) and its opacity (/50) taken off. */
function utility(word: string): string {
  let depth = 0;
  let after = 0;
  for (let at = 0; at < word.length; at++) {
    const c = word[at];
    if (c === "[" || c === "(") depth++;
    else if (c === "]" || c === ")") depth--;
    else if (c === ":" && depth === 0) after = at + 1;
  }
  return word.slice(after).replace(/^!/, "").replace(/!$/, "");
}

test("every colour class names a colour of the theme", () => {
  const { styles: css, sources: files } = read();
  const theme = blocks(css);
  const colours = new Set(
    theme
      .filter((block) => block.prelude === "@theme inline")
      .flatMap((block) => [...block.body.matchAll(/--color-([\w-]+)\s*:/g)].map(([, name]) => name ?? ""))
  );
  const sizes = new Set(
    theme
      .filter((block) => block.prelude === "@theme")
      .flatMap((block) => [...block.body.matchAll(/--text-([\w-]+)\s*:/g)].map(([, name]) => name ?? ""))
  );
  expect(colours).toContain("background");
  expect(colours).toContain("destructive");
  const found = files.flatMap(([path, source]) =>
    literals(path, source).flatMap((literal) =>
      literal.text
        .split(/\s+/)
        .map((word) => utility(word))
        .filter((word) => {
          const match = /^(bg|text|border|ring|outline|fill|stroke|divide|decoration|caret|placeholder)-(.+)$/.exec(
            word
          );
          if (match === null || noColour.has(word)) return false;
          const [, prefix, value = ""] = match;
          // An arbitrary value, bg-(--nw-mark-background) or text-[13px], is the class's own.
          if (/[[(]/.test(value)) return false;
          const name = value.replace(/\/.*$/, "");
          if (noThemeColour.has(name) || colours.has(name)) return false;
          return !(prefix === "text" && sizes.has(name));
        })
        .map((word) => `${path}:${literal.line}: ${word}`)
    )
  );
  expect(found).toEqual([]);
});
