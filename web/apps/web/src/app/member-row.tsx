import { observer } from "mobx-react-lite";

import { Button } from "../components/ui/button";
import { useT } from "../i18n/i18n";
import { ConfirmDialog } from "./confirm-dialog";
import { MemberSummary, memberWho, type Member } from "./member-summary";
import { RoleMenu } from "./role-menu";

type MemberRowProps<Role extends string> = {
  member: Member & { role: Role };
  /** Whether the member is the signed-in account. */
  you: boolean;
  /** Whether the row offers to change the role and to remove: an admin's, for another member. */
  manage: boolean;
  /** The roles the menu offers, from the most rights to the fewest, and the name of each. */
  roles: readonly Role[];
  roleLabel: (role: Role) => string;
  /** What the row adds after the role, e.g. the higher role the member has by the workspace (M3 handoff 2). */
  note?: string;
  /** The removal's question and what it does to the member. */
  removeTitle: string;
  removeBody: string;
  /** Changes the member's role; a refusal is the list's to say. */
  changeRole: (role: Role) => Promise<void>;
  /** Removes the member. */
  remove: () => Promise<void>;
  /** Where the focus goes once the member is removed, with the row. */
  removed: () => void;
};

function MemberRowView<Role extends string>({
  member,
  you,
  manage,
  roles,
  roleLabel,
  note,
  removeTitle,
  removeBody,
  changeRole,
  remove,
  removed,
}: MemberRowProps<Role>) {
  const t = useT();
  const who = memberWho(member, t);
  return (
    <li className="flex flex-wrap items-center justify-between gap-4 p-4">
      <MemberSummary member={member} you={you} />
      <div className="flex flex-wrap items-center gap-2">
        {manage ? (
          <>
            <RoleMenu role={member.role} roles={roles} label={roleLabel} who={who} changeRole={changeRole} />
            {note !== undefined && <span className="text-sm text-muted-foreground">{note}</span>}
            <ConfirmDialog
              trigger={
                <Button variant="outline" aria-label={t("members.removeLabel", { name: who })}>
                  {t("members.remove")}
                </Button>
              }
              title={removeTitle}
              description={removeBody}
              confirmLabel={t("members.remove")}
              sendingLabel={t("members.removing")}
              cancelLabel={t("members.cancel")}
              confirm={remove}
              focusAfter={removed}
            />
          </>
        ) : (
          <span className="text-sm">
            {roleLabel(member.role)}
            {note !== undefined && <span className="text-muted-foreground"> · {note}</span>}
          </span>
        )}
      </div>
    </li>
  );
}

/**
 * MemberRow is a member of a list, a workspace's (M2/P6 design 3.3) or a
 * notebook's (M3/P4 design 3.4): the name, the address unless the viewer
 * is a guest, when they joined, and the role, with its note; for an
 * admin, another member's role changes as one is chosen in its menu, and
 * they can be removed. The controls name the member by the address too,
 * where it shows: two members may have the same name. The two lists
 * differ only in their roles and their removal's words (M3 handoff 3).
 */
export const MemberRow = observer(MemberRowView);
