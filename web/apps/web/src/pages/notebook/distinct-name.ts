import type { Translate } from "../../i18n/i18n";
import { placeOfTitle, type TreeIndex } from "../../stores/page-tree";

/**
 * distinctName is how the tree names the page id (v0.1 design 13.2, item
 * 17): its title, and where another page of the notebook has the same,
 * also the pages it is under, or, at the root, that it is at the notebook's
 * top level, as the trash and the move say it, not by the notebook's name,
 * which a root page may have too (M12/P2/S2); undefined for a page the tree
 * does not have.
 */
export function distinctName(tree: TreeIndex | undefined, id: string, t: Translate): string | undefined {
  const node = tree?.byId.get(id);
  if (tree === undefined || node === undefined) {
    return undefined;
  }
  const place = placeOfTitle(tree, id);
  if (place === undefined) {
    return node.name;
  }
  return place === "" ? t("page.nameAtTop", { name: node.name }) : t("page.nameIn", { name: node.name, place });
}
