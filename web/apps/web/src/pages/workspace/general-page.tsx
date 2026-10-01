import { observer } from "mobx-react-lite";
import { useRef, useState, type FormEvent } from "react";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { useForm } from "../../app/form";
import { workspaceNameProblem } from "../../app/slug";
import { FormField } from "../../components/form-field";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { Workspace } from "../../services/workspace.service";
import { useWorkspaces } from "../../stores/context";
import { useWorkspace } from "./workspace-layout";

/**
 * GeneralPage is a workspace's name and address (M2/P5 design 3.6): its
 * admins rename and delete it; the others see them.
 */
export const GeneralPage = observer(function GeneralPage() {
  const workspace = useWorkspace();
  const t = useT();
  const admin = workspace.role === "admin";
  return (
    <div className="space-y-10">
      <section className="max-w-md space-y-6">
        <h2 className="text-lg font-semibold">{t("workspaceSettings.general")}</h2>
        {admin ? (
          <RenameForm workspace={workspace} />
        ) : (
          <div className="space-y-1">
            <h3 className="text-sm font-medium">{t("createWorkspace.name")}</h3>
            <p>{workspace.name}</p>
            <p className="text-sm text-muted-foreground">{t("workspaceSettings.nameAdminsOnly")}</p>
          </div>
        )}
        <div className="space-y-1">
          <h3 className="text-sm font-medium">{t("createWorkspace.slug")}</h3>
          <p>{workspace.slug}</p>
          <p className="text-sm text-muted-foreground">{t("workspaceSettings.slugFixed")}</p>
        </div>
      </section>
      {admin && <DeleteSection workspace={workspace} />}
    </div>
  );
});

/**
 * RenameForm renames the workspace: the name goes out only when it changed
 * (trimmed, as the server keeps it); the switcher shows the new one. Until
 * it is edited, and again once a save went through, the field shows the
 * name as the list has it, a rename by another admin too: Save then sends
 * nothing back over it. A name edited while its rename was out keeps the
 * edit, as DisplayNameForm has it: what it shows then is not what was
 * saved, and the next save sends it.
 */
const RenameForm = observer(function RenameForm({ workspace }: { workspace: Workspace }) {
  const workspaces = useWorkspaces();
  const t = useT();
  /** What was typed since the last save; none, the name as the list has it. */
  const [draft, setDraft] = useState<string>();
  const name = draft ?? workspace.name;
  const [saved, setSaved] = useState(false);
  const { ref, sending, banner, problemOf, submit } = useForm(["name"]);
  /** How many times the field was edited: a save tells whether the name it sent is still the one shown. */
  const edits = useRef(0);

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const problem = workspaceNameProblem(name);
    const trimmed = name.trim();
    const edit = edits.current;
    setSaved(false);
    void submit(problem === undefined ? {} : { name: problem }, async () => {
      if (trimmed !== workspace.name) {
        await workspaces.rename(workspace.slug, trimmed);
      }
      if (edits.current === edit) {
        setDraft(undefined);
        setSaved(true);
      }
    });
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      {banner !== undefined && <Alert>{banner}</Alert>}
      <FormField
        label={t("createWorkspace.name")}
        name="name"
        value={name}
        error={problemOf("name")}
        onChange={(event) => {
          edits.current++;
          setDraft(event.target.value);
          setSaved(false);
        }}
      />
      <div className="flex items-center gap-3">
        <Button type="submit" disabled={sending}>
          {t("workspaceSettings.save")}
        </Button>
        <output className="text-sm text-muted-foreground">{saved ? t("workspaceSettings.saved") : ""}</output>
      </div>
    </form>
  );
});

/**
 * DeleteSection deletes the workspace for every member, once its slug is
 * typed; the shell then goes to /, which lands on another workspace or on
 * the creation page.
 */
function DeleteSection({ workspace }: { workspace: Workspace }) {
  const workspaces = useWorkspaces();
  const t = useT();
  return (
    <section className="max-w-md space-y-3">
      <h2 className="text-lg font-semibold">{t("workspaceSettings.deleteTitle")}</h2>
      <p className="text-sm text-muted-foreground">{t("workspaceSettings.deleteBody")}</p>
      <ConfirmDialog
        trigger={<Button variant="destructive">{t("workspaceSettings.delete")}</Button>}
        title={t("workspaceSettings.deleteConfirmTitle", { name: workspace.name })}
        description={t("workspaceSettings.deleteBody")}
        typedConfirmation={{ label: t("workspaceSettings.typeSlug", { slug: workspace.slug }), value: workspace.slug }}
        confirmLabel={t("workspaceSettings.deleteConfirm")}
        sendingLabel={t("workspaceSettings.deleting")}
        cancelLabel={t("workspaceSettings.cancel")}
        confirm={() => workspaces.remove(workspace.slug)}
      />
    </section>
  );
}
