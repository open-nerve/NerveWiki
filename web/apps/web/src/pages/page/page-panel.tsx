import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { PageBacklinks } from "./page-backlinks";
import { PageOutline } from "./page-outline";
import { PageProperties } from "./page-properties";
import { PageVersions } from "./page-versions";

/**
 * PagePanel is the page's right column (M6/P7 design 7): its outline while
 * it is read, not while it is edited (M6 design 4.12), its backlinks and
 * its properties, each leading to a page by href, and its versions (M8
 * design 4.13), each leading to its version's page. From xl up it is
 * beside the page's content, at the top as the window scrolls, scrolling
 * itself when it is taller; narrower, after the content. Never the
 * window's scroll anchor: held at the top, it would not move as what is
 * above the reader in the page changes size, nor have the window follow.
 * Scrolling itself, it pads its edges for the focus rings.
 */
export function PagePanel({
  notebook,
  page,
  editing,
  href,
}: {
  notebook: Notebook;
  page: TreeNode;
  editing: boolean;
  href: (id: string) => string;
}) {
  const t = useT();
  return (
    <aside
      aria-label={t("page.panel")}
      className="min-w-0 space-y-4 xl:sticky xl:top-5 xl:-m-1 xl:max-h-[calc(100svh-2.5rem)] xl:overflow-y-auto xl:p-1 xl:[overflow-anchor:none]"
    >
      {!editing && <PageOutline notebook={notebook} page={page.id} />}
      <PageBacklinks notebook={notebook} page={page.id} href={href} />
      <PageProperties notebook={notebook} page={page.id} href={href} />
      <PageVersions notebook={notebook} page={page.id} href={(revision) => `${href(page.id)}/versions/${revision}`} />
    </aside>
  );
}
