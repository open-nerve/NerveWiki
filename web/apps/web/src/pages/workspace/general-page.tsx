import { observer } from "mobx-react-lite";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { NameRow } from "../../app/name-row";
import { SettingsCard, SettingsRow, SettingsSection } from "../../app/settings-page";
import { workspaceNameProblem } from "../../app/slug";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { Workspace } from "../../services/workspace.service";
import { useWorkspaces } from "../../stores/context";
import { useWorkspace } from "./workspace-layout";

/**
 * GeneralPage is a workspace's name and address (M2/P5 design 3.6), the basics' card (M13/P3/S2, decisions 820, 844):
 * its admins rename it as the name is changed (NameRow) and delete it, the danger zone's (decision 826); the others
 * see the name, and everyone the address, each a value its row shows.
 */
export const GeneralPage = observer(function GeneralPage() {
  const workspace = useWorkspace();
  const workspaces = useWorkspaces();
  const t = useT();
  const admin = workspace.role === "admin";
  return (
    <>
      <SettingsSection title={t("settings.basics")}>
        <SettingsCard>
          {admin ? (
            <NameRow
              current={workspace.name}
              field="name"
              label={t("createWorkspace.name")}
              check={workspaceNameProblem}
              save={(name) => workspaces.rename(workspace.slug, name)}
              saveLabel={t("workspaceSettings.save")}
              savedLabel={t("workspaceSettings.saved")}
            />
          ) : (
            <SettingsRow
              title={t("createWorkspace.name")}
              description={t("workspaceSettings.nameAdminsOnly")}
              value={workspace.name}
            />
          )}
          <SettingsRow
            title={t("createWorkspace.slug")}
            description={t("workspaceSettings.slugFixed")}
            value={workspace.slug}
          />
        </SettingsCard>
      </SettingsSection>
      {admin && <DangerSection workspace={workspace} />}
    </>
  );
});

/**
 * DangerSection deletes the workspace for every member, once its slug is typed; the shell then goes to /, which lands
 * on another workspace or on the creation page.
 */
function DangerSection({ workspace }: { workspace: Workspace }) {
  const workspaces = useWorkspaces();
  const t = useT();
  return (
    <SettingsSection title={t("settings.danger")}>
      <SettingsCard>
        <SettingsRow title={t("workspaceSettings.deleteTitle")} description={t("workspaceSettings.deleteBody")}>
          {() => (
            <ConfirmDialog
              trigger={
                <Button variant="destructive" size="sm" shape="pill">
                  {t("workspaceSettings.delete")}
                </Button>
              }
              title={t("workspaceSettings.deleteConfirmTitle", { name: workspace.name })}
              description={t("workspaceSettings.deleteBody")}
              typedConfirmation={{
                label: t("workspaceSettings.typeSlug", { slug: workspace.slug }),
                value: workspace.slug,
              }}
              confirmLabel={t("workspaceSettings.deleteConfirm")}
              sendingLabel={t("workspaceSettings.deleting")}
              cancelLabel={t("workspaceSettings.cancel")}
              confirm={() => workspaces.remove(workspace.slug)}
            />
          )}
        </SettingsRow>
      </SettingsCard>
    </SettingsSection>
  );
}
