import { observer } from "mobx-react-lite";
import { Link } from "react-router";
import useSWR from "swr";

import { useArrivalFocus } from "../../app/arrival";
import { useFollowRole } from "../../app/follow-role";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { Workspace } from "../../services/workspace.service";
import { useOwnerless } from "../../stores/context";
import { createsNotebooks } from "../../stores/notebook.store";
import { CreateNotebookDialog } from "./create-notebook-dialog";
import { NotebookGroups } from "./notebook-groups";
import { useWorkspace } from "./workspace-layout";

/**
 * WorkspaceHomePage is a workspace's first page (M3/P4 design 3.3): its
 * notebooks in their two groups, as cards; with none, New notebook for the
 * workspace's admins and members. Its admins read there how many
 * notebooks have no admin, if any (M3/P5 design 3.3).
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
      {workspace.role === "admin" && <OwnerlessReminder workspace={workspace} />}
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

/**
 * OwnerlessReminder says how many of the workspace's notebooks have no
 * admin, with a way to them; nothing when none has, or while they are not
 * read: it is no part of the home's own content. It reads them as the
 * ownerless page does, once for both.
 */
const OwnerlessReminder = observer(function OwnerlessReminder({ workspace }: { workspace: Workspace }) {
  const ownerless = useOwnerless(workspace);
  const t = useT();
  useSWR(["ownerless", workspace.id], () => ownerless.load(), { onError: useFollowRole() });
  const count = ownerless.list?.length ?? 0;
  if (count === 0) {
    return null;
  }
  return (
    <p className="text-sm">
      {t("ownerless.reminder", { count })}{" "}
      <Link to={`/${workspace.slug}/settings/ownerless`} className="underline underline-offset-4">
        {t("ownerless.review")}
      </Link>
    </p>
  );
});
