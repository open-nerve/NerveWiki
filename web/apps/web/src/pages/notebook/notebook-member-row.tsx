import { observer } from "mobx-react-lite";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { MemberSummary, memberWho } from "../../app/member-summary";
import { RoleMenu } from "../../app/role-menu";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { NotebookMember } from "../../services/notebook-member.service";
import type { Notebook, NotebookRole } from "../../services/notebook.service";
import { notebookRoles } from "./notebook-roles";

type NotebookMemberRowProps = {
  notebook: Notebook;
  member: NotebookMember;
  /** Whether the member is the signed-in account. */
  you: boolean;
  /** Whether the row offers to change the role and to remove: an admin's, for another member. */
  manage: boolean;
  /** Changes the member's role; a refusal is the list's to say. */
  changeRole: (role: NotebookRole) => Promise<void>;
  /** Removes the member. */
  remove: () => Promise<void>;
  /** Where the focus goes once the member is removed, with the row. */
  removed: () => void;
};

/**
 * NotebookMemberRow is a member of a notebook's list (M3/P4 design 3.4), as
 * a workspace's MemberRow is: who they are and their role; for an admin,
 * another member's role menu and Remove.
 */
export const NotebookMemberRow = observer(function NotebookMemberRow({
  notebook,
  member,
  you,
  manage,
  changeRole,
  remove,
  removed,
}: NotebookMemberRowProps) {
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
              roles={notebookRoles}
              label={(role) => t(`notebookRole.${role}`)}
              who={who}
              changeRole={changeRole}
            />
            <ConfirmDialog
              trigger={
                <Button variant="outline" aria-label={t("members.removeLabel", { name: who })}>
                  {t("members.remove")}
                </Button>
              }
              title={t("notebookMembers.removeTitle", { name: member.display_name, notebook: notebook.name })}
              description={t("notebookMembers.removeBody")}
              confirmLabel={t("members.remove")}
              sendingLabel={t("members.removing")}
              cancelLabel={t("members.cancel")}
              confirm={remove}
              focusAfter={removed}
            />
          </>
        ) : (
          <span className="text-sm">{t(`notebookRole.${member.role}`)}</span>
        )}
      </div>
    </li>
  );
});
