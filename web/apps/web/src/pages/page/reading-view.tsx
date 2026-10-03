import { observer } from "mobx-react-lite";
import { useContext, useLayoutEffect, useRef } from "react";
import useSWR, { useSWRConfig } from "swr";

import { writesPages } from "../../app/effective-role";
import { NotLoaded } from "../../app/not-loaded";
import { enhance, Enhancements } from "../../reading/enhancement";
import { ApiError } from "../../services/api";
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
 * the reverse order (reading/enhancement.ts). A writer's tick task items
 * through it, and refused tells the page what was refused, undefined as a
 * toggle starts (M5/P6 design 3.5). A task item's checkbox that had the
 * focus as the HTML is replaced has it back in the new HTML: a tick does
 * not move it.
 */
export const ReadingView = observer(function ReadingView({
  notebook,
  page,
  refused,
}: {
  notebook: Notebook;
  page: TreeNode;
  refused: (error: unknown) => void;
}) {
  const { slug } = useWorkspace();
  const pages = usePageTree(notebook);
  const enhancements = useContext(Enhancements);
  const { mutate: revalidate } = useSWRConfig();
  const { data, error, mutate } = useSWR(["page-view", notebook.id, page.id], () => pages.view(page.id));
  const article = useRef<HTMLElement>(null);
  // The task item focused as the HTML was replaced, by its data-task.
  const focusedTask = useRef<string | undefined>(undefined);
  const html = data?.html;
  const revision = data?.revision;
  const { id: notebookId, role } = notebook;
  useLayoutEffect(() => {
    const container = article.current;
    if (container === null || html === undefined || revision === undefined) {
      return undefined;
    }
    container.innerHTML = html;
    const undo = enhance(enhancements, container, {
      workspace: slug,
      notebook: notebookId,
      page: page.id,
      revision,
      role,
      reload: () => void mutate(),
      toggleTask: writesPages(role)
        ? async (offset, checked) => {
            refused(undefined);
            try {
              await pages.toggleTask(page.id, { base_revision: revision, offset, checked });
            } catch (failure) {
              const code = failure instanceof ApiError ? failure.code : undefined;
              if (code === "page.revision_mismatch") {
                await mutate();
              } else if (code === "page.locked") {
                await revalidate(["edit-lock", page.id]);
              }
              throw failure;
            }
            await mutate();
          }
        : undefined,
      report: refused,
    });
    const task = focusedTask.current;
    focusedTask.current = undefined;
    if (task !== undefined) {
      container.querySelector<HTMLElement>(`[data-task="${CSS.escape(task)}"]`)?.focus();
    }
    return () => {
      // Undone, a checkbox is disabled, which takes its focus: which had it is read first.
      const focused = document.activeElement;
      focusedTask.current =
        focused instanceof HTMLElement && container.contains(focused) ? focused.dataset.task : undefined;
      undo();
    };
  }, [html, revision, enhancements, slug, notebookId, role, page.id, mutate, pages, revalidate, refused]);
  if (data === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  // Named by the page: it can get the focus to scroll a wide content (reading/scroll-focus.ts).
  return <article ref={article} aria-label={page.name} className="nw-reading min-w-0" />;
});
