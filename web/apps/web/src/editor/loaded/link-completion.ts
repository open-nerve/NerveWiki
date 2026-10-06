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
 * its anchor, its display text (after | or a table's \|), and its ]].
 */
const closedRest = /^([^[\]|#^\n]*?)((?:[#^][^[\]|\n]*?)?)((?:\\?\|[^[\]\n]*?)?)\]\]/;

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

/** The lines that open and close a frontmatter (fixtures' rule 10). */
const frontmatterOpens = /^---[ \t]*$/;
const frontmatterCloses = /^(?:---|\.\.\.)[ \t]*$/;

/** How long a read serves the completions of the same [[ or #: a composition ended starts one anew. */
const servesMs = 30_000;

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
  ".nw-completion-pause": {
    position: "absolute",
    width: "1px",
    height: "1px",
    overflow: "hidden",
    clipPath: "inset(50%)",
    whiteSpace: "nowrap",
  },
});

/** placeOf is where completion is asked, if it may answer there: undefined where it may not. */
function placeOf({ state, pos, view }: CompletionContext): Place | undefined {
  if (state.readOnly || view?.composing === true) {
    return undefined;
  }
  let table = false;
  for (let node: ReturnType<typeof syntaxTree>["topNode"] | null = syntaxTree(state).resolveInner(pos, -1); node;) {
    if (raw.has(node.name)) {
      return undefined;
    }
    table ||= node.name === "Table";
    node = node.parent;
  }
  return { table, frontmatter: inFrontmatter(state, pos) };
}

/** inFrontmatter tells whether pos is in the content's frontmatter: after its opening line, before its closing one. */
function inFrontmatter(state: EditorState, pos: number): boolean {
  const { doc } = state;
  const at = doc.lineAt(pos).number;
  if (at === 1 || !frontmatterOpens.test(doc.line(1).text)) {
    return false;
  }
  for (let n = 2; n < at; n++) {
    if (frontmatterCloses.test(doc.line(n).text)) {
      return false;
    }
  }
  return !frontmatterCloses.test(doc.line(at).text);
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
    const targets = await reading(from);
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
        // One with a bracket or a line's end would end the link it is written in.
        if (!/[[\]\r\n]/.test(alias)) {
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
    // In a link being written, a '#' is its anchor's.
    if (written === null || place === undefined || place.frontmatter || /\[\[(?:(?!\]\]).)*$/.test(before)) {
      return null;
    }
    const from = completion.pos - (written[1] ?? "").length;
    const counted = await reading(from);
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
 * the same place within servesMs: those of one [[ or # that an input
 * method's compositions start anew. A failed read is said on the console,
 * answers undefined (no completion), and is not remembered.
 */
function remembered<T>(reading: () => Promise<T>): (from: number) => Promise<T | undefined> {
  let last: { from: number; at: number; read: Promise<T | undefined> } | undefined;
  return (from) => {
    const now = Date.now();
    if (last !== undefined && last.from === from && now - last.at < servesMs) {
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
