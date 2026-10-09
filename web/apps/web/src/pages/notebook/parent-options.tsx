import { useT } from "../../i18n/i18n";
import type { TreeNode } from "../../services/page.service";
import type { PageTreeStore } from "../../stores/page-tree.store";

/** rootOption is the root's value among the parents' options. */
export const rootOption = "";

/**
 * ParentOptions are the options of a select of where a page or an
 * attachment goes: the notebook's top level, then each of parents by its
 * path in the tree, its ancestors' titles and its own.
 */
export function ParentOptions({ pages, parents }: { pages: PageTreeStore; parents: readonly TreeNode[] }) {
  const t = useT();
  return (
    <>
      <option value={rootOption}>{t("page.moveRoot")}</option>
      {parents.map((each) => (
        <option key={each.id} value={each.id}>
          {[...pages.ancestorsOf(each.id), each].map((step) => step.name).join(" / ")}
        </option>
      ))}
    </>
  );
}

/** Problem is a select's problem under it, which the select names as its description. */
export function Problem({ id, text }: { id: string; text: string | undefined }) {
  return text === undefined ? null : (
    <p id={id} className="text-sm text-destructive">
      {text}
    </p>
  );
}
