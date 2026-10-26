import { observer } from "mobx-react-lite";
import { useRef, useState, type FormEvent } from "react";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { useForm } from "../../app/form";
import { notebookNameProblem, notebookNameTexts } from "../../app/notebook-name";
import { RenameForm } from "../../app/rename-form";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { Notebook, WorkspaceAccess } from "../../services/notebook.service";
import type { Workspace } from "../../services/workspace.service";
import { useNotebooks } from "../../stores/context";
import { useWorkspace } from "../workspace/workspace-layout";
import { AccessOptions } from "./access-options";
import { useNotebook } from "./notebook-layout";

/**
 * NotebookGeneralPage is a notebook's name, workspace access and mode
 * (M3/P4 design 3.4, M10/P1 design 4.4): its admins change them and delete
 * it; the others see them.
 */
export const NotebookGeneralPage = observer(function NotebookGeneralPage() {
  const workspace = useWorkspace();
  const notebook = useNotebook();
  const notebooks = useNotebooks(workspace);
  const t = useT();
  const admin = notebook.role === "admin";
  return (
    <div className="space-y-10">
      <section className="max-w-md space-y-6">
        <h2 className="text-lg font-semibold">{t("notebookSettings.general")}</h2>
        {admin ? (
          <>
            <RenameForm
              current={notebook.name}
              label={t("notebooks.name")}
              hint={t("notebooks.nameHint")}
              autoComplete="off"
              check={notebookNameProblem}
              fieldTexts={notebookNameTexts}
              rename={(name) => notebooks.update(notebook.id, { name })}
              saveLabel={t("notebookSettings.save")}
              savedLabel={t("notebookSettings.saved")}
            />
            <AccessForm workspace={workspace} notebook={notebook} />
          </>
        ) : (
          <>
            <div className="space-y-1">
              <h3 className="text-sm font-medium">{t("notebooks.name")}</h3>
              <p className="break-words">{notebook.name}</p>
              <p className="text-sm text-muted-foreground">{t("notebookSettings.nameAdminsOnly")}</p>
            </div>
            <div className="space-y-1">
              <h3 className="text-sm font-medium">{t("access.legend")}</h3>
              <p>{t(`access.${notebook.workspace_access}`)}</p>
              <p className="text-sm text-muted-foreground">{t(`access.${notebook.workspace_access}Hint`)}</p>
              <p className="text-sm text-muted-foreground">{t("notebookSettings.accessAdminsOnly")}</p>
            </div>
          </>
        )}
      </section>
      <ModeSection workspace={workspace} notebook={notebook} />
      {admin && <DeleteSection workspace={workspace} notebook={notebook} />}
    </div>
  );
});

type NotebookProps = { workspace: Workspace; notebook: Notebook };

/**
 * AccessForm changes who in the workspace sees the notebook: the access
 * chosen goes out on Save, not as it is chosen, since the arrow keys choose
 * as they move. Only a change goes out; the left column may then show the
 * notebook in the other group. Like RenameForm, it shows the access as the
 * list has it until one is chosen, and again once saved, unless another
 * was chosen while the save was out.
 */
const AccessForm = observer(function AccessForm({ workspace, notebook }: NotebookProps) {
  const notebooks = useNotebooks(workspace);
  const t = useT();
  /** The access chosen since the last save; none, the access as the list has it. */
  const [draft, setDraft] = useState<WorkspaceAccess>();
  const access = draft ?? notebook.workspace_access;
  const [saved, setSaved] = useState(false);
  const { ref, sending, banner, submit } = useForm<never>([]);
  /** How many times the access was chosen: a save tells whether the access it sent is still the one shown. */
  const edits = useRef(0);

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const chosen = access;
    const edit = edits.current;
    setSaved(false);
    void submit({}, async () => {
      if (chosen !== notebook.workspace_access) {
        await notebooks.update(notebook.id, { workspace_access: chosen });
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
      <AccessOptions
        value={access}
        onChange={(chosen) => {
          edits.current++;
          setDraft(chosen);
          setSaved(false);
        }}
      />
      <div className="flex items-center gap-3">
        <Button type="submit" disabled={sending}>
          {t("notebookSettings.save")}
        </Button>
        <output className="text-sm text-muted-foreground">{saved ? t("notebookSettings.saved") : ""}</output>
      </div>
    </form>
  );
});

/** ModeSection keeps a refused downgrade's dialog even when a reread removes its trigger. */
function ModeSection({ workspace, notebook }: NotebookProps) {
  const notebooks = useNotebooks(workspace);
  const t = useT();
  const [open, setOpen] = useState(false);
  const heading = useRef<HTMLHeadingElement>(null);
  const button = useRef<HTMLButtonElement>(null);
  const wiki = notebook.mode === "llm_wiki";
  const downgrades = wiki && notebook.role === "admin";

  function onClosed(done: boolean) {
    (done ? heading.current : (button.current ?? heading.current))?.focus();
  }

  return (
    <section className="max-w-md space-y-3">
      <h2 ref={heading} tabIndex={-1} className="text-lg font-semibold outline-none">
        {t("notebookSettings.mode")}
      </h2>
      <p>{wiki ? t("notebookSettings.modeLLMWiki") : t("notebookSettings.modeStandard")}</p>
      <p className="text-sm text-muted-foreground">
        {wiki ? t("notebookSettings.modeLLMWikiHint") : t("notebookSettings.modeStandardHint")}
      </p>
      {downgrades && (
        <Button ref={button} variant="outline" onClick={() => setOpen(true)}>
          {t("notebookSettings.downgrade")}
        </Button>
      )}
      <ConfirmDialog
        held={{ open, onOpenChange: setOpen, onClosed }}
        title={t("notebookSettings.downgradeTitle", { name: notebook.name })}
        description={t("notebookSettings.downgradeBody")}
        confirmLabel={t("notebookSettings.downgradeConfirm")}
        sendingLabel={t("notebookSettings.downgrading")}
        cancelLabel={t("notebookSettings.cancel")}
        confirm={() => notebooks.downgrade(notebook.id)}
        texts={{ "transfer.busy": "transfer.writingBusy", forbidden: "notebookSettings.downgradeAdminsOnly" }}
        tone="default"
      />
    </section>
  );
}

/**
 * DeleteSection deletes the notebook for every member, once its name is
 * typed (M3 design 4); the shell then goes to the workspace's home,
 * arrived at.
 */
function DeleteSection({ workspace, notebook }: NotebookProps) {
  const notebooks = useNotebooks(workspace);
  const t = useT();
  return (
    <section className="max-w-md space-y-3">
      <h2 className="text-lg font-semibold">{t("notebookSettings.deleteTitle")}</h2>
      <p className="text-sm text-muted-foreground">{t("notebookSettings.deleteBody")}</p>
      <ConfirmDialog
        trigger={<Button variant="destructive">{t("notebookSettings.delete")}</Button>}
        title={t("notebookSettings.deleteConfirmTitle", { name: notebook.name })}
        description={t("notebookSettings.deleteBody")}
        typedConfirmation={{ label: t("notebookSettings.typeName", { name: notebook.name }), value: notebook.name }}
        confirmLabel={t("notebookSettings.deleteConfirm")}
        sendingLabel={t("notebookSettings.deleting")}
        cancelLabel={t("notebookSettings.cancel")}
        confirm={() => notebooks.remove(notebook.id)}
      />
    </section>
  );
}
