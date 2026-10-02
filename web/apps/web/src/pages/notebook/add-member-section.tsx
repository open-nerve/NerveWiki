import { observer } from "mobx-react-lite";
import { useId, useRef, useState, type FormEvent } from "react";
import useSWR, { useSWRConfig } from "swr";

import { useForm } from "../../app/form";
import { memberWho } from "../../app/member-summary";
import { NotLoaded } from "../../app/not-loaded";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { Label } from "../../components/ui/label";
import { NativeSelect } from "../../components/ui/native-select";
import { useT } from "../../i18n/i18n";
import type { WorkspaceMember } from "../../services/member.service";
import type { Notebook, NotebookRole } from "../../services/notebook.service";
import type { Workspace } from "../../services/workspace.service";
import { useMembers, useNotebookMembers } from "../../stores/context";
import { NotebookRoleOptions } from "./notebook-roles";

/** SectionProps are what each section of the members page is given. */
export type SectionProps = { workspace: Workspace; notebook: Notebook };

/**
 * AddSection adds a member of the workspace who is not in the notebook
 * yet: its candidates are the two lists' difference.
 */
export const AddSection = observer(function AddSection({ workspace, notebook }: SectionProps) {
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
 * another is. Once added, the status says who, and the choice is emptied
 * unless another member was chosen while the addition was out (v0.1
 * design 13.2, item 12);
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
  /** How many times the member was chosen: an addition tells whether the field still shows the one it sent. */
  const edits = useRef(0);
  const { ref, sending, banner, problemOf, submit } = useForm(additionFields);
  const memberProblem = problemOf("user_id");
  // The field stays with none to choose, saying why: a problem shown under it stays, and the focus on it.
  const memberNote = memberProblem ?? (candidates.length === 0 ? t("notebookMembers.noCandidates") : undefined);

  async function add(chosen: string) {
    const edit = edits.current;
    setAdded(undefined);
    const done = await submit(chosen === "" ? { user_id: "field.required" } : {}, async () => {
      const member = await members.add(chosen, role);
      if (edits.current === edit) {
        setUserId("");
      }
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
              edits.current++;
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
