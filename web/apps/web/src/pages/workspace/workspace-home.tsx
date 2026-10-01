import { observer } from "mobx-react-lite";
import { Link } from "react-router";

import { useArrivalFocus } from "../../app/arrival";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import { createsNotebooks } from "../../stores/notebook.store";
import { CreateNotebookDialog } from "./create-notebook-dialog";
import { NotebookGroups } from "./notebook-groups";
import { useWorkspace } from "./workspace-layout";

/**
 * WorkspaceHomePage is a workspace's first page (M3/P4 design 3.3): its
 * notebooks in their two groups, as cards; with none, New notebook for the
 * workspace's admins and members.
 */
export const WorkspaceHomePage = observer(function WorkspaceHomePage() {
  const workspace = useWorkspace();
  const t = useT();
  const heading = useArrivalFocus<HTMLHeadingElement>();
  return (
    <section className="max-w-4xl space-y-6">
      <h1 ref={heading} tabIndex={-1} className="text-2xl font-semibold outline-none">
        {workspace.name}
      </h1>
      <NotebookGroups
        workspace={workspace}
        headingClassName="text-lg font-medium"
        listClassName="grid gap-3 sm:grid-cols-2 lg:grid-cols-3"
        renderItem={(notebook) => (
          <Link
            to={`/${workspace.slug}/notebooks/${notebook.id}`}
            className="rounded-lg border p-4 hover:bg-accent focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none"
          >
            <span className="block truncate font-medium">{notebook.name}</span>
            <span className="block text-sm text-muted-foreground">{t(`access.${notebook.workspace_access}`)}</span>
          </Link>
        )}
        empty={
          <div className="space-y-3">
            <p className="text-muted-foreground">{t("notebooks.empty")}</p>
            {createsNotebooks(workspace) && (
              <CreateNotebookDialog workspace={workspace} trigger={<Button>{t("notebooks.create")}</Button>} />
            )}
          </div>
        }
      />
    </section>
  );
});
