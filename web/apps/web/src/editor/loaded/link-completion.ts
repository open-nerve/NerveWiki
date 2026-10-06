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
import { EditorView } from "@codemirror/view";

import type { Build, EditorContext } from "../registry";

/** A link's target being written: after the last [[ of the line, what the target's text may hold, to the cursor. */
const linkBefore = /\[\[([^[\]|#^\n]*)$/;

/** What a link's target being written may go on with, its completion kept. */
const linkGoesOn = /^[^[\]|#^\n]*$/;

/**
 * A tag being written: a '#' at the line's start or after a space (a
 * full-width one too), and a tag's characters to the cursor (fixtures'
 * rule 9; the server's isTagRune).
 */
const tagBefore = /(?:^|\s)#([\p{L}\p{M}\p{N}_\-/]*)$/u;

/** What a tag being written may go on with, its completion kept. */
const tagGoesOn = /^[\p{L}\p{M}\p{N}_\-/]*$/u;

/** The nodes of lezer's Markdown that are code: nothing in them completes. */
const code = new Set(["InlineCode", "FencedCode", "CodeBlock", "CodeText", "CodeInfo", "CodeMark"]);

/**
 * linkCompletion completes a link's target after [[ with the notebook's
 * pages and their aliases, and a tag after # with its tags (M6/P7 design
 * 4, 5), from the data the editor's context reads, once a completion. Not
 * in code, nor in a content that cannot be changed, nor while an input
 * method composes (design 6): its completion is closed as one starts, and
 * started again by CodeMirror as it ends having changed the text.
 * CodeMirror lets an input method have the keys while it composes, Enter
 * among them.
 */
export const linkCompletion: Build = (context) => [
  autocompletion({ override: [pages(context), tags(context)], icons: false }),
  EditorView.domEventObservers({
    compositionstart: (_, view) => void closeCompletion(view),
  }),
];

/** completes tells whether completion may answer where it is asked. */
function completes({ state, pos, view }: CompletionContext): boolean {
  if (state.readOnly || view?.composing === true) {
    return false;
  }
  const inner = syntaxTree(state).resolveInner(pos, -1);
  for (let node: typeof inner | null = inner; node !== null; node = node.parent) {
    if (code.has(node.name)) {
      return false;
    }
  }
  return true;
}

/** lineBefore is the cursor's line up to it. */
function lineBefore({ state, pos }: CompletionContext): string {
  return state.sliceDoc(state.doc.lineAt(pos).from, pos);
}

/** pages completes a link's target with the pages, by their title, and their aliases. */
function pages(context: EditorContext): CompletionSource {
  return async (completion): Promise<CompletionResult | null> => {
    const written = linkBefore.exec(lineBefore(completion));
    if (written === null || !completes(completion)) {
      return null;
    }
    const targets = await read(() => context.linkTargets());
    if (targets === undefined || completion.aborted) {
      return null;
    }
    const options: Completion[] = [];
    for (const target of targets) {
      // Matched by its link, which ends with its title, as written for a title others share.
      options.push({
        label: target.link,
        displayLabel: target.name,
        detail: target.link === target.name ? undefined : target.link,
        apply: closing(`${target.link}]]`),
      });
      for (const alias of target.aliases) {
        options.push({ label: alias, detail: `→ ${target.name}`, apply: closing(`${target.link}|${alias}]]`) });
      }
    }
    return { from: completion.pos - (written[1] ?? "").length, options, validFor: linkGoesOn };
  };
}

/** tags completes a tag with the notebook's tags, each with how many pages have it. */
function tags(context: EditorContext): CompletionSource {
  return async (completion): Promise<CompletionResult | null> => {
    const written = tagBefore.exec(lineBefore(completion));
    if (written === null || !completes(completion)) {
      return null;
    }
    const counted = await read(() => context.tags());
    if (counted === undefined || completion.aborted) {
      return null;
    }
    const options = counted.map(({ tag, count }): Completion => ({
      label: tag,
      detail: completion.state.phrase("$ pages", count),
    }));
    return { from: completion.pos - (written[1] ?? "").length, options, validFor: tagGoesOn };
  };
}

/** read answers what reading answers, or undefined, said on the console, when it fails: no completion then. */
async function read<T>(reading: () => Promise<T>): Promise<T | undefined> {
  try {
    return await reading();
  } catch (error) {
    console.error("The completion's data could not be read", error);
    return undefined;
  }
}

/**
 * closing is a completion's apply that writes text over what was typed, a
 * link's ]] after the cursor with it: none is written twice. The cursor
 * goes after it.
 */
function closing(text: string) {
  return (view: EditorView, completion: Completion, from: number, to: number) => {
    const end = text.endsWith("]]") && view.state.sliceDoc(to, to + 2) === "]]" ? to + 2 : to;
    view.dispatch({
      changes: { from, to: end, insert: text },
      selection: { anchor: from + text.length },
      userEvent: "input.complete",
      annotations: pickedCompletion.of(completion),
    });
  };
}
