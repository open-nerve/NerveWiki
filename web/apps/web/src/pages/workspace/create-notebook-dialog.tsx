import { useRef, useState, type FormEvent, type ReactElement } from "react";
import { useNavigate } from "react-router";

import { arrived } from "../../app/arrival";
import { useForm } from "../../app/form";
import { notebookNameProblem, notebookNameTexts } from "../../app/notebook-name";
import { FormField } from "../../components/form-field";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle, DialogTrigger } from "../../components/ui/dialog";
import { useT } from "../../i18n/i18n";
import type { Notebook, WorkspaceAccess } from "../../services/notebook.service";
import type { Workspace } from "../../services/workspace.service";
import { useNotebooks } from "../../stores/context";
import { AccessOptions } from "../notebook/access-options";

/**
 * CreateNotebookDialog creates a notebook in workspace, of which the
 * account becomes the admin, then goes to its home, arrived at (M3/P4
 * design 3.3, 3.5): the dialog closes, and the focus goes to the home's
 * heading, not back to trigger. The form lives only while the dialog is
 * open: what was typed and cancelled is not offered again. A creation
 * answered after the dialog was cancelled goes nowhere: the left column
 * lists the notebook, as the token list gets a token whose dialog closed.
 */
export function CreateNotebookDialog({ workspace, trigger }: { workspace: Workspace; trigger: ReactElement }) {
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  /** How many times the dialog opened or closed: a creation sent before the last of them goes nowhere. */
  const turns = useRef(0);
  const created = useRef(false);
  function show(next: boolean) {
    turns.current += 1;
    setOpen(next);
  }
  /** creating is what a creation sent now does once answered: arrive at the notebook, unless the dialog closed since. */
  function creating() {
    const turn = turns.current;
    return (notebook: Notebook) => {
      if (turn !== turns.current) {
        return;
      }
      created.current = true;
      show(false);
      void navigate(`/${workspace.slug}/notebooks/${notebook.id}`, { state: arrived });
    };
  }
  return (
    <Dialog open={open} onOpenChange={show}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      {open && (
        <DialogContent
          onCloseAutoFocus={(event) => {
            if (created.current) {
              event.preventDefault();
              created.current = false;
            }
          }}
        >
          <CreateNotebookForm workspace={workspace} cancel={() => show(false)} creating={creating} />
        </DialogContent>
      )}
    </Dialog>
  );
}

/** The fields shown: a problem with the access, which the radio buttons cannot make, goes above the form. */
const fields = ["name"] as const;

type CreateNotebookFormProps = {
  workspace: Workspace;
  /** Called as the creation goes out: what to do with the notebook once created. */
  creating: () => (notebook: Notebook) => void;
  cancel: () => void;
};

function CreateNotebookForm({ workspace, creating, cancel }: CreateNotebookFormProps) {
  const notebooks = useNotebooks(workspace);
  const t = useT();
  const [name, setName] = useState("");
  const [access, setAccess] = useState<WorkspaceAccess>("none");
  const { ref, sending, banner, problemOf, submit } = useForm(fields, { fieldTexts: notebookNameTexts });

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const problem = notebookNameProblem(name);
    void submit(problem === undefined ? {} : { name: problem }, async () => {
      const arrive = creating();
      arrive(await notebooks.create({ name: name.trim(), workspace_access: access }));
    });
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      <DialogTitle>{t("notebooks.create")}</DialogTitle>
      <DialogDescription>{t("notebooks.createBody")}</DialogDescription>
      {banner !== undefined && <Alert>{banner}</Alert>}
      <FormField
        label={t("notebooks.name")}
        name="name"
        autoComplete="off"
        value={name}
        error={problemOf("name")}
        hint={t("notebooks.nameHint")}
        onChange={(event) => setName(event.target.value)}
      />
      <AccessOptions value={access} onChange={setAccess} />
      <div className="flex justify-end gap-2">
        <Button variant="outline" onClick={cancel}>
          {t("notebooks.cancel")}
        </Button>
        <Button type="submit" disabled={sending}>
          {t("notebooks.submit")}
        </Button>
      </div>
    </form>
  );
}
