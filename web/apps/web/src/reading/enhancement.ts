import { createContext } from "react";

import type { Translate } from "../i18n/i18n";
import type { NotebookRole } from "../services/notebook.service";
import { appLinks } from "./app-links";
import { codeHighlight, highlightWorker } from "./highlight";
import { scrollRegions } from "./scroll-regions";
import { taskToggle } from "./task-toggle";
import { unresolvedLinks } from "./unresolved-links";

/**
 * UnresolvedLink is a link to a page that is not there (nw-unresolved) as
 * a reader acts on it (M6/P6 design 7): its target as the view carries it
 * (data-nw-target), what it is (a link, an embed's or an image's), and its
 * element.
 */
export type UnresolvedLink = { target: string; kind: "link" | "embed" | "image"; element: HTMLElement };

/**
 * ReadingContext is what an enhancement knows of the reading view it runs
 * in (M4/P5 design 3.8): where the page is, the revision its HTML was
 * rendered from, the account's role in the notebook, the app's texts in
 * the reader's language, and a way to read the view again. An enhancement ticks a task item through it (M5/P6
 * design 3.5), goes to another address of the app (M6/P3 design 6.7),
 * hands it a link to a page that is not there (M6/P6 design 7), and
 * reports to the page what it could not do.
 */
export type ReadingContext = {
  workspace: string;
  notebook: string;
  page: string;
  revision: number;
  role: NotebookRole;
  /** t is the app's text of a key in the reader's language: an enhancement's names and labels. */
  t: Translate;
  reload: () => void;
  /** navigate goes to the app's address to through the router. */
  navigate: (to: string) => void;
  /**
   * toggleTask ticks (checked) or clears the task item at offset in the
   * view's revision, then reads the view again, and settles once it is
   * read; while a toggle of the page is out, it does nothing. It rejects
   * with the refusal, a revision passed once the view is read again. Only
   * a writer of the notebook's pages has it.
   */
  toggleTask?: (offset: number, checked: boolean) => Promise<void>;
  /** report hands error to the page, which says it as it says a refusal of Edit. */
  report: (error: unknown) => void;
  /**
   * unresolved has the view answer link, acted on: its dialog creates the
   * page for a writer, where the server says it would go, or says why it
   * is not there; or the view goes to the page the link leads to by now.
   */
  unresolved: (link: UnresolvedLink) => void;
};

/**
 * An Enhancement works on the reading view's HTML once it is in the page,
 * without changing its structure, and answers what undoes it (its
 * listeners, its work under way), or nothing. The HTML is the server's,
 * sanitized: an enhancement adds behaviour, never markup from elsewhere
 * unchecked.
 */
export type Enhancement = (container: HTMLElement, context: ReadingContext) => (() => void) | undefined;

/**
 * readingEnhancements are the app's enhancements, in the order they run
 * (M4 design 8): M4 has code highlighting, and the keyboard's way to what
 * scrolls sideways (scrollRegions since M6); M5 the task items' ticks; M6 the links into the app,
 * and those to pages not there; M7
 * adds its own here. The app's composition root (main.tsx) gives them to
 * the reading views through Enhancements; without it they have none.
 */
export const readingEnhancements: readonly Enhancement[] = [
  codeHighlight(highlightWorker),
  scrollRegions,
  taskToggle,
  appLinks,
  unresolvedLinks,
];

export const Enhancements = createContext<readonly Enhancement[]>([]);

/**
 * enhance runs enhancements on container in their order and answers what
 * undoes them all, in the reverse order. One that throws, running or
 * undone, is logged and leaves the others and the page be.
 */
export function enhance(
  enhancements: readonly Enhancement[],
  container: HTMLElement,
  context: ReadingContext
): () => void {
  const undo: (() => void)[] = [];
  for (const enhancement of enhancements) {
    try {
      const cleanup = enhancement(container, context);
      if (cleanup !== undefined) {
        undo.unshift(cleanup);
      }
    } catch (error) {
      console.error("A reading view's enhancement failed", error);
    }
  }
  return () => {
    for (const cleanup of undo) {
      try {
        cleanup();
      } catch (error) {
        console.error("A reading view's enhancement failed to clean up", error);
      }
    }
  };
}
