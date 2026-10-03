import { observer } from "mobx-react-lite";
import { useContext, useLayoutEffect, useRef } from "react";
import useSWR from "swr";

import { NotLoaded } from "../../app/not-loaded";
import { enhance, Enhancements } from "../../reading/enhancement";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { usePageTree } from "../../stores/context";
import { useWorkspace } from "../workspace/workspace-layout";

/**
 * ReadingView is the page's content as the server renders it (M4/P5 design
 * 3.8): HTML the server sanitized, which goes into the article as it is.
 * It is read by page, and read again as SWR does (a refocus, a retry):
 * another's write then shows. A 503 server_busy is read again after its
 * Retry-After.
 *
 * Once the HTML is in, the app's enhancements run on it in their order;
 * before the HTML is replaced, and when the view goes, they are undone in
 * the reverse order (reading/enhancement.ts).
 */
export const ReadingView = observer(function ReadingView({ notebook, page }: { notebook: Notebook; page: TreeNode }) {
  const { slug } = useWorkspace();
  const pages = usePageTree(notebook);
  const enhancements = useContext(Enhancements);
  const { data, error, mutate } = useSWR(["page-view", page.id], () => pages.view(page.id));
  const article = useRef<HTMLElement>(null);
  const html = data?.html;
  const revision = data?.revision;
  const { id: notebookId, role } = notebook;
  useLayoutEffect(() => {
    const container = article.current;
    if (container === null || html === undefined || revision === undefined) {
      return undefined;
    }
    container.innerHTML = html;
    return enhance(enhancements, container, {
      workspace: slug,
      notebook: notebookId,
      page: page.id,
      revision,
      role,
      reload: () => void mutate(),
    });
  }, [html, revision, enhancements, slug, notebookId, role, page.id, mutate]);
  if (data === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  return <article ref={article} className="nw-reading min-w-0" />;
});
