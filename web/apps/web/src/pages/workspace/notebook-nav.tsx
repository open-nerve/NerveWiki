import { Plus } from "lucide-react";
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
 * refused, so a guest is not offered it.
 */
export const NotebookNav = observer(function NotebookNav({ workspace }: { workspace: Workspace }) {
  const t = useT();
  const here = useMounted();
  return (
    <div className="space-y-4">
      <NotebookGroups
        workspace={workspace}
        headingClassName="px-3 text-xs font-medium text-muted-foreground"
        listClassName="flex flex-col gap-1"
        renderItem={(notebook) => (
          <NavItem to={`/${workspace.slug}/notebooks/${notebook.id}`}>
            <span className="block truncate">{notebook.name}</span>
          </NavItem>
        )}
        empty={<p className="px-3 text-sm text-muted-foreground">{t("notebooks.empty")}</p>}
      />
      {createsNotebooks(workspace) && (
        <CreateNotebookDialog
          workspace={workspace}
          here={here}
          trigger={
            <Button variant="ghost" className="w-full justify-start px-3">
              <Plus />
              {t("notebooks.create")}
            </Button>
          }
        />
      )}
    </div>
  );
});
