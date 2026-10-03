import { createContext } from "react";

import type { NotebookRole } from "../services/notebook.service";
import { codeHighlight, highlightWorker } from "./highlight";
import { scrollFocus } from "./scroll-focus";

/**
 * ReadingContext is what an enhancement knows of the reading view it runs
 * in (M4/P5 design 3.8): where the page is, the revision its HTML was
 * rendered from, the account's role in the notebook, and a way to read
 * the view again.
 */
export type ReadingContext = {
  workspace: string;
  notebook: string;
  page: string;
  revision: number;
  role: NotebookRole;
  reload: () => void;
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
 * scrolls sideways; M5, M6 and M7 add theirs here.
 * The app's composition root (main.tsx) gives them to the reading views
 * through Enhancements; without it they have none.
 */
export const readingEnhancements: readonly Enhancement[] = [codeHighlight(highlightWorker), scrollFocus];

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
