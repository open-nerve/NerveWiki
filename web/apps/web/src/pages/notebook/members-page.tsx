import { observer } from "mobx-react-lite";
import { useRef, useState, type RefObject } from "react";
import { Link } from "react-router";
import useSWR, { useSWRConfig } from "swr";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { NotebookRole } from "../../services/notebook.service";
import type { Workspace } from "../../services/workspace.service";
import { useAccount, useNotebookMembers, useNotebooks } from "../../stores/context";
import { useWorkspace } from "../workspace/workspace-layout";
import { AddSection, type SectionProps } from "./add-member-section";
import { NotebookMemberRow } from "./notebook-member-row";
import { useNotebook } from "./notebook-layout";

/**
 * NotebookMembersPage is who is in the notebook (M3/P4 design 3.4): whoever
 * sees it sees its members; an admin changes the others' roles, removes
 * them and adds the workspace's members; an explicit member can leave. A
 * change to who is in it is read into the workspace's notebooks too: their
 * member counts make the groups.
 */
export const NotebookMembersPage = observer(function NotebookMembersPage() {
  const workspace = useWorkspace();
  const notebook = useNotebook();
  const members = useNotebookMembers(notebook);
  const { me } = useAccount();
  // A row that leaves, or the leave section, takes the focus with it: it comes to the list's heading instead.
  const heading = useRef<HTMLHeadingElement>(null);
  // Only an explicit member has a membership to end: one who sees the notebook by its access has none.
  const explicit = members.list?.some((member) => member.user_id === me.id) === true;
  return (
    <div className="space-y-10">
      <MembersSection workspace={workspace} notebook={notebook} heading={heading} />
      {notebook.role === "admin" && <AddSection workspace={workspace} notebook={notebook} />}
      {explicit && <LeaveSection workspace={workspace} notebook={notebook} heading={heading} />}
    </div>
  );
});

const MembersSection = observer(function MembersSection({
  workspace,
  notebook,
  heading,
}: SectionProps & { heading: RefObject<HTMLHeadingElement | null> }) {
  const members = useNotebookMembers(notebook);
  const { me } = useAccount();
  const t = useT();
  const { mutate: reload } = useSWRConfig();
  const { error, mutate } = useSWR(["notebook-members", notebook.id], () => members.load());
  const [failure, setFailure] = useState<unknown>();
  const failed = failure === undefined ? undefined : errorText(failure, t);

  /**
   * changeRole changes the role of the membership id. A refusal says why
   * above the list, until the next change; the list is read again, and the
   * notebooks too: one member gone, or the account's own role changed
   * elsewhere, which the controls then follow.
   */
  async function changeRole(id: string, role: NotebookRole) {
    setFailure(undefined);
    try {
      await members.changeRole(id, role);
    } catch (refusal) {
      setFailure(refusal);
      void mutate();
      void reload(["notebooks", workspace.id]);
    }
  }

  async function remove(id: string) {
    await members.remove(id);
    void reload(["notebooks", workspace.id]);
  }

  return (
    <section className="space-y-4">
      <h2 ref={heading} tabIndex={-1} className="text-lg font-semibold outline-none">
        {t("notebookSettings.members")}
      </h2>
      {failed !== undefined && <Alert>{failed}</Alert>}
      {members.list !== undefined && !members.list.some((member) => member.role === "admin") && (
        <NoAdmin workspace={workspace} />
      )}
      {members.list === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : (
        <ul aria-label={t("notebookSettings.members")} className="divide-y rounded-md border">
          {members.list.map((member) => (
            <NotebookMemberRow
              key={member.id}
              notebook={notebook}
              member={member}
              you={member.user_id === me.id}
              manage={notebook.role === "admin" && member.user_id !== me.id}
              changeRole={(role) => changeRole(member.id, role)}
              remove={() => remove(member.id)}
              removed={() => heading.current?.focus()}
            />
          ))}
        </ul>
      )}
    </section>
  );
});

/**
 * NoAdmin says the notebook has no admin (M3/P5 design 3.4): ownerless, it
 * stays in use by its members. A workspace admin is shown where to take it
 * over.
 */
function NoAdmin({ workspace }: { workspace: Workspace }) {
  const t = useT();
  return (
    <p className="text-sm">
      {t("notebookMembers.noAdmin")}{" "}
      {workspace.role === "admin" ? (
        <Link to={`/${workspace.slug}/settings/ownerless`} className="underline underline-offset-4">
          {t("notebookMembers.takeOverThere")}
        </Link>
      ) : (
        t("notebookMembers.adminsTakeOver")
      )}
    </p>
  );
}

/**
 * LeaveSection ends the account's membership (M3/P4 design 3.4): a
 * notebook it no longer sees then sends the shell to the workspace's home;
 * one open to the workspace it still sees stays, its members read again.
 * The only admin is refused, which the dialog says, as it says a
 * membership that has ended already.
 */
function LeaveSection({
  workspace,
  notebook,
  heading,
}: SectionProps & { heading: RefObject<HTMLHeadingElement | null> }) {
  const notebooks = useNotebooks(workspace);
  const t = useT();
  const { mutate: reload } = useSWRConfig();
  return (
    <section className="max-w-md space-y-3">
      <h2 className="text-lg font-semibold">{t("notebookMembers.leaveTitle")}</h2>
      <p className="text-sm text-muted-foreground">{t("notebookMembers.leaveBody")}</p>
      <ConfirmDialog
        trigger={<Button variant="outline">{t("notebookMembers.leave")}</Button>}
        title={t("notebookMembers.leaveConfirmTitle", { name: notebook.name })}
        description={t("notebookMembers.leaveBody")}
        confirmLabel={t("notebookMembers.leaveConfirm")}
        sendingLabel={t("notebookMembers.leaving")}
        cancelLabel={t("notebookSettings.cancel")}
        confirm={async () => {
          await notebooks.leave(notebook.id);
          // Out of sight, the notebook's members answer 404: the shell is on its way home.
          if (notebooks.byId(notebook.id) !== undefined) {
            await reload(["notebook-members", notebook.id]);
          }
        }}
        focusAfter={() => heading.current?.focus()}
        texts={{
          "notebook.sole_admin": "notebookMembers.soleAdmin",
          "notebook.member_not_found": "notebookMembers.notMember",
        }}
      />
    </section>
  );
}
