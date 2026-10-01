import { observer } from "mobx-react-lite";
import { Navigate, Outlet, useParams } from "react-router";
import useSWR from "swr";

import { arrived } from "../../app/arrival";
import { NotLoaded } from "../../app/not-loaded";
import type { Notebook } from "../../services/notebook.service";
import { useNotebooks } from "../../stores/context";
import { NotFoundPage } from "../not-found";
import { useWorkspace } from "../workspace/workspace-layout";

/**
 * useNotebook is the notebook of the page's address, as the workspace's
 * list holds it, renamed too: only below NotebookLayout, which shows its
 * pages once it has found it.
 */
export function useNotebook(): Notebook {
  const { id = "" } = useParams();
  const notebook = useNotebooks(useWorkspace()).byId(id);
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
 * home instead, arrived at. Its pages start anew with each notebook.
 */
export const NotebookLayout = observer(function NotebookLayout() {
  const { id = "" } = useParams();
  const workspace = useWorkspace();
  const notebooks = useNotebooks(workspace);
  const { error, mutate } = useSWR(["notebooks", workspace.id], () => notebooks.load());
  if (notebooks.list === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  const notebook = notebooks.byId(id);
  if (notebook === undefined) {
    return notebooks.wasRemoved(id) ? <Navigate replace to={`/${workspace.slug}`} state={arrived} /> : <NotFoundPage />;
  }
  return <Outlet key={notebook.id} />;
});
