import { reaction } from "mobx";
import { observer } from "mobx-react-lite";
import { useContext, useEffect, useLayoutEffect, useRef, useState } from "react";
import { useNavigate } from "react-router";
import useSWR, { useSWRConfig } from "swr";

import { arrived } from "../../app/arrival";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { useT } from "../../i18n/i18n";
import { enhance, Enhancements } from "../../reading/enhancement";
import type { Notebook } from "../../services/notebook.service";
import { useAssets, usePageTree, useStore } from "../../stores/context";
import { useWorkspace } from "../workspace/workspace-layout";
import { eachRead, stamped, useAssetsExpiry } from "./assets-expiry";
import { useUnresolvedLinks } from "./unresolved-link";

/**
 * VersionView is the reading view of the version of the page that holds
 * revision (M8 design 4.3), by its key, named label: the server's HTML of
 * its content, its links leading where they lead now, which the reading
 * enhancements run on as on the page's (reading-view.tsx), read only: no
 * task item is ticked, and a link to a page that is not there says so,
 * creating none. Its attachments' addresses expire as the page's do: it is
 * read again before they do (page-view.ts). What an enhancement reports
 * shows above it, until the view is read anew. A version's page shows it,
 * and the trash's preview a page's in the trash, at its revision (M8/P2
 * design 4.3): the page need not be in the tree.
 */
export const VersionView = observer(function VersionView({
  notebook,
  page,
  revision,
  label,
}: {
  notebook: Notebook;
  /** The page's id. */
  page: string;
  revision: number;
  /** The view's name, as its article has it. */
  label: string;
}) {
  const { slug } = useWorkspace();
  const pages = usePageTree(notebook);
  const assets = useAssets(notebook);
  const enhancements = useContext(Enhancements);
  const t = useT();
  const { preferences } = useStore();
  const navigate = useNavigate();
  const [failure, setFailure] = useState<unknown>(undefined);
  const { data, error, mutate } = useSWR(
    ["page-version-view", notebook.id, page, revision],
    () => stamped(pages.versionView(page, revision)),
    eachRead
  );
  const view = useAssetsExpiry(data, () => void mutate());
  const html = view?.html;
  // What an enhancement reported of a view goes with it.
  const [reported, setReported] = useState(html);
  if (reported !== html) {
    setReported(html);
    setFailure(undefined);
  }
  const article = useRef<HTMLElement>(null);
  const links = useUnresolvedLinks({
    notebook,
    page,
    pages,
    article,
    reload: () => void mutate(),
    report: setFailure,
    readOnly: true,
  });
  const latestUnresolved = useRef(links.unresolved);
  useEffect(() => {
    latestUnresolved.current = links.unresolved;
  });
  const at = view?.revision;
  const assetsExpire = view?.assets_expire_at ?? null;
  const { id: notebookId, role } = notebook;
  useLayoutEffect(() => {
    const container = article.current;
    if (container === null || html === undefined || at === undefined) {
      return undefined;
    }
    container.innerHTML = html;
    return enhance(enhancements, container, {
      workspace: slug,
      notebook: notebookId,
      page,
      revision: at,
      role,
      t,
      locale: preferences.locale,
      theme: () => preferences.resolvedTheme,
      onThemeChange: (listener) => reaction(() => preferences.resolvedTheme, listener),
      reload: () => void mutate(),
      navigate: (to) => void navigate(to, to.includes("#") ? undefined : { state: arrived }),
      report: setFailure,
      unresolved: (link) => void latestUnresolved.current(link),
      assetsExpire,
      assetAddress: (id) => assets.address(id),
    });
  }, [html, at, assetsExpire, enhancements, slug, notebookId, role, t, preferences, page, mutate, navigate, assets]);
  if (view === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  return (
    <div className="space-y-4">
      {failure !== undefined && <Alert>{errorText(failure, t)}</Alert>}
      <article ref={article} aria-label={label} className="nw-reading min-w-0" />
      {links.dialog}
    </div>
  );
});

/**
 * useLeftBehind takes what was read of the page's versions out of SWR's
 * cache once its version's page, or its preview in the trash, is left (M8
 * design 4.13; M8/P2 design 4.3): its versions' contents and views, and the
 * page's content now, each up to 5 MiB, would stay in memory for each
 * version or item visited, and a view would be read again with each change
 * of the tree. They go, to be read as they show again, deduplication or
 * not. Once the turn is over: StrictMode's unmount, mounted again at once,
 * takes nothing out. A cache gone by then, with its generation, holds
 * nothing.
 */
export function useLeftBehind(notebook: string, page: string): void {
  const { mutate } = useSWRConfig();
  const leaving = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => {
    clearTimeout(leaving.current);
    return () => {
      leaving.current = setTimeout(() => {
        mutate(
          (key) =>
            Array.isArray(key) &&
            (key[0] === "page-version" || key[0] === "page-version-view" || key[0] === "page-content") &&
            key[1] === notebook &&
            key[2] === page,
          undefined
        ).catch(() => undefined);
      }, 0);
    };
  }, [mutate, notebook, page]);
}
