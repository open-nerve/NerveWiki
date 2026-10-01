import { ChevronDown } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useState } from "react";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { Button } from "../../components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "../../components/ui/dropdown-menu";
import { formatDate } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import type { WorkspaceMember, WorkspaceRole } from "../../services/member.service";
import type { Workspace } from "../../services/workspace.service";
import { useStore } from "../../stores/context";
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
  const { preferences } = useStore();
  const t = useT();
  const who =
    member.email === null ? member.display_name : t("members.who", { name: member.display_name, email: member.email });
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
            <RoleMenu role={member.role} who={who} changeRole={changeRole} />
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

type RoleMenuProps = { role: WorkspaceRole; who: string; changeRole: (role: WorkspaceRole) => Promise<void> };

/**
 * RoleMenu changes a member's role as one is chosen: a menu, so that
 * moving through the roles chooses none, and the focus goes back to its
 * button. One change goes out at a time; a choice made while one is out
 * is not taken.
 */
function RoleMenu({ role, who, changeRole }: RoleMenuProps) {
  const t = useT();
  const [sending, setSending] = useState(false);

  async function choose(chosen: WorkspaceRole) {
    if (sending || chosen === role) {
      return;
    }
    setSending(true);
    try {
      await changeRole(chosen);
    } finally {
      setSending(false);
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          aria-label={t("members.roleOf", { role: t(`role.${role}`), name: who })}
          aria-busy={sending || undefined}
        >
          {t(`role.${role}`)}
          <ChevronDown />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuRadioGroup value={role} onValueChange={(chosen) => void choose(chosen as WorkspaceRole)}>
          {roles.map((each) => (
            <DropdownMenuRadioItem key={each} value={each}>
              {t(`role.${each}`)}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
