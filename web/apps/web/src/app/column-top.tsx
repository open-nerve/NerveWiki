import { createContext, use, type ReactNode } from "react";

/**
 * What a shell puts at the top of its page's column: a notebook's upgrade banner (M13/P4/S1, decision 857), which
 * NotebookLayout gives every page of a notebook (M13/P5, decision 915).
 */
export const ColumnTopContext = createContext<ReactNode>(null);

/** ColumnTop is what the shell put at the top of the page's column, if anything: its own component, rendered alone. */
export function ColumnTop() {
  return use(ColumnTopContext);
}
