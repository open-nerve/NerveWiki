import { observer } from "mobx-react-lite";
import { useState } from "react";
import useSWR, { useSWRConfig } from "swr";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { MemberRow } from "../../app/member-row";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { SettingsCard, SettingsRow, SettingsSection, useSettingsHeading } from "../../app/settings-page";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { WorkspaceRole } from "../../services/member.service";
import type { Workspace } from "../../services/workspace.service";
import { useAccount, useMembers, useWorkspaces } from "../../stores/context";
import { InvitationsSection } from "./invitations-section";
import { roles } from "./role-options";
import { useWorkspace } from "./workspace-layout";

/**
 * MembersPage is who is in the workspace (M2/P6 design 3.3): every member
 * sees the members and can leave; an admin changes the others' roles,
 * removes them, and invites. The members a card's list, untitled (the
 * page's name says it; M13/P3/S2, decision 851), the invitations a group
 * of their own, the leaving the danger zone's (decision 826).
 */
export const MembersPage = observer(function MembersPage() {
  const workspace = useWorkspace();
  return (
    <>
      <MembersSection workspace={workspace} />
      {workspace.role === "admin" && <InvitationsSection workspace={workspace} />}
      <LeaveSection workspace={workspace} />
    </>
  );
});

const MembersSection = observer(function MembersSection({ workspace }: { workspace: Workspace }) {
  const members = useMembers(workspace);
  const { me } = useAccount();
  const t = useT();
  const { mutate: reload } = useSWRConfig();
  const { error, mutate } = useSWR(["members", workspace.id], () => members.load());
  const [failure, setFailure] = useState<unknown>();
  // A removed member's row leaves with its button: the focus comes to the page's h1, its name (M13/P3/S2, decision
  // 815), instead of the page.
  const heading = useSettingsHeading();
  const failed = failure === undefined ? undefined : errorText(failure, t);

  /**
   * changeRole changes the role of the membership id. A refusal says why
   * above the list, until the next change; the list is read again, and the
   * workspaces too, whatever the refusal: one member gone, or the account's
   * own role changed elsewhere, which the controls then follow.
   */
  async function changeRole(id: string, role: WorkspaceRole) {
    setFailure(undefined);
    try {
      await members.changeRole(id, role);
    } catch (refusal) {
      setFailure(refusal);
      void mutate();
      void reload("workspaces");
    }
  }

  /**
   * remove ends the membership id; the invitations pending to the member's
   * address went with it, and the notebooks they alone administered are
   * ownerless now.
   */
  async function remove(id: string) {
    await members.remove(id);
    void reload(["invitations", workspace.id]);
    void reload(["ownerless", workspace.id]);
  }

  return (
    <SettingsSection>
      {failed !== undefined && <Alert>{failed}</Alert>}
      {members.list === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : (
        <SettingsCard list label={t("workspaceSettings.members")}>
          {members.list.map((member) => (
            <MemberRow
              key={member.id}
              member={member}
              you={member.user_id === me.id}
              manage={workspace.role === "admin" && member.user_id !== me.id}
              roles={roles}
              roleLabel={(role) => t(`role.${role}`)}
              removeTitle={t("members.removeTitle", { name: member.display_name, workspace: workspace.name })}
              removeBody={t("members.removeBody")}
              changeRole={(role) => changeRole(member.id, role)}
              remove={() => remove(member.id)}
              removed={() => heading.current?.focus()}
            />
          ))}
        </SettingsCard>
      )}
    </SettingsSection>
  );
});

/**
 * LeaveSection ends the account's membership, a row of the danger zone (M13/P3/S2, decision 826); the shell then goes
 * to /, as after a deletion (M2/P5 design 3.6).
 */
function LeaveSection({ workspace }: { workspace: Workspace }) {
  const workspaces = useWorkspaces();
  const t = useT();
  return (
    <SettingsSection title={t("settings.danger")}>
      <SettingsCard>
        <SettingsRow title={t("members.leaveTitle")} description={t("members.leaveBody")}>
          {() => (
            <ConfirmDialog
              trigger={
                <Button variant="destructive" size="sm" shape="pill">
                  {t("members.leave")}
                </Button>
              }
              title={t("members.leaveConfirmTitle", { name: workspace.name })}
              description={t("members.leaveBody")}
              confirmLabel={t("members.leaveConfirm")}
              sendingLabel={t("members.leaving")}
              cancelLabel={t("members.cancel")}
              confirm={() => workspaces.leave(workspace.slug)}
              texts={{ "notebook.sole_admin": "members.leaveNotebookSoleAdmin" }}
            />
          )}
        </SettingsRow>
      </SettingsCard>
    </SettingsSection>
  );
}
