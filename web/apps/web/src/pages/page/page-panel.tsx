import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { PageOutline } from "./page-outline";

/**
 * PagePanel is the page's right column (M6/P7 design 7): its outline while
 * it is read, not while it is edited (M6 design 4.12). From xl up it is
 * beside the page's content, at the top as the window scrolls, scrolling
 * itself when it is taller; narrower, after the content. Never the
 * window's scroll anchor: held at the top, it would not move as what is
 * above the reader in the page changes size, nor have the window follow.
 */
export function PagePanel({ notebook, page, editing }: { notebook: Notebook; page: TreeNode; editing: boolean }) {
  const t = useT();
  return (
    <aside
      aria-label={t("page.panel")}
      className="min-w-0 space-y-4 xl:sticky xl:top-6 xl:max-h-[calc(100svh-3rem)] xl:overflow-y-auto xl:[overflow-anchor:none]"
    >
      {!editing && <PageOutline notebook={notebook.id} page={page.id} />}
    </aside>
  );
}
