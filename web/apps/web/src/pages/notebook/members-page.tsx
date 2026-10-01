import { observer } from "mobx-react-lite";
import { useId, useRef, useState, type FormEvent, type RefObject } from "react";
import { Link } from "react-router";
import useSWR, { useSWRConfig } from "swr";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { useForm } from "../../app/form";
import { memberWho } from "../../app/member-summary";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { Label } from "../../components/ui/label";
import { NativeSelect } from "../../components/ui/native-select";
import { useT } from "../../i18n/i18n";
import type { WorkspaceMember } from "../../services/member.service";
import type { Notebook, NotebookRole } from "../../services/notebook.service";
import type { Workspace } from "../../services/workspace.service";
import { useAccount, useMembers, useNotebookMembers, useNotebooks } from "../../stores/context";
import { useWorkspace } from "../workspace/workspace-layout";
import { NotebookMemberRow } from "./notebook-member-row";
import { useNotebook } from "./notebook-layout";
import { NotebookRoleOptions } from "./notebook-roles";

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

type SectionProps = { workspace: Workspace; notebook: Notebook };

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
 * AddSection adds a member of the workspace who is not in the notebook
 * yet: its candidates are the two lists' difference.
 */
const AddSection = observer(function AddSection({ workspace, notebook }: SectionProps) {
  const workspaceMembers = useMembers(workspace);
  const notebookMembers = useNotebookMembers(notebook);
  const t = useT();
  const { error, mutate } = useSWR(["members", workspace.id], () => workspaceMembers.load());
  const ins = notebookMembers.list;
  const candidates =
    ins === undefined
      ? undefined
      : workspaceMembers.list?.filter((member) => !ins.some((each) => each.user_id === member.user_id));
  return (
    <section className="max-w-xl space-y-4">
      <div className="space-y-1">
        <h2 className="text-lg font-semibold">{t("notebookMembers.addTitle")}</h2>
        <p className="text-sm text-muted-foreground">{t("notebookMembers.addBody")}</p>
      </div>
      {candidates === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : (
        <AddForm workspace={workspace} notebook={notebook} candidates={candidates} />
      )}
    </section>
  );
});

/** The fields shown: a problem with the role, which the select cannot make, goes above the form. */
const additionFields = ["user_id"] as const;

/**
 * AddForm adds the member chosen with the role chosen, an editor unless
 * another is. Once added, the choice is emptied and the status says who;
 * a refusal reads the lists again: the account chosen may have just left
 * the workspace, or been added by another admin; the account itself may
 * no longer be the notebook's admin. With no one left to add, the
 * member's field says so.
 */
function AddForm({ workspace, notebook, candidates }: SectionProps & { candidates: WorkspaceMember[] }) {
  const members = useNotebookMembers(notebook);
  const t = useT();
  const { mutate: reload } = useSWRConfig();
  const memberId = useId();
  const roleId = useId();
  const [userId, setUserId] = useState("");
  const [role, setRole] = useState<NotebookRole>("editor");
  const [added, setAdded] = useState<string>();
  const { ref, sending, banner, problemOf, submit } = useForm(additionFields);
  const memberProblem = problemOf("user_id");
  // The field stays with none to choose, saying why: a problem shown under it stays, and the focus on it.
  const memberNote = memberProblem ?? (candidates.length === 0 ? t("notebookMembers.noCandidates") : undefined);

  async function add(chosen: string) {
    setAdded(undefined);
    const done = await submit(chosen === "" ? { user_id: "field.required" } : {}, async () => {
      const member = await members.add(chosen, role);
      setUserId("");
      setAdded(member.display_name);
      void reload(["notebooks", workspace.id]);
    });
    if (!done && chosen !== "") {
      void reload(["members", workspace.id]);
      void reload(["notebook-members", notebook.id]);
      void reload(["notebooks", workspace.id]);
    }
  }

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void add(candidates.some((member) => member.user_id === userId) ? userId : "");
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      {banner !== undefined && <Alert>{banner}</Alert>}
      <div className="flex flex-wrap items-start gap-3">
        <div className="min-w-56 flex-1 space-y-2">
          <Label htmlFor={memberId}>{t("notebookMembers.member")}</Label>
          <NativeSelect
            id={memberId}
            name="user_id"
            value={userId}
            aria-invalid={memberProblem !== undefined || undefined}
            aria-describedby={memberNote === undefined ? undefined : `${memberId}-note`}
            onChange={(event) => {
              setUserId(event.target.value);
              setAdded(undefined);
            }}
            className="w-full"
          >
            <option value="">{t("notebookMembers.choose")}</option>
            {candidates.map((member) => (
              <option key={member.user_id} value={member.user_id}>
                {memberWho(member, t)}
              </option>
            ))}
          </NativeSelect>
          {memberNote !== undefined && (
            <p
              id={`${memberId}-note`}
              className={memberProblem === undefined ? "text-sm text-muted-foreground" : "text-sm text-destructive"}
            >
              {memberNote}
            </p>
          )}
        </div>
        <div className="space-y-2">
          <Label htmlFor={roleId}>{t("notebookMembers.role")}</Label>
          <NativeSelect
            id={roleId}
            name="role"
            value={role}
            onChange={(event) => setRole(event.target.value as NotebookRole)}
          >
            <NotebookRoleOptions />
          </NativeSelect>
        </div>
      </div>
      <Button type="submit" disabled={sending}>
        {t("notebookMembers.add")}
      </Button>
      <output className="block text-sm text-muted-foreground">
        {added === undefined ? "" : t("notebookMembers.added", { name: added })}
      </output>
    </form>
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
