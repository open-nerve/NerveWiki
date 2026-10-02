import { observer } from "mobx-react-lite";
import { useLayoutEffect, useRef } from "react";
import useSWR from "swr";

import { NotLoaded } from "../../app/not-loaded";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { usePageTree } from "../../stores/context";

/**
 * ReadingView is the page's content as the server renders it (M4/P5 design
 * 3.8): HTML the server sanitized, which goes into the article as it is.
 * It is read by page, and read again as SWR does (a refocus, a retry):
 * another's write then shows. A 503 server_busy is read again after its
 * Retry-After.
 */
export const ReadingView = observer(function ReadingView({ notebook, page }: { notebook: Notebook; page: TreeNode }) {
  const pages = usePageTree(notebook);
  const { data, error, mutate } = useSWR(["page-view", page.id], () => pages.view(page.id));
  const article = useRef<HTMLElement>(null);
  const html = data?.html;
  useLayoutEffect(() => {
    if (article.current !== null && html !== undefined) {
      article.current.innerHTML = html;
    }
  }, [html]);
  if (data === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  return <article ref={article} className="nw-reading min-w-0" />;
});
