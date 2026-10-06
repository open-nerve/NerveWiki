import { observer } from "mobx-react-lite";
import { useParams } from "react-router";
import useSWR from "swr";

import { useArrivalFocus } from "../../app/arrival";
import { useDocumentTitle } from "../../app/document-title";
import { NotLoaded } from "../../app/not-loaded";
import { useT } from "../../i18n/i18n";
import { usePageTree } from "../../stores/context";
import { inTreeOrder } from "../../stores/page-tree";
import { SubpageList } from "../page/subpage-list";
import { useWorkspace } from "../workspace/workspace-layout";
import { distinctName } from "./distinct-name";
import { useNotebook } from "./notebook-layout";

/**
 * TagPage is the pages of a notebook's tag (M6/P6 design 8): the pages
 * that have the tag or a tag under it (#a/b under #a), in the tree's
 * order, each named as the tree names it. The tag is the address's one
 * segment, its '/' written %2F, as a reading view's tag links to it. The
 * tag's pages are read by tag, and named by the tree: a page it does not
 * have, deleted since, is left out. Neither is paged.
 */
export const TagPage = observer(function TagPage() {
  const { slug } = useWorkspace();
  const notebook = useNotebook();
  const pages = usePageTree(notebook);
  const { tag = "" } = useParams();
  const t = useT();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  useDocumentTitle(`#${tag}`, notebook.name);
  const tree = useSWR(["pages", notebook.id], () => pages.load());
  const tagged = useSWR(["tag-pages", notebook.id, tag], () => pages.tagPages(tag));
  const index = pages.tree;
  const listed =
    index === undefined || tagged.data === undefined ? undefined : inTreeOrder(index, new Set(tagged.data));
  const home = `/${slug}/notebooks/${notebook.id}`;
  return (
    <section className="space-y-4">
      <h1 ref={heading} tabIndex={-1} className="text-2xl font-semibold break-words outline-none">
        #{tag}
      </h1>
      {listed === undefined ? (
        <NotLoaded
          error={tagged.error ?? tree.error}
          retry={() => {
            void tagged.mutate();
            void tree.mutate();
          }}
        />
      ) : listed.length === 0 ? (
        <p className="text-muted-foreground">{t("tag.empty")}</p>
      ) : (
        <SubpageList
          label={t("tag.pages", { tag })}
          pages={listed}
          href={(id) => `${home}/pages/${id}`}
          nameOf={(page) => distinctName(index, notebook, page.id, t) ?? page.name}
        />
      )}
    </section>
  );
});
