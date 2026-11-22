import { readFileSync } from "node:fs";
import { URL as FileURL } from "node:url";
import { parseSync } from "vite";
import { expect, test } from "vitest";

// The design tokens' conventions (M13/P1/S1, decision 751; v0.1 design 13.2 item 32): both themes have every colour,
// amber marks a state and fills nothing, components name the theme's colours, never a palette's nor a written value.
// The stylesheets are read from the disk: vitest serves a stylesheet's ?raw as "" (P1 design 1). The sources are read
// as the bundler sees them, the tests, their helpers and the fakes under testing/ left out, and parsed: what counts is
// in their string literals, not in their comments.

const readCss = (path: string) => readFileSync(new FileURL(path, import.meta.url), "utf8");
const styles = readCss("./styles.css");
const reading = readCss("./reading/reading.css");
const sources = Object.entries(
  import.meta.glob<string>(["./**/*.{ts,tsx}", "!./**/*.test.{ts,tsx}", "!./test/**", "!./**/testing/**"], {
    query: "?raw",
    import: "default",
    eager: true,
  })
);

/** read answers the stylesheets and the sources, once it has made sure they were read: a check of nothing passes. */
function read(): { styles: string; reading: string; sources: [string, string][] } {
  expect(styles.length).toBeGreaterThan(1000);
  expect(reading.length).toBeGreaterThan(1000);
  expect(sources.length).toBeGreaterThan(250);
  expect(sources.map(([path]) => path)).toContain("./components/ui/button.tsx");
  return { styles, reading, sources };
}

/** hits are the lines of text, named by file, on which a pattern matches, each as "file:line: the line". */
function hits(file: string, text: string, ...patterns: RegExp[]): string[] {
  const lines = text.split("\n");
  const found = new Set<string>();
  for (const pattern of patterns) {
    for (const match of text.matchAll(new RegExp(pattern.source, `${pattern.flags.replace("g", "")}g`))) {
      const line = text.slice(0, match.index).split("\n").length;
      found.add(`${file}:${line}: ${lines[line - 1]?.trim() ?? ""}`);
    }
  }
  return [...found];
}

/** uncommented is css with its comments blanked, its lines where they were. */
function uncommented(css: string): string {
  return css.replaceAll(/\/\*[\s\S]*?\*\//g, (comment) => comment.replaceAll(/[^\n]/g, " "));
}

/** Block is a rule at the top of a stylesheet, outside any other: what is before its {, and what is in it. */
type Block = { prelude: string; body: string };

/** blocks are css's top rules, its comments left out; a statement (@import, @custom-variant) is none. */
function blocks(css: string): Block[] {
  const text = uncommented(css);
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

/** themed are the names of the custom properties of prefix (--text-) the theme declares, @theme inline or not. */
function themed(css: string, prefix: string): Set<string> {
  const names = new Set<string>();
  for (const block of blocks(css).filter((b) => /^@theme(?:\s+inline)?$/.test(b.prelude))) {
    for (const [, name] of block.body.matchAll(new RegExp(String.raw`${prefix}([\w-]+)\s*:`, "g"))) {
      names.add(name ?? "");
    }
  }
  return names;
}

/** Literal is a string literal of a source and the line it starts on. */
type Literal = { text: string; line: number };

/**
 * Parsed is what a source's string literals say: all of them, a module's path not one, each part of a template one,
 * and a property whose value is one also as "key: value", as a style object declares it; and those in a class list.
 */
type Parsed = { literals: Literal[]; classes: Literal[] };

/** Node is a node of the parser's tree, as much of it as parsed reads. */
type Node = { type: string; start: number; [key: string]: unknown };

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
/** What is called with class lists. */
const classFunctions = new Set(["cn", "cva", "clsx"]);

const nodeOf = (value: unknown): Node | undefined =>
  typeof value === "object" && value !== null && typeof (value as Node).type === "string" ? (value as Node) : undefined;
/** nameOf is an identifier's name, a member's property's, a key's text. */
const nameOf = (value: unknown): string | undefined => {
  const node = nodeOf(value);
  if (node === undefined) return undefined;
  if (node.type === "MemberExpression") return nameOf(node.property);
  if (typeof node.name === "string") return node.name;
  return typeof node.value === "string" ? node.value : undefined;
};

/**
 * classesUnder is whether what is under node's key is a class list: a className (JSX's, the DOM's), what cn, cva and
 * clsx are given (their nested arrays, objects, templates and conditionals with it), a classList's classes, and what a
 * name that ends in Class or ClassName is set to (the diff's buttonClass).
 */
function classesUnder(node: Node, key: string): boolean {
  switch (node.type) {
    case "JSXAttribute":
      return key === "value" && nameOf(node.name) === "className";
    case "CallExpression": {
      const callee = nodeOf(node.callee);
      const ofClassList =
        callee?.type === "MemberExpression" && nameOf(nodeOf(callee.object)?.property) === "classList";
      return key === "arguments" && (classFunctions.has(nameOf(callee) ?? "") || ofClassList);
    }
    case "AssignmentExpression":
      return key === "right" && nameOf(node.left) === "className";
    case "VariableDeclarator":
      return key === "init" && /[cC]lass(?:Name)?$/.test(nameOf(node.id) ?? "");
    default:
      return false;
  }
}

const parsedOf = new Map<string, Parsed>();

/** parsed is what source's string literals say (Parsed), parsed once. */
function parsed(path: string, source: string): Parsed {
  const known = parsedOf.get(path);
  if (known !== undefined) return known;
  const { program, errors } = parseSync(path, source);
  expect(errors, path).toEqual([]);
  const literals: Literal[] = [];
  const classes: Literal[] = [];
  const lineOf = (at: number) => source.slice(0, at).split("\n").length;
  const found = (literal: Literal, inClasses: boolean) => {
    literals.push(literal);
    if (inClasses) classes.push(literal);
  };
  const visit = (node: Node, inClasses: boolean, parent?: Node): void => {
    if (node.type === "Literal" && typeof node.value === "string") {
      if (!modulePathOf.has(parent?.type ?? "")) found({ text: node.value, line: lineOf(node.start) }, inClasses);
    } else if (node.type === "TemplateElement") {
      const { cooked, raw } = node.value as { cooked: string | null; raw: string };
      found({ text: cooked ?? raw, line: lineOf(node.start) }, inClasses);
    } else if (node.type === "Property" && node.computed !== true) {
      const value = nodeOf(node.value);
      const text =
        value?.type === "Literal" && typeof value.value === "string"
          ? value.value
          : value?.type === "TemplateLiteral"
            ? (value.quasis as Node[]).map((quasi) => (quasi.value as { raw: string }).raw).join("_")
            : undefined;
      const key = nameOf(node.key);
      if (text !== undefined && key !== undefined) literals.push({ text: `${key}: ${text}`, line: lineOf(node.start) });
    }
    for (const [key, child] of Object.entries(node)) {
      if (key === "parent") continue;
      const under = inClasses || classesUnder(node, key);
      for (const each of Array.isArray(child) ? child : [child]) {
        const next = nodeOf(each);
        if (next !== undefined) visit(next, under, node);
      }
    }
  };
  visit(program as unknown as Node, false);
  const answer = { literals, classes };
  parsedOf.set(path, answer);
  return answer;
}

/**
 * walkedRight makes sure the parse reads what it is to read: a JSX attribute's class list and a cva string, both as
 * literals and as classes. A walk that read none would find nothing wrong anywhere.
 */
function walkedRight(files: [string, string][]): void {
  const words = (path: string, of: keyof Parsed) =>
    parsed(path, files.find(([at]) => at === path)?.[1] ?? "")[of].flatMap(({ text }) => text.split(/\s+/));
  for (const of of ["literals", "classes"] as const) {
    expect(words("./components/ui/dialog.tsx", of), of).toContain("bg-overlay");
    expect(words("./components/ui/button.tsx", of), of).toContain("bg-primary");
  }
}

/** literalHits are the literals of files on which a pattern matches, each as "file:line: the literal". */
function literalHits(files: [string, string][], patterns: RegExp[], skip: Set<string> = new Set()): string[] {
  return files
    .filter(([path]) => !skip.has(path))
    .flatMap(([path, source]) =>
      parsed(path, source)
        .literals.filter(({ text }) => patterns.some((pattern) => new RegExp(pattern.source, pattern.flags).test(text)))
        .map(({ text, line }) => `${path}:${line}: ${text}`)
    );
}

test("every colour of the light theme is the dark theme's too", () => {
  const { styles: css } = read();
  const light = declared(css, ":root");
  // What the theme maps is declared: a variable gone from both themes, or misspelt in the map, names nothing.
  const mapped = blocks(css)
    .filter((block) => block.prelude === "@theme inline")
    .flatMap((block) => [...block.body.matchAll(/var\((--[\w-]+)\)/g)].map(([, name]) => name ?? ""));
  expect(mapped.length).toBeGreaterThanOrEqual(20);
  expect(mapped.filter((name) => !light.has(name))).toEqual([]);
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
  const { styles: css, reading: view, sources: files } = read();
  const found = [
    ...hits("./styles.css", css, /accent-foreground/),
    ...hits("./reading/reading.css", view, /accent-foreground/),
    ...files.flatMap(([path, source]) => hits(path, source, /accent-foreground/)),
  ];
  expect(found).toEqual([]);
});

// Amber, or the ring that is amber, as a background: a class, bg-accent, bg-(--accent), bg-[var(--ring)]; a
// declaration, in a stylesheet or a style object, its colour or its image (a gradient) of it. Its tint, bg-accent-soft
// or var(--accent-soft), is the current row's.
const amberClasses = [/\bbg-(?:accent|ring)(?![-\w])/, /\bbg-(?:\(|\[var\()--(?:color-)?(?:accent|ring)(?![-\w])/];
const amberBackground =
  /background(?:-?[cC]olor|-?[iI]mage)?["']?\s*:\s*["'`]?[^"'`;{}]*var\(--(?:color-)?(?:accent|ring)\)/;

test("amber is no fill", () => {
  const { styles: css, reading: view, sources: files } = read();
  walkedRight(files);
  const found = [
    ...hits("./styles.css", uncommented(css), ...amberClasses, amberBackground),
    ...hits("./reading/reading.css", uncommented(view), ...amberClasses, amberBackground),
    ...literalHits(files, [...amberClasses, amberBackground]),
  ];
  expect(found).toEqual([]);
});

// A palette's colour as a class: bg-black/50, text-white, border-red-500 and the like.
const paletteClass =
  /\b(?:bg|text|border|ring|outline|fill|stroke|from|to|via|decoration|divide|placeholder|caret|accent|shadow)-(?:black|white|slate|gray|zinc|neutral|stone|mauve|mist|olive|taupe|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)(?:-\d{2,3})?(?:\/\d+)?\b/;
// A colour written in a class's arbitrary value: bg-[#000], text-[rgb(0_0_0)], bg-[lab(50%_40_59)].
const arbitraryColour = /-\[(?:#|(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color|color-mix)\()/;
// A colour written as a value: #rgb, #rgba, #rrggbb, #rrggbbaa or a colour function, where it is one, what a literal
// starts with or what follows a ":" in a declaration; not an "#add" in a sentence. A named colour ("black") is not
// looked for: English has the same words.
const colourValue = String.raw`(?:#(?:[\da-f]{8}|[\da-f]{6}|[\da-f]{3,4})(?![\w-])|(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color|color-mix)\()`;
const writtenColour = new RegExp(String.raw`^\s*${colourValue}|:[^;{}]*?(?<![\w#-])${colourValue}`, "i");
// Where a colour may be written (v0.1 design 13.2 item 32): mermaid's themeVariables take hexadecimal colours alone;
// the editor's links and matches until P4/S3.
const writtenColourMayBe = new Set(["./editor/theme.ts", "./reading/diagrams.ts"]);

test("components use the theme's colours, not Tailwind's palette nor written values", () => {
  const { sources: files } = read();
  walkedRight(files);
  const found = [
    ...literalHits(files, [paletteClass, arbitraryColour]),
    ...literalHits(files, [writtenColour], writtenColourMayBe),
  ];
  expect(found).toEqual([]);
});

// The classes of these prefixes in the class lists that name no colour, as Tailwind spells them: sizes, styles, an
// alignment, the shadows Tailwind has. A number (a width, an offset, a stop) or a side alone (border-b) is none either.
const noColour = new Set([
  "decoration-dashed",
  "outline-none",
  "ring-inset",
  "shadow-lg",
  "shadow-md",
  "shadow-xs",
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
const colourClass =
  /^(bg|text|border|ring|outline|fill|stroke|divide|decoration|caret|placeholder|from|via|to|shadow)-(.+)$/;

/**
 * utility is a word of a class list with its variants (hover:, [&_svg]:, outside brackets and parentheses), its
 * importance (!) and its opacity (/50, /[0.5], /(--a)) taken off, and a side (border-t-) or an offset (ring-offset-)
 * before a colour too: border-t-border names the colour border.
 */
function utility(word: string): string {
  let depth = 0;
  let after = 0;
  for (let at = 0; at < word.length; at++) {
    const c = word[at];
    if (c === "[" || c === "(") depth++;
    else if (c === "]" || c === ")") depth--;
    else if (c === ":" && depth === 0) after = at + 1;
  }
  return word
    .slice(after)
    .replace(/^!/, "")
    .replace(/!$/, "")
    .replace(/\/(?:[\w.%]+|\[[^\]]*\]|\([^)]*\))$/, "")
    .replace(/^border-[xytrblse]-(?=.)/, "border-")
    .replace(/^(ring|outline)-offset-(?=.)/, "$1-");
}

test("every colour class names a colour of the theme", () => {
  const { styles: css, sources: files } = read();
  walkedRight(files);
  const colours = new Set(
    blocks(css)
      .filter((block) => block.prelude === "@theme inline")
      .flatMap((block) => [...block.body.matchAll(/--color-([\w-]+)\s*:/g)].map(([, name]) => name ?? ""))
  );
  const sizes = themed(css, "--text-");
  const shadows = themed(css, "--shadow-");
  expect(colours).toContain("background");
  expect(colours).toContain("destructive");
  expect(sizes).toContain("row");
  expect(shadows).toContain("popover");
  const named = (word: string): boolean => {
    const match = colourClass.exec(word);
    if (match === null || noColour.has(word)) return true;
    const [, prefix, value = ""] = match;
    // An arbitrary value, bg-(--nw-mark-background) or text-[13px], is the class's own.
    if (/[[(]/.test(value)) return true;
    if (/^(?:\d+(?:\.\d+)?%?|px)$/.test(value)) return true;
    if ((prefix === "border" && /^[xytrblse]$/.test(value)) || (prefix === "divide" && /^[xy]$/.test(value)))
      return true;
    if (noThemeColour.has(value) || colours.has(value)) return true;
    return (prefix === "text" && sizes.has(value)) || (prefix === "shadow" && shadows.has(value));
  };
  const found = files.flatMap(([path, source]) =>
    parsed(path, source).classes.flatMap(({ text, line }) =>
      text
        .split(/\s+/)
        .filter((word) => !named(utility(word)))
        .map((word) => `${path}:${line}: ${word}`)
    )
  );
  expect(found).toEqual([]);
});
