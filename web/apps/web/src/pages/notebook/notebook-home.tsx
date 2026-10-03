import { Plus } from "lucide-react";
import { observer } from "mobx-react-lite";
import { Link } from "react-router";
import useSWR from "swr";

import { useArrivalFocus } from "../../app/arrival";
import { useMounted } from "../../app/mounted";
import { NotLoaded } from "../../app/not-loaded";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import { usePageTree } from "../../stores/context";
import { SubpageList } from "../page/subpage-list";
import { useWorkspace } from "../workspace/workspace-layout";
import { useNewPage } from "./new-page";
import { useNotebook } from "./notebook-layout";
import { writes } from "./page-tree";

/**
 * NotebookHomePage is a notebook's first page (M4/P5 design 3.5): its
 * pages at the root, as the left column has them, New page for its
 * editors and admins, and a way to its settings.
 */
export const NotebookHomePage = observer(function NotebookHomePage() {
  const { slug } = useWorkspace();
  const notebook = useNotebook();
  const pages = usePageTree(notebook);
  const t = useT();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  const newPage = useNewPage(notebook, useMounted());
  const { error, mutate } = useSWR(["pages", notebook.id], () => pages.load());
  const home = `/${slug}/notebooks/${notebook.id}`;
  const roots = pages.childrenOf(null);
  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 ref={heading} tabIndex={-1} className="text-2xl font-semibold outline-none">
          {notebook.name}
        </h1>
        {writes(notebook) && (
          <Button disabled={newPage.sending} onClick={() => void newPage.create(null)}>
            <Plus />
            {t("page.new")}
          </Button>
        )}
      </div>
      {newPage.failed !== undefined && <Alert>{newPage.failed}</Alert>}
      {pages.nodes === undefined ? (
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
