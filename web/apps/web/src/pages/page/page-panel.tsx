import { observer } from "mobx-react-lite";

import { useBeside } from "../../app/narrow";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { EditOutline, type OutlineBox } from "./edit-outline";
import { PageBacklinks } from "./page-backlinks";
import { PageOutline } from "./page-outline";
import { PageProperties } from "./page-properties";
import { PageVersions } from "./page-versions";

/**
 * ColumnOutline is the right column's outline: the view's while the page is read, and while it is edited until its
 * editor first tells its headings (its content read meanwhile), measuring nothing then (review A's P4S3A-M1); the
 * editor's after.
 */
const ColumnOutline = observer(function ColumnOutline({
  editing,
  outline,
  notebook,
  page,
  active,
}: {
  editing: boolean;
  outline: OutlineBox;
  notebook: Notebook;
  page: string;
  active: boolean;
}) {
  return editing && outline.ready ? (
    <EditOutline box={outline} />
  ) : (
    <PageOutline notebook={notebook} page={page} active={!editing && active} />
  );
});

/**
 * PagePanel is the page's right column (M6/P7 design 7): its outline, the
 * view's while it is read, the editor's while it is edited (M13/P4/S3,
 * decision 876; ColumnOutline), its backlinks and its properties, each leading
 * to a page by href, and its versions (M8 design 4.13), each leading to
 * its version's page. From xl up it is
 * beside the page's content, the window's height, held at the top as the
 * window scrolls, scrolling itself, a line on its left, its negative
 * margins taking back the main's padding (M13/P4/S1, decision 860: the
 * mockup's .panel); open as the head's toggle has it (decision 859):
 * folded away, it stays mounted, hidden, its reads and events as they
 * were. Narrower, after the content, as wide as it is, centred: open or
 * not, the toggle not shown. Never the window's scroll anchor: held at the
 * top, it would not move as what is above the reader in the page changes
 * size, nor have the window follow. Its outline marks the section read
 * only while it is open beside the page (decision 886). data-page-panel is
 * where the outline watches the reader (its id, React's, has colons).
 */
export function PagePanel({
  id,
  open,
  notebook,
  page,
  editing,
  outline,
  href,
}: {
  id: string;
  open: boolean;
  notebook: Notebook;
  page: TreeNode;
  editing: boolean;
  /** The edit's outline, its editor's headings. */
  outline: OutlineBox;
  href: (id: string) => string;
}) {
  const t = useT();
  const beside = useBeside();
  return (
    <aside
      id={id}
      aria-label={t("page.panel")}
      data-open={open}
      data-page-panel
      className="min-w-0 space-y-6 max-xl:mx-auto max-xl:mt-10 max-xl:max-w-180 xl:sticky xl:top-0 xl:-me-10 xl:-mt-3 xl:-mb-10 xl:h-svh xl:w-63 xl:shrink-0 xl:overflow-y-auto xl:border-l xl:border-border xl:pt-16 xl:pr-4 xl:pb-5 xl:pl-4.5 xl:[overflow-anchor:none] xl:data-[open=false]:hidden"
    >
      <ColumnOutline editing={editing} outline={outline} notebook={notebook} page={page.id} active={open && beside} />
      <PageBacklinks notebook={notebook} page={page.id} href={href} />
      <PageProperties notebook={notebook} page={page.id} href={href} />
      <PageVersions notebook={notebook} page={page.id} href={(revision) => `${href(page.id)}/versions/${revision}`} />
    </aside>
  );
}
