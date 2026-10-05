import type { Translate } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import { placeOfTitle, type TreeIndex } from "../../stores/page-tree";

/**
 * distinctName is how the tree names the page id (v0.1 design 13.2, item
 * 17): its title, and where another page of the notebook has the same,
 * also the pages it is under, or the notebook at the root; undefined for a
 * page the tree does not have.
 */
export function distinctName(
  tree: TreeIndex | undefined,
  notebook: Notebook,
  id: string,
  t: Translate
): string | undefined {
  const node = tree?.byId.get(id);
  if (tree === undefined || node === undefined) {
    return undefined;
  }
  const place = placeOfTitle(tree, id);
  return place === undefined ? node.name : t("page.nameIn", { name: node.name, place: place || notebook.name });
}
