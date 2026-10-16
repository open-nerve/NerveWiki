import { arrived } from "../app/arrival";
import type { TreeNode } from "../services/page.service";
import type { Located } from "./page/attachment-row";

/**
 * openOf is where a link to a node goes, of the notebook whose home is
 * home: a page to itself, arrived at; an attachment to where it is, under
 * parent, its parent page or the notebook's home, its row there taking the
 * focus (Located). The trash's Open of a node restored, the history's
 * links to its nodes (M8/P2, P3 design 4.2) and a search's hits (M8/P5
 * design 4.3) lead so.
 */
export function openOf(
  { id, kind, parent }: { id: string; kind: TreeNode["kind"]; parent: string | null },
  home: string
): { to: string; state: typeof arrived | Located } {
  if (kind === "page") {
    return { to: `${home}/pages/${id}`, state: arrived };
  }
  return { to: parent === null ? home : `${home}/pages/${parent}`, state: { attachment: id } };
}
