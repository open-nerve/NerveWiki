import { observer } from "mobx-react-lite";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { MemberSummary, memberWho } from "../../app/member-summary";
import { RoleMenu } from "../../app/role-menu";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { WorkspaceMember, WorkspaceRole } from "../../services/member.service";
import type { Workspace } from "../../services/workspace.service";
import { roles } from "./role-options";

type MemberRowProps = {
  workspace: Workspace;
  member: WorkspaceMember;
  /** Whether the member is the signed-in account. */
  you: boolean;
  /** Whether the row offers to change the role and to remove: an admin's, for another member. */
  manage: boolean;
  /** Changes the member's role; a refusal is the list's to say. */
  changeRole: (role: WorkspaceRole) => Promise<void>;
  /** Removes the member. */
  remove: () => Promise<void>;
  /** Where the focus goes once the member is removed, with the row. */
  removed: () => void;
};

/**
 * MemberRow is a member of the list (M2/P6 design 3.3): the name, the
 * address unless the viewer is a guest, when they joined, and the role;
 * for an admin, another member's role changes as one is chosen in its
 * menu, and they can be removed. The controls name the member by the
 * address too, where it shows: two members may have the same name.
 */
export const MemberRow = observer(function MemberRow({
  workspace,
  member,
  you,
  manage,
  changeRole,
  remove,
  removed,
}: MemberRowProps) {
  const t = useT();
  const who = memberWho(member, t);
  return (
    <li className="flex flex-wrap items-center justify-between gap-4 p-4">
      <MemberSummary member={member} you={you} />
      <div className="flex items-center gap-2">
        {manage ? (
          <>
            <RoleMenu
              role={member.role}
              roles={roles}
              label={(role) => t(`role.${role}`)}
              who={who}
              changeRole={changeRole}
            />
            <ConfirmDialog
              trigger={
                <Button variant="outline" aria-label={t("members.removeLabel", { name: who })}>
                  {t("members.remove")}
                </Button>
              }
              title={t("members.removeTitle", { name: member.display_name, workspace: workspace.name })}
              description={t("members.removeBody")}
              confirmLabel={t("members.remove")}
              sendingLabel={t("members.removing")}
              cancelLabel={t("members.cancel")}
              confirm={remove}
              focusAfter={removed}
            />
          </>
        ) : (
          <span className="text-sm">{t(`role.${member.role}`)}</span>
        )}
      </div>
    </li>
  );
});
