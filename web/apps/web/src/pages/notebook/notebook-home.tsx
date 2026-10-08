import { Plus } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useState } from "react";
import { Link } from "react-router";
import useSWR from "swr";

import { useArrivalFocus } from "../../app/arrival";
import { useDocumentTitle } from "../../app/document-title";
import { writesPages } from "../../app/effective-role";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import { usePageTree } from "../../stores/context";
import { SubpageList } from "../page/subpage-list";
import { useWorkspace } from "../workspace/workspace-layout";
import { useNewPage } from "./new-page";
import { useNotebook } from "./notebook-layout";

/**
 * NotebookHomePage is a notebook's first page (M4/P5 design 3.5): its
 * pages at the root, as the left column has them, New page for its
 * editors and admins, and a way to its settings.
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
  const roots = pages.childrenOf(null);
  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 ref={heading} tabIndex={-1} className="text-2xl font-semibold outline-none">
          {notebook.name}
        </h1>
        {writesPages(notebook.role) && (
          <Button
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
      {pages.tree === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : roots.length === 0 ? (
        <p className="text-muted-foreground">{t("notebook.homeEmpty")}</p>
      ) : (
        <SubpageList label={t("notebook.pages")} pages={roots} href={(id) => `${home}/pages/${id}`} />
      )}
      <Link to={`${home}/settings`} className="text-sm underline underline-offset-4">
        {t("notebookSettings.title")}
      </Link>
    </section>
  );
});
