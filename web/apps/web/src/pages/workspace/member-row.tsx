import { observer } from "mobx-react-lite";
import { useState } from "react";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { NativeSelect } from "../../components/ui/native-select";
import { Button } from "../../components/ui/button";
import { formatDate } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import type { WorkspaceMember, WorkspaceRole } from "../../services/member.service";
import type { Workspace } from "../../services/workspace.service";
import { useMembers, useStore } from "../../stores/context";

const roles: readonly WorkspaceRole[] = ["admin", "member", "guest"];

type MemberRowProps = {
  workspace: Workspace;
  member: WorkspaceMember;
  /** Whether the member is the signed-in account. */
  you: boolean;
  /** Whether the row offers to change the role and to remove: an admin's, for another member. */
  manage: boolean;
  /** Called with why a change of role was refused. */
  refused: (error: unknown) => void;
  /** Where the focus goes once the member is removed, with the row. */
  removed: () => void;
};

/**
 * MemberRow is a member of the list (M2/P6 design 3.3): the name, the
 * address unless the viewer is a guest, when they joined, and the role;
 * for an admin, another member's role changes as it is chosen, and they
 * can be removed.
 */
export const MemberRow = observer(function MemberRow({
  workspace,
  member,
  you,
  manage,
  refused,
  removed,
}: MemberRowProps) {
  const members = useMembers(workspace);
  const { preferences } = useStore();
  const t = useT();
  const [sending, setSending] = useState(false);

  async function changeRole(role: WorkspaceRole) {
    setSending(true);
    try {
      await members.changeRole(member.id, role);
    } catch (error) {
      refused(error);
    } finally {
      setSending(false);
    }
  }

  return (
    <li className="flex flex-wrap items-center justify-between gap-4 p-4">
      <div className="min-w-0 space-y-1">
        <p className="flex flex-wrap items-center gap-2 font-medium break-words">
          {member.display_name}
          {you && (
            <span className="rounded border px-1.5 text-xs font-normal text-muted-foreground">{t("members.you")}</span>
          )}
        </p>
        <p className="text-sm break-all text-muted-foreground">
          {member.email !== null && <>{member.email} · </>}
          {t("members.joined", { date: formatDate(member.created_at, preferences.locale) })}
        </p>
      </div>
      <div className="flex items-center gap-2">
        {manage ? (
          <>
            <NativeSelect
              aria-label={t("members.roleOf", { name: member.display_name })}
              value={member.role}
              disabled={sending}
              onChange={(event) => void changeRole(event.target.value as WorkspaceRole)}
            >
              {roles.map((role) => (
                <option key={role} value={role}>
                  {t(`role.${role}`)}
                </option>
              ))}
            </NativeSelect>
            <ConfirmDialog
              trigger={
                <Button variant="outline" aria-label={t("members.removeLabel", { name: member.display_name })}>
                  {t("members.remove")}
                </Button>
              }
              title={t("members.removeTitle", { name: member.display_name, workspace: workspace.name })}
              description={t("members.removeBody")}
              confirmLabel={t("members.remove")}
              sendingLabel={t("members.removing")}
              cancelLabel={t("members.cancel")}
              confirm={() => members.remove(member.id)}
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
