import { Book, Plus } from "lucide-react";
import { observer } from "mobx-react-lite";

import { useMounted } from "../../app/mounted";
import { NavItem } from "../../components/nav-item";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { Workspace } from "../../services/workspace.service";
import { createsNotebooks } from "../../stores/notebook.store";
import { CreateNotebookDialog } from "./create-notebook-dialog";
import { NotebookGroups } from "./notebook-groups";

/**
 * NotebookNav is the notebooks' part of a workspace's navigation (M3/P4
 * design 3.3): the two groups, each notebook a link to its home, and New
 * notebook for the workspace's admins and members; a guest's creation is
 * refused, so a guest is not offered it. Rows of the left column, its
 * groups' headings quiet (M13/P2/S1).
 */
export const NotebookNav = observer(function NotebookNav({ workspace }: { workspace: Workspace }) {
  const t = useT();
  const here = useMounted();
  return (
    <div className="flex flex-col gap-0.5">
      <NotebookGroups
        workspace={workspace}
        headingClassName="px-2 pt-3.5 pb-1 text-xs font-normal text-faint"
        listClassName="flex flex-col gap-0.5"
        groupClassName=""
        renderItem={(notebook) => (
          <NavItem to={`/${workspace.slug}/notebooks/${notebook.id}`}>
            <Book aria-hidden />
            <span className="min-w-0 truncate">{notebook.name}</span>
          </NavItem>
        )}
        empty={<p className="px-2 py-1 text-row text-faint">{t("notebooks.empty")}</p>}
      />
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
