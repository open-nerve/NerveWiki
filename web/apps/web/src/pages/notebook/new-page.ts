import { useState } from "react";
import { useNavigate } from "react-router";

import { arrived } from "../../app/arrival";
import { errorText } from "../../app/problem-messages";
import { useT } from "../../i18n/i18n";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import { usePageTree } from "../../stores/context";
import { freeTitle } from "../../stores/page-tree";
import { useWorkspace } from "../workspace/workspace-layout";

/** How many titles a creation tries before it gives up (M4 design 4). */
const attempts = 3;

/**
 * useNewPage creates pages in notebook (M4/P5 design 3.7): each is titled
 * its siblings' first free Untitled, Untitled 2, …; a title taken in the
 * meantime (409 page.title_taken: another tab, or a title the client
 * compares otherwise than the server) has it try the next, three titles at
 * most. Once created, the tab goes to the page, arrived at, unless the
 * place it was asked from is gone (here). A failure is the caller's to
 * show, until the next creation.
 */
export function useNewPage(notebook: Notebook, here: () => boolean) {
  const { slug } = useWorkspace();
  const pages = usePageTree(notebook);
  const navigate = useNavigate();
  const t = useT();
  const [sending, setSending] = useState(false);
  const [failure, setFailure] = useState<unknown>();

  /** titled creates the page under parent by the first title free beside taken, and the next while each is taken. */
  async function titled(parent: string | null, taken: readonly string[]): Promise<string> {
    const title = freeTitle(pages.childrenOf(parent), taken, (n) =>
      n === 1 ? t("page.untitled") : t("page.untitledN", { n })
    );
    try {
      return await pages.create(parent, title);
    } catch (error) {
      if (taken.length + 1 >= attempts || !(error instanceof ApiError && error.code === "page.title_taken")) {
        throw error;
      }
      return titled(parent, [...taken, title]);
    }
  }

  async function create(parent: string | null): Promise<void> {
    setSending(true);
    setFailure(undefined);
    try {
      const id = await titled(parent, []);
      if (here()) {
        void navigate(`/${slug}/notebooks/${notebook.id}/pages/${id}`, { state: arrived });
      }
    } catch (error) {
      setFailure(error);
    } finally {
      setSending(false);
    }
  }

  return { create, sending, failed: failure === undefined ? undefined : errorText(failure, t) };
}
