import { Plus } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useState } from "react";
import useSWR from "swr";

import { useArrivalFocus } from "../../app/arrival";
import { ColumnTop } from "../../app/column-top";
import { useDocumentTitle } from "../../app/document-title";
import { writesNotebook } from "../../app/effective-role";
import { NotLoaded } from "../../app/not-loaded";
import { listColumnClass, titleRowClass, titleTextClass } from "../../app/page-classes";
import { errorText } from "../../app/problem-messages";
import { pageTitleClass } from "../../app/settings-page";
import { EmptyState } from "../../components/empty-state";
import { PageGlyph } from "../../components/page-icon";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import { cn } from "../../lib/cn";
import { usePageTree } from "../../stores/context";
import { childrenOf } from "../../stores/page-tree";
import { AttachmentsSection } from "../page/attachments-section";
import { SubpageList } from "../page/subpage-list";
import { useWorkspace } from "../workspace/workspace-layout";
import { useNewPage } from "./new-page";
import { useNotebook } from "./notebook-layout";

/**
 * NotebookHomePage is a notebook's first page (M4/P5 design 3.5): its
 * pages at the root, as the left column has them, each with how many pages
 * are under it, New page for its editors and admins on the right of its
 * title, and the attachments at the root (M7/P4 design 3.5). Centred, at
 * most 720px wide, as a page's column, the upgrade's banner at its top
 * whatever it shows (M13/P5/S1, decisions 915, 916, 919); its settings are
 * the left column's cog away.
 */
export const NotebookHomePage = observer(function NotebookHomePage() {
  const workspace = useWorkspace();
  const { slug } = workspace;
  const notebook = useNotebook();
  const pages = usePageTree(notebook);
  const t = useT();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  useDocumentTitle(notebook.name, workspace.name);
  const newPage = useNewPage(notebook);
  const [failure, setFailure] = useState<unknown>();
  const { error, mutate } = useSWR(["pages", notebook.id], () => pages.load());
  const home = `/${slug}/notebooks/${notebook.id}`;
  // Read once in the render: each row's count is of the same tree, a computed this observer keeps (decision 919).
  const tree = pages.tree;
  const roots = tree === undefined ? [] : childrenOf(tree, null);
  return (
    <section className={listColumnClass}>
      <ColumnTop />
      <div className={titleRowClass}>
        <h1 ref={heading} tabIndex={-1} className={cn(pageTitleClass, titleTextClass)}>
          {notebook.name}
        </h1>
        {writesNotebook(notebook) && (
          <Button
            size="sm"
            shape="pill"
            className="ms-auto"
            disabled={newPage.sending}
            onClick={() => {
              setFailure(undefined);
              void newPage.create(null).then(setFailure);
            }}
          >
            <Plus />
            {t("page.new")}
          </Button>
        )}
      </div>
      {failure !== undefined && <Alert>{errorText(failure, t)}</Alert>}
      {tree === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : roots.length === 0 ? (
        // New page is in the title's row: the empty state offers none of its own.
        <EmptyState icon={PageGlyph}>{t("notebook.homeEmpty")}</EmptyState>
      ) : (
        <SubpageList
          label={t("notebook.pages")}
          pages={roots}
          href={(id) => `${home}/pages/${id}`}
          countOf={(page) => childrenOf(tree, page.id).length}
        />
      )}
      <AttachmentsSection notebook={notebook} parent={null} />
    </section>
  );
});
