import {
  autocompletion,
  closeCompletion,
  pickedCompletion,
  type Completion,
  type CompletionContext,
  type CompletionResult,
  type CompletionSource,
} from "@codemirror/autocomplete";
import { syntaxTree } from "@codemirror/language";
import type { EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";

import type { Build, EditorContext } from "../registry";

/**
 * A link's target being written: after the last [[ of the line, what the
 * target's text may hold, to the cursor; the backslashes and the '!' before
 * it, which escape it or make it an embed.
 */
const linkBefore = /(\\*)(!?)\[\[([^[\]|#^\n]*)$/;

/** What a link's target being written may go on with, its completion kept. */
const linkGoesOn = /^[^[\]|#^\n]*$/;

/**
 * What follows the cursor in a link already closed: the rest of its target,
 * its anchor (after #), its display text (after | or a table's \|), and its
 * ]]. A '^' is the target's, as the server reads it (no title holds one).
 */
const closedRest = /^([^[\]|#\n]*?)((?:#[^[\]|\n]*?)?)((?:\\?\|[^[\]\n]*?)?)\]\]/;

/**
 * A tag being written: a '#' at the line's start or after a space (a
 * full-width one too), and a tag's characters to the cursor (fixtures'
 * rule 9; the server's isTagRune).
 */
const tagBefore = /(?:^|\s)#([\p{L}\p{M}\p{N}_\-/]*)$/u;

/** What a tag being written may go on with, its completion kept. */
const tagGoesOn = /^[\p{L}\p{M}\p{N}_\-/]*$/u;

/** The rest of a tag after the cursor: a pick writes over it. */
const tagRest = /^[\p{L}\p{M}\p{N}_\-/]*/u;

/** A tag the body can write, as #tag: a tag's characters, not digits alone, nor slashes alone (rule 9). */
const writableTag = /^(?![0-9]+$)(?!\/+$)[\p{L}\p{M}\p{N}_\-/]+$/u;

/** The nodes of lezer's Markdown whose text is no Markdown: nothing in them completes. */
const raw = new Set([
  "InlineCode",
  "FencedCode",
  "CodeBlock",
  "CodeText",
  "CodeInfo",
  "CodeMark",
  "HTMLBlock",
  "HTMLTag",
  "Comment",
  "CommentBlock",
  "ProcessingInstruction",
  "URL",
  "Autolink",
]);

/**
 * How long a read serves the completions of the same [[ or #: an input
 * method's compositions, and a query that matches nothing (whose results
 * CodeMirror drops at each key), start one anew each time.
 */
const servesMs = 10_000;

/** Where the cursor is, for a completion: in a table, in a frontmatter. */
type Place = { table: boolean; frontmatter: boolean };

/**
 * linkCompletion completes a link's target after [[ with the notebook's
 * pages and their aliases, and a tag after # with its tags (M6/P7 design
 * 4, 5), from the data the editor's context reads, once for each [[ or #.
 * Not in code or raw HTML, nor in a content that cannot be changed, nor
 * while an input method composes (design 6): its completion is closed as
 * one starts, and started again by CodeMirror as it ends having changed
 * the text. CodeMirror lets an input method have the keys while it
 * composes, Enter among them. In a frontmatter only a link in quotes
 * completes, the way a property link is written; no tag, as YAML reads a
 * comment there.
 */
export const linkCompletion: Build = (context) => [
  autocompletion({
    override: [pages(context), tags(context)],
    icons: false,
    addToOptions: [{ render: pause, position: 70 }],
  }),
  EditorView.domEventObservers({
    compositionstart: (_, view) => void closeCompletion(view),
  }),
  completionTheme,
];

/**
 * pause is what a screen reader hears between an option's text and its
 * detail, which it reads as one name otherwise ("topic/0176 pages"); it is
 * not seen.
 */
function pause(option: Completion): Node | null {
  if (option.detail === undefined) {
    return null;
  }
  const comma = document.createElement("span");
  comma.className = "nw-completion-pause";
  comma.textContent = ", ";
  return comma;
}

/** completionTheme draws the completion's list in the app's colours, light and dark (the editor's own are light). */
const completionTheme = EditorView.theme({
  ".cm-tooltip.cm-tooltip-autocomplete": {
    backgroundColor: "var(--popover)",
    color: "var(--popover-foreground)",
    border: "1px solid var(--border)",
    borderRadius: "calc(var(--radius) - 2px)",
  },
  ".cm-tooltip.cm-tooltip-autocomplete > ul": { fontFamily: "inherit", maxHeight: "16rem" },
  ".cm-tooltip.cm-tooltip-autocomplete > ul > li": { padding: "0.25rem 0.5rem" },
  ".cm-tooltip.cm-tooltip-autocomplete > ul > li[aria-selected]": {
    backgroundColor: "var(--accent)",
    color: "var(--accent-foreground)",
  },
  ".cm-completionDetail": { color: "var(--muted-foreground)", fontStyle: "normal", marginLeft: "0.75rem" },
  ".cm-completionMatchedText": { textDecoration: "none", fontWeight: "600" },
  // Of no size and in the flow: Chromium names an option out of the flow with spaces around it.
  ".nw-completion-pause": { fontSize: "0" },
});

/** placeOf is where completion is asked, if it may answer there: undefined where it may not. */
function placeOf({ state, pos, view }: CompletionContext): Place | undefined {
  if (state.readOnly || view?.composing === true) {
    return undefined;
  }
  const at = nodesAt(state, pos);
  const frontmatter = inFrontmatter(state, pos);
  // The editor parses a frontmatter as Markdown, which its YAML is not: no code nor raw HTML there.
  if (!frontmatter && at.some((name) => raw.has(name))) {
    return undefined;
  }
  return { table: at.includes("Table"), frontmatter };
}

/** nodesAt is the names of the syntax nodes pos is in, from the innermost out. */
function nodesAt(state: EditorState, pos: number): string[] {
  const names: string[] = [];
  for (let node: ReturnType<typeof syntaxTree>["topNode"] | null = syntaxTree(state).resolveInner(pos, -1); node;) {
    names.push(node.name);
    node = node.parent;
  }
  return names;
}

/**
 * inFrontmatter tells whether pos is in the content's frontmatter, as the
 * server's frontmatterSpan finds it: a first line "---", up to a later
 * line "---" ("..." closes nothing). The content the editor loads has
 * no byte order mark (line-breaks.ts); one pasted at its start is not
 * looked for (accepted). One not closed, which the server reads as the
 * body until it is, is one being written as far as a blank line: a
 * link or a tag written there as the body's would be nothing once it is
 * closed.
 */
function inFrontmatter(state: EditorState, pos: number): boolean {
  const { doc } = state;
  const at = doc.lineAt(pos).number;
  if (at === 1 || doc.line(1).text !== "---") {
    return false;
  }
  let empty: number | undefined;
  for (let n = 2; n <= doc.lines; n++) {
    const { text } = doc.line(n);
    if (text === "---") {
      return at < n;
    }
    if (empty === undefined && /^[ \t]*$/.test(text)) {
      empty = n;
    }
  }
  return at < (empty ?? doc.lines + 1);
}

/**
 * inLinkWritten tells whether a '#' at the end of before, the line up to it
 * from from, is in a link being written: after a [[ not closed on the line,
 * in a table its cell (after its last '|' no backslash escapes), which no
 * backslash escapes and is no code's nor raw HTML's.
 */
function inLinkWritten(state: EditorState, from: number, before: string, table: boolean): boolean {
  const cell = table ? ([...before.matchAll(/(?<!\\)\|/g)].at(-1)?.index ?? -1) + 1 : 0;
  const opens = before.lastIndexOf("[[");
  if (opens < cell || before.includes("]]", opens)) {
    return false;
  }
  const slashes = /\\*$/.exec(before.slice(0, opens))?.[0].length ?? 0;
  return slashes % 2 === 0 && !nodesAt(state, from + opens + 1).some((name) => raw.has(name));
}

/** lineBefore is the cursor's line up to it. */
function lineBefore({ state, pos }: CompletionContext): string {
  return state.sliceDoc(state.doc.lineAt(pos).from, pos);
}

/** pages completes a link's target with the pages, by their link (a title, or its path), and their aliases. */
function pages(context: EditorContext): CompletionSource {
  const reading = remembered(() => context.linkTargets());
  return async (completion): Promise<CompletionResult | null> => {
    const before = lineBefore(completion);
    const written = linkBefore.exec(before);
    if (written === null) {
      return null;
    }
    const [whole, slashes = "", bang = "", query = ""] = written;
    const opens = before.length - whole.length + slashes.length + bang.length;
    const place = placeOf(completion);
    if (
      place === undefined ||
      (bang === "" && slashes.length % 2 === 1) ||
      (place.frontmatter && !/["']$/.test(before.slice(0, opens)))
    ) {
      return null;
    }
    const from = completion.pos - query.length;
    const targets = await reading(from, completion.explicit);
    if (targets === undefined || completion.aborted) {
      return null;
    }
    // An embed shows its page: an alias would be its display text, which an embed takes for a size.
    const embed = bang === "!" && slashes.length % 2 === 0;
    const separator = place.table ? "\\|" : "|";
    const options: Completion[] = [];
    for (const target of targets) {
      // Matched by its link, which holds its title: a path for a title others share.
      options.push({
        label: target.link,
        displayLabel: target.name,
        detail: target.link === target.name ? undefined : target.link,
        apply: writing(target.link, undefined, separator),
      });
      for (const alias of embed ? [] : target.aliases) {
        // One with a bracket or a line's end would end the link it is written in; in a table, one with a '|' the cell.
        if (!(place.table ? /[[\]\r\n|]/ : /[[\]\r\n]/).test(alias)) {
          options.push({ label: alias, detail: `→ ${target.link}`, apply: writing(target.link, alias, separator) });
        }
      }
    }
    return { from, options, validFor: linkGoesOn, getMatch: shownMatch };
  };
}

/** tags completes a tag with the notebook's tags the body can write, each with how many pages have it. */
function tags(context: EditorContext): CompletionSource {
  const reading = remembered(() => context.tags());
  return async (completion): Promise<CompletionResult | null> => {
    const before = lineBefore(completion);
    const written = tagBefore.exec(before);
    const place = written === null ? undefined : placeOf(completion);
    const line = completion.state.doc.lineAt(completion.pos).from;
    // In a link being written, a '#' is its anchor's.
    if (
      written === null ||
      place === undefined ||
      place.frontmatter ||
      inLinkWritten(completion.state, line, before, place.table)
    ) {
      return null;
    }
    const from = completion.pos - (written[1] ?? "").length;
    const counted = await reading(from, completion.explicit);
    if (counted === undefined || completion.aborted) {
      return null;
    }
    const options: Completion[] = [];
    for (const { tag, count } of counted) {
      if (writableTag.test(tag)) {
        options.push({
          label: tag,
          detail: completion.state.phrase(count === 1 ? "$ page" : "$ pages", count),
          apply: tagWriting(tag),
        });
      }
    }
    return { from, options, validFor: tagGoesOn };
  };
}

/**
 * remembered reads through reading once for the completions that start at
 * the same place within servesMs: those of one [[ or # that compositions
 * and keys start anew. One asked for (Ctrl+Space; on macOS, where the
 * system often takes it, Option+` or Option+I too) reads anew. Within the
 * while, a page made elsewhere meanwhile is not listed at the same place
 * (accepted). A failed read is said on the console, answers undefined (no
 * completion), and is not remembered.
 */
function remembered<T>(reading: () => Promise<T>): (from: number, explicit: boolean) => Promise<T | undefined> {
  let last: { from: number; at: number; read: Promise<T | undefined> } | undefined;
  return (from, explicit) => {
    const now = Date.now();
    if (!explicit && last !== undefined && last.from === from && now - last.at < servesMs) {
      return last.read;
    }
    const read = reading().catch((error: unknown) => {
      console.error("The completion's data could not be read", error);
      if (last?.read === read) {
        last = undefined;
      }
      return undefined;
    });
    last = { from, at: now, read };
    return read;
  };
}

/** shownMatch is where a page's option matched, in its title as shown: in the link it is matched by, its title's last. */
function shownMatch(option: Completion, matched: readonly number[] = []): readonly number[] {
  if (option.displayLabel === undefined) {
    return matched;
  }
  const shift = option.label.lastIndexOf(option.displayLabel);
  if (shift === -1) {
    return [];
  }
  const ranges: number[] = [];
  for (let i = 0; i + 1 < matched.length; i += 2) {
    const from = Math.max((matched[i] ?? 0) - shift, 0);
    const to = Math.min((matched[i + 1] ?? 0) - shift, option.displayLabel.length);
    if (to > from) {
      ranges.push(from, to);
    }
  }
  return ranges;
}

/**
 * writing is a link's apply: the target written over what was typed, as
 * [[link]], or [[link|alias]] (\| in a table, rule 3), the cursor after
 * it. In a link already closed, only its target's rest is written over:
 * its anchor stays, and its display text unless an alias is picked; no
 * ]] is written twice.
 */
function writing(link: string, alias: string | undefined, separator: string) {
  return (view: EditorView, completion: Completion, from: number, to: number) => {
    const rest = closedRest.exec(view.state.sliceDoc(to, view.state.doc.lineAt(to).to));
    let insert = alias === undefined ? `${link}]]` : `${link}${separator}${alias}]]`;
    let [end, cursor] = [to, from + insert.length];
    if (rest !== null) {
      const [whole, target = "", anchor = "", display = ""] = rest;
      insert = alias === undefined ? link : `${link}${anchor}${separator}${alias}`;
      end = to + target.length + (alias === undefined ? 0 : anchor.length + display.length);
      cursor = from + insert.length + (alias === undefined ? whole.length - target.length : 2);
    }
    view.dispatch({
      changes: { from, to: end, insert },
      selection: { anchor: cursor },
      userEvent: "input.complete",
      annotations: pickedCompletion.of(completion),
    });
  };
}

/** tagWriting is a tag's apply: the tag written over what was typed, and over the rest of the tag the cursor was in. */
function tagWriting(tag: string) {
  return (view: EditorView, completion: Completion, from: number, to: number) => {
    const rest = tagRest.exec(view.state.sliceDoc(to, view.state.doc.lineAt(to).to))?.[0] ?? "";
    view.dispatch({
      changes: { from, to: to + rest.length, insert: tag },
      selection: { anchor: from + tag.length },
      userEvent: "input.complete",
      annotations: pickedCompletion.of(completion),
    });
  };
}
