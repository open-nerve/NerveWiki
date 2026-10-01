import { observer } from "mobx-react-lite";
import { useState, type FormEvent } from "react";
import useSWR from "swr";

import { useForm } from "../app/form";
import { notebookTarget } from "../app/landing";
import { notebookNameProblem, notebookNameTexts } from "../app/notebook-name";
import { NotLoaded } from "../app/not-loaded";
import { FormField } from "../components/form-field";
import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { useT } from "../i18n/i18n";
import type { Workspace } from "../services/workspace.service";
import { useNotebooks, useStore, useWorkspaces } from "../stores/context";
import { GoOn, type StepProps } from "./go-on";

/**
 * NotebookStep sees the account to a first notebook (M3/P4 design 3.6), in
 * the workspace it lands on where it may create one. One that sees a
 * notebook there goes on at once; one that sees none creates a private
 * one, which it then sees: a retry after a failed record creates no
 * second. Without such a workspace, it reads how to get one, and goes on.
 */
export const NotebookStep = observer(function NotebookStep({ complete }: StepProps) {
  const workspaces = useWorkspaces();
  const { preferences } = useStore();
  const { error, mutate } = useSWR("workspaces", () => workspaces.load());
  if (workspaces.list === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  const target = notebookTarget(workspaces.list, preferences.lastWorkspace());
  if (target === undefined) {
    return <NoTarget complete={complete} />;
  }
  return <InWorkspace key={target.id} workspace={target} complete={complete} />;
});

const InWorkspace = observer(function InWorkspace({ workspace, complete }: StepProps & { workspace: Workspace }) {
  const notebooks = useNotebooks(workspace);
  const { error, mutate } = useSWR(["notebooks", workspace.id], () => notebooks.load());
  if (notebooks.list === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  if (notebooks.list.length > 0) {
    return <GoOn complete={complete} />;
  }
  // Once created, the workspace has a notebook the account sees: the step goes on as above.
  return <FirstNotebookForm workspace={workspace} />;
});

/**
 * FirstNotebookForm creates a private notebook, named My notes in the
 * page's language unless the user names it. Its workspace becomes the one
 * the account lands on, where the notebook shows.
 */
function FirstNotebookForm({ workspace }: { workspace: Workspace }) {
  const notebooks = useNotebooks(workspace);
  const { preferences } = useStore();
  const t = useT();
  const [name, setName] = useState(() => t("onboarding.notebook.defaultName"));
  const { ref, sending, banner, problemOf, submit } = useForm(["name"], { fieldTexts: notebookNameTexts });

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const problem = notebookNameProblem(name);
    void submit(problem === undefined ? {} : { name: problem }, async () => {
      await notebooks.create({ name: name.trim(), workspace_access: "none" });
      preferences.setLastWorkspace(workspace.slug);
    });
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      <p className="text-sm text-muted-foreground">{t("onboarding.notebook.hint", { workspace: workspace.name })}</p>
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
      <Button type="submit" className="w-full" disabled={sending}>
        {t("onboarding.notebook.create")}
      </Button>
    </form>
  );
}

/** NoTarget says where notebooks are created when the account may create none yet; Continue completes the step. */
function NoTarget({ complete }: StepProps) {
  const t = useT();
  const { ref, sending, banner, submit } = useForm<never>([]);

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void submit({}, complete);
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      {banner !== undefined && <Alert>{banner}</Alert>}
      <p className="text-muted-foreground">{t("onboarding.notebook.noTarget")}</p>
      <Button type="submit" className="w-full" disabled={sending}>
        {t("onboarding.continue")}
      </Button>
    </form>
  );
}
