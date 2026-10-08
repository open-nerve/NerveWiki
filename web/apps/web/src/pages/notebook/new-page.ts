import { useEffect, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router";

import { arrived } from "../../app/arrival";
import { useMounted } from "../../app/mounted";
import { useT } from "../../i18n/i18n";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import { usePageTree } from "../../stores/context";
import { freeTitle } from "../../stores/page-tree";
import { useWorkspace } from "../workspace/workspace-layout";

/** How many titles a creation tries before it gives up (M4 design 4): the first free one and the next two. */
const attempts = 3;

/**
 * useNewPage creates pages in notebook (M4/P5 design 3.7): each is titled
 * its siblings' first free Untitled, Untitled 2, …, the attachments beside
 * it among them (M7/P2 design 3.10); a title taken in the
 * meantime (409 page.title_taken: another tab, or a title the client
 * compares otherwise than the server) has it try the next, three titles at
 * most. Once created, the tab goes to the page, arrived at, unless the user
 * left the place it was asked from: the component that asked is gone, or
 * the address changed (v0.1 design 13.2, item 16). create answers why it
 * failed, for the caller to show, or undefined.
 */
export function useNewPage(notebook: Notebook) {
  const { slug } = useWorkspace();
  const pages = usePageTree(notebook);
  const navigate = useNavigate();
  const { key } = useLocation();
  const mounted = useMounted();
  const t = useT();
  const [sending, setSending] = useState(false);
  /** The address as of the last render. */
  const at = useRef(key);
  useEffect(() => {
    at.current = key;
  }, [key]);

  /** titled creates the page under parent by the first title free beside taken, and the next while each is taken. */
  async function titled(parent: string | null, taken: readonly string[]): Promise<string> {
    const title = freeTitle(pages.siblingsOf(parent), taken, (n) =>
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

  async function create(parent: string | null): Promise<unknown> {
    const from = at.current;
    setSending(true);
    try {
      const id = await titled(parent, []);
      if (mounted() && at.current === from) {
        void navigate(`/${slug}/notebooks/${notebook.id}/pages/${id}`, { state: arrived });
      }
      return undefined;
    } catch (error) {
      return error;
    } finally {
      setSending(false);
    }
  }

  return { create, sending };
}
