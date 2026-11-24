import { observer } from "mobx-react-lite";
import { createContext, use, useLayoutEffect, useState } from "react";
import { createPortal } from "react-dom";
import { Navigate, Outlet, useMatches, useParams } from "react-router";
import useSWR from "swr";

import { arrived } from "../../app/arrival";
import { ColumnTopContext } from "../../app/column-top";
import { NotLoaded } from "../../app/not-loaded";
import type { Notebook } from "../../services/notebook.service";
import { useNotebooks, usePageTree, useStore } from "../../stores/context";
import { NotFoundPage } from "../not-found";
import { useNotebookSlot } from "../workspace/notebook-slot";
import { useWorkspace } from "../workspace/workspace-layout";
import { NotebookPlaces } from "./notebook-places";
import { PageTree } from "./page-tree";
import { QuickSwitch } from "./quick-switch";
import { UpgradeBanner } from "./upgrade-banner";

const NotebookContext = createContext<Notebook | undefined>(undefined);

/** What a route says of its page's column: column, that the upgrade's banner is at its top; settings, a settings page's. */
type ColumnHandle = { column?: boolean; settings?: boolean };

/**
 * useNotebook is the notebook of the page's address, as the workspace's
 * list holds it, renamed too, or as it was last found while it stays
 * shown gone: only below NotebookLayout, which shows its pages once it has
 * found it.
 */
export function useNotebook(): Notebook {
  const notebook = use(NotebookContext);
  if (notebook === undefined) {
    throw new Error("useNotebook is used outside NotebookLayout");
  }
  return notebook;
}

/**
 * NotebookLayout is the shell of a notebook's pages (M3/P4 design 3.4). It
 * finds the notebook of the address in the workspace's list, which is the
 * notebooks the account sees with its role in each: an id the list does
 * not have is no page of the app's, whether the notebook is gone or
 * hidden; one this tab has just deleted or left goes to the workspace's
 * home instead, arrived at. Its pages start anew with each notebook, and
 * so do its page tree, which it puts in the workspace's left column
 * (M4/P5 design 3.6) with its other places below it (notebook-places.tsx),
 * and its quick switch (3.10); above its pages, the banner of its upgrade
 * under way (M11/P1 design 4.4): at the top of the page's column of a page
 * and of the notebook's settings, which draw it there (ColumnTop; M13/P4/S1,
 * decision 857), as the render reads the route, above the others'. The tree and the places go into the
 * container the workspace's shell keeps for the notebook of the address,
 * which the list's item of the notebook holds (notebook-slot.tsx,
 * M13/P2/S2). A notebook gone while
 * this tab edits one of its pages with changes not saved stays, as it was
 * last found, until the edit ends (M5/P4 design 3.9).
 */
export const NotebookLayout = observer(function NotebookLayout() {
  const { id = "" } = useParams();
  const workspace = useWorkspace();
  const notebooks = useNotebooks(workspace);
  const store = useStore();
  // The id as the address writes it: the list's item of the same id holds the container (NotebookNav).
  const container = useNotebookSlot().containerFor(id);
  const { error, mutate } = useSWR(["notebooks", workspace.id], () => notebooks.load());
  const [last, setLast] = useState<Notebook | undefined>(undefined);
  const columned = useMatches().some((match) => {
    const handle = match.handle as ColumnHandle | undefined;
    return handle?.column === true || handle?.settings === true;
  });
  const found = notebooks.list === undefined ? undefined : notebooks.byId(id);
  if (found !== undefined && found !== last) {
    setLast(found);
  }
  if (notebooks.list === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  const notebook = found ?? (last?.id === id && store.unsavedEdit({ notebookId: id }) ? last : undefined);
  if (notebook === undefined) {
    return notebooks.wasRemoved(id) ? <Navigate replace to={`/${workspace.slug}`} state={arrived} /> : <NotFoundPage />;
  }
  return (
    <NotebookContext value={notebook}>
      <TreeWatch notebook={notebook} />
      {/* Keyed apart from the outlet's notebook.id, its sibling's: one key for both is React's error (review C-M2). */}
      {createPortal(
        <>
          <PageTree notebook={notebook} />
          <NotebookPlaces notebook={notebook} />
        </>,
        container,
        `tree-${notebook.id}`
      )}
      <QuickSwitch notebook={notebook} />
      {/* Outside the outlet: the upgrade's state changing remounts none of the page; a page's column draws it at its
          top, ColumnTop alone rendering again as it changes. The outlet at one place in the tree, columned or not. */}
      {!columned && <UpgradeBanner notebook={notebook} />}
      <ColumnTopContext value={columned ? <UpgradeBanner notebook={notebook} /> : null}>
        <Outlet key={notebook.id} />
      </ColumnTopContext>
    </NotebookContext>
  );
});

/**
 * TreeWatch has the notebook's tree watched while its shell shows (M5-polish 27.2): left, the events read it no
 * more, and a page shown again waits for a read since. Its layout effect runs before the pages' effects.
 */
function TreeWatch({ notebook }: { notebook: Notebook }) {
  const pages = usePageTree(notebook);
  useLayoutEffect(() => pages.watch(), [pages]);
  return null;
}
