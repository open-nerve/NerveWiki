import { NavItem } from "../../components/nav-item";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import { useWorkspace } from "../workspace/workspace-layout";

/**
 * NotebookPlaces is the notebook's places other than its pages in the
 * left column, below its page tree (M8 design 4.13; M8/P2 design 4.5): its
 * trash, P3 its history. A navigation of its own, named: the tree's links
 * are its pages, each, as the tree's tests and readers take them.
 */
export function NotebookPlaces({ notebook }: { notebook: Notebook }) {
  const t = useT();
  const { slug } = useWorkspace();
  return (
    <nav aria-label={t("trash.places", { notebook: notebook.name })} className="flex flex-col">
      <NavItem to={`/${slug}/notebooks/${notebook.id}/trash`}>{t("trash.title")}</NavItem>
    </nav>
  );
}
