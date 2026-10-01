import { observer } from "mobx-react-lite";
import { useRef, useState } from "react";
import useSWR, { useSWRConfig } from "swr";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { Workspace } from "../../services/workspace.service";
import { useAccount, useMembers, useWorkspaces } from "../../stores/context";
import { MemberRow } from "./member-row";
import { useWorkspace } from "./workspace-layout";

/**
 * MembersPage is who is in the workspace (M2/P6 design 3.3): every member
 * sees the members and can leave; an admin changes the others' roles and
 * removes them.
 */
export const MembersPage = observer(function MembersPage() {
  const workspace = useWorkspace();
  return (
    <div className="space-y-10">
      <MembersSection workspace={workspace} />
      <LeaveSection workspace={workspace} />
    </div>
  );
});

const MembersSection = observer(function MembersSection({ workspace }: { workspace: Workspace }) {
  const members = useMembers(workspace);
  const { me } = useAccount();
  const t = useT();
  const { mutate: reload } = useSWRConfig();
  const { error, mutate } = useSWR(["members", workspace.id], () => members.load());
  const [failure, setFailure] = useState<unknown>();
  // A removed member's row leaves with its button: the focus comes to the section's heading instead of the page.
  const heading = useRef<HTMLHeadingElement>(null);
  const failed = failure === undefined ? undefined : errorText(failure, t);

  /**
   * A change of role refused says why above the list. The list is read
   * again, and the workspaces too: a refusal may come of the account's own
   * role, changed elsewhere, which the controls then follow.
   */
  function refused(refusal: unknown) {
    setFailure(refusal);
    void mutate();
    void reload("workspaces");
  }

  return (
    <section className="space-y-4">
      <h2 ref={heading} tabIndex={-1} className="text-lg font-semibold outline-none">
        {t("workspaceSettings.members")}
      </h2>
      {failed !== undefined && <Alert>{failed}</Alert>}
      {members.list === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : (
        <ul className="divide-y rounded-md border">
          {members.list.map((member) => (
            <MemberRow
              key={member.id}
              workspace={workspace}
              member={member}
              you={member.user_id === me.id}
              manage={workspace.role === "admin" && member.user_id !== me.id}
              refused={refused}
              removed={() => heading.current?.focus()}
            />
          ))}
        </ul>
      )}
    </section>
  );
});

/** LeaveSection ends the account's membership; the shell then goes to /, as after a deletion (M2/P5 design 3.6). */
function LeaveSection({ workspace }: { workspace: Workspace }) {
  const workspaces = useWorkspaces();
  const t = useT();
  return (
    <section className="max-w-md space-y-3">
      <h2 className="text-lg font-semibold">{t("members.leaveTitle")}</h2>
      <p className="text-sm text-muted-foreground">{t("members.leaveBody")}</p>
      <ConfirmDialog
        trigger={<Button variant="outline">{t("members.leave")}</Button>}
        title={t("members.leaveConfirmTitle", { name: workspace.name })}
        description={t("members.leaveBody")}
        confirmLabel={t("members.leaveConfirm")}
        sendingLabel={t("members.leaving")}
        cancelLabel={t("members.cancel")}
        confirm={() => workspaces.leave(workspace.slug)}
      />
    </section>
  );
}
