import { observer } from "mobx-react-lite";
import { useParams } from "react-router";
import useSWR from "swr";

import { useArrivalFocus } from "../../app/arrival";
import { NotLoaded } from "../../app/not-loaded";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { usePageTree } from "../../stores/context";
import { useNotebook } from "../notebook/notebook-layout";
import { NotFoundPage } from "../not-found";
import { useWorkspace } from "../workspace/workspace-layout";
import { Breadcrumbs } from "./breadcrumbs";
import { ReadingView } from "./reading-view";
import { SubpageList } from "./subpage-list";

/**
 * PageLayout is a page's shell (M4/P5 design 3.5). It finds the page of the
 * address in the notebook's tree, as the left column shows it: until the
 * tree is read it shows nothing of the page; a page the tree does not
 * have is no page of the app's. The page's title, ancestors and children
 * come from the tree too, so that the tree read again refreshes them.
 */
export const PageLayout = observer(function PageLayout() {
  const notebook = useNotebook();
  const pages = usePageTree(notebook);
  const { pageId = "" } = useParams();
  const { error, mutate } = useSWR(["pages", notebook.id], () => pages.load());
  if (pages.nodes === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  const page = pages.byId(pageId);
  if (page === undefined) {
    return <NotFoundPage />;
  }
  return <PageShell key={page.id} notebook={notebook} page={page} />;
});

/** PageShell is the page found: where it is, its title, its reading view and its children. */
const PageShell = observer(function PageShell({ notebook, page }: { notebook: Notebook; page: TreeNode }) {
  const { slug } = useWorkspace();
  const pages = usePageTree(notebook);
  const t = useT();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  const home = `/${slug}/notebooks/${notebook.id}`;
  const href = (id?: string) => (id === undefined ? home : `${home}/pages/${id}`);
  const children = pages.childrenOf(page.id);
  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <Breadcrumbs notebook={notebook} ancestors={pages.ancestorsOf(page.id)} page={page} href={href} />
        <h1 ref={heading} tabIndex={-1} className="text-3xl font-semibold break-words outline-none">
          {page.name}
        </h1>
      </div>
      <ReadingView notebook={notebook} page={page} />
      {children.length > 0 && (
        <section className="space-y-2">
          <h2 className="text-sm font-medium text-muted-foreground">{t("page.subpages")}</h2>
          <SubpageList label={t("page.subpages")} pages={children} href={href} />
        </section>
      )}
    </div>
  );
});
