import { Book, ChevronRight, Plus } from "lucide-react";
import { observer } from "mobx-react-lite";
import { Link, useMatch } from "react-router";

import { arrived } from "../../app/arrival";
import { useMounted } from "../../app/mounted";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { Workspace } from "../../services/workspace.service";
import { useNotebooks, useStore } from "../../stores/context";
import { createsNotebooks } from "../../stores/notebook.store";
import { CreateNotebookDialog } from "./create-notebook-dialog";
import { NotebookGroups } from "./notebook-groups";
import { SlotHolder } from "./notebook-slot";

/**
 * NotebookNav is the notebooks' part of a workspace's navigation (M3/P4
 * design 3.3): the two groups, and New notebook for the workspace's admins
 * and members; a guest's creation is refused, so a guest is not offered it.
 * Rows of the left column, its groups' headings quiet (M13/P2/S1). The
 * notebook of the address is open in its item (M13/P2/S2, decision 776):
 * its row, page tree and places, which its shell draws into the container
 * the item holds (SlotHolder); every other is a link to its home, arrived
 * at, its item turning into the open notebook's as it is followed
 * (decision 777). The notebook of the address is as the address writes it,
 * as the notebook's shell finds it: one in upper case is no notebook of
 * the list. One the list no longer has, kept while a page of it is edited
 * unsaved as its shell keeps it, is open after the groups.
 */
export const NotebookNav = observer(function NotebookNav({ workspace }: { workspace: Workspace }) {
  const t = useT();
  const here = useMounted();
  const store = useStore();
  const notebooks = useNotebooks(workspace);
  const current = useMatch("/:slug/notebooks/:id/*")?.params.id;
  return (
    <div className="flex flex-col gap-0.5">
      <NotebookGroups
        workspace={workspace}
        headingClassName="px-2 pt-3.5 pb-1 text-xs leading-4.5 font-normal text-faint"
        listClassName="flex flex-col gap-0.5"
        groupClassName="flex flex-col gap-0.5"
        renderItem={(notebook) =>
          notebook.id === current ? (
            <SlotHolder id={notebook.id} />
          ) : (
            <NotebookLink workspace={workspace} notebook={notebook} />
          )
        }
        empty={<p className="px-2 py-1 text-row text-faint">{t("notebooks.empty")}</p>}
      />
      {current !== undefined &&
        notebooks.list !== undefined &&
        notebooks.byId(current) === undefined &&
        store.unsavedEdit({ notebookId: current }) && <SlotHolder id={current} />}
      {createsNotebooks(workspace) && (
        <CreateNotebookDialog
          workspace={workspace}
          here={here}
          trigger={
            <Button
              variant="ghost"
              className="h-7.5 w-full justify-start gap-2.25 rounded-row px-2 text-row font-normal"
            >
              <Plus />
              {t("notebooks.create")}
            </Button>
          }
        />
      )}
    </div>
  );
});

/**
 * NotebookLink is a notebook's row in the list while another is open: its home's link, arrived at (decision 777),
 * the link going as its item turns into the open notebook's. Its ⌄, book and name where the open notebook's row has
 * them (decision 778); its text its name alone, which the stories read.
 */
function NotebookLink({ workspace, notebook }: { workspace: Workspace; notebook: Notebook }) {
  return (
    <Link
      to={`/${workspace.slug}/notebooks/${notebook.id}`}
      state={arrived}
      className="flex h-7.5 items-center gap-2.25 rounded-row px-2 text-row text-muted-foreground transition-colors duration-150 hover:bg-hover hover:text-foreground"
    >
      <ChevronRight aria-hidden className="-mr-1 -ml-0.5 size-3.5 shrink-0 text-faint" />
      <Book aria-hidden className="size-4 shrink-0" />
      <span className="min-w-0 truncate">{notebook.name}</span>
    </Link>
  );
}
