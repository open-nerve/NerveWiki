import { observer } from "mobx-react-lite";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { RenameForm } from "../../app/rename-form";
import { workspaceNameProblem } from "../../app/slug";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { Workspace } from "../../services/workspace.service";
import { useWorkspaces } from "../../stores/context";
import { useWorkspace } from "./workspace-layout";

/**
 * GeneralPage is a workspace's name and address (M2/P5 design 3.6): its
 * admins rename and delete it; the others see them.
 */
export const GeneralPage = observer(function GeneralPage() {
  const workspace = useWorkspace();
  const workspaces = useWorkspaces();
  const t = useT();
  const admin = workspace.role === "admin";
  return (
    <div className="space-y-10">
      <section className="max-w-md space-y-6">
        <h2 className="text-lg font-semibold">{t("workspaceSettings.general")}</h2>
        {admin ? (
          <RenameForm
            current={workspace.name}
            label={t("createWorkspace.name")}
            check={workspaceNameProblem}
            rename={(name) => workspaces.rename(workspace.slug, name)}
            saveLabel={t("workspaceSettings.save")}
            savedLabel={t("workspaceSettings.saved")}
          />
        ) : (
          <div className="space-y-1">
            <h3 className="text-sm font-medium">{t("createWorkspace.name")}</h3>
            <p>{workspace.name}</p>
            <p className="text-sm text-muted-foreground">{t("workspaceSettings.nameAdminsOnly")}</p>
          </div>
        )}
        <div className="space-y-1">
          <h3 className="text-sm font-medium">{t("createWorkspace.slug")}</h3>
          <p>{workspace.slug}</p>
          <p className="text-sm text-muted-foreground">{t("workspaceSettings.slugFixed")}</p>
        </div>
      </section>
      {admin && <DeleteSection workspace={workspace} />}
    </div>
  );
});

/**
 * DeleteSection deletes the workspace for every member, once its slug is
 * typed; the shell then goes to /, which lands on another workspace or on
 * the creation page.
 */
function DeleteSection({ workspace }: { workspace: Workspace }) {
  const workspaces = useWorkspaces();
  const t = useT();
  return (
    <section className="max-w-md space-y-3">
      <h2 className="text-lg font-semibold">{t("workspaceSettings.deleteTitle")}</h2>
      <p className="text-sm text-muted-foreground">{t("workspaceSettings.deleteBody")}</p>
      <ConfirmDialog
        trigger={<Button variant="destructive">{t("workspaceSettings.delete")}</Button>}
        title={t("workspaceSettings.deleteConfirmTitle", { name: workspace.name })}
        description={t("workspaceSettings.deleteBody")}
        typedConfirmation={{ label: t("workspaceSettings.typeSlug", { slug: workspace.slug }), value: workspace.slug }}
        confirmLabel={t("workspaceSettings.deleteConfirm")}
        sendingLabel={t("workspaceSettings.deleting")}
        cancelLabel={t("workspaceSettings.cancel")}
        confirm={() => workspaces.remove(workspace.slug)}
      />
    </section>
  );
}
